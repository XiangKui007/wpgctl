// Package render 实现 site.yaml + 模板 → 最终 compose/env/nacos 配置渲染。
//
// 关键原则（方案 §5.1 / §6.5）：
// site.yaml 是源数据（编译源），各服务目录下的 .env/compose 是产物，产物永不手改。
package render

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 渲染选项。
type Options struct {
	Site        *config.SiteConfig
	Manifest    *config.Manifest
	PackageDir  string // release/patch 包根目录
	OutputDir   string // 渲染输出目录，默认 {workspace}/rendered/{version}
}

// Result 渲染结果。
type Result struct {
	OutputDir   string
	Files       []string
	ServiceEnvs map[string]string // 服务名 -> .env 路径
	ComposeFiles []string
}

// Context 注入模板的根上下文。
type Context struct {
	Site       config.SiteInfo
	Nodes      []config.Node
	Profiles   []string
	Middleware config.MiddlewareConfig
	Overrides  map[string]config.Override
	Paths      config.PathsConfig
	Manifest   *config.Manifest
	Version    string
	// HostIP 主业务节点物理 IP（供 Kafka advertised.listeners 等使用）。
	HostIP string
	// ProfileEnabled 便于模板内判断 profile 开关。
	ProfileEnabled map[string]bool
}

// Run 执行配置渲染，输出到 OutputDir 并保留快照供排障。
func Run(opts Options) (*Result, error) {
	if opts.Site == nil || opts.Manifest == nil {
		return nil, fmt.Errorf("site 与 manifest 均不能为空")
	}
	if opts.PackageDir == "" {
		return nil, fmt.Errorf("package 目录不能为空")
	}

	outDir := opts.OutputDir
	if outDir == "" {
		outDir = filepath.Join(opts.Site.Paths.Workspace, "rendered", opts.Manifest.Version)
	}
	if err := util.EnsureDir(outDir); err != nil {
		return nil, err
	}

	ctx := buildContext(opts.Site, opts.Manifest)
	res := &Result{
		OutputDir:   outDir,
		ServiceEnvs: map[string]string{},
	}

	tplRoot := filepath.Join(opts.PackageDir, "compose", "templates")
	if util.DirExists(tplRoot) {
		if err := renderTree(tplRoot, filepath.Join(outDir, "compose"), ctx, ".tmpl", res); err != nil {
			return res, err
		}
	}

	envTpl := filepath.Join(opts.PackageDir, "config", "env.tmpl")
	if util.FileExists(envTpl) {
		dest := filepath.Join(outDir, ".env")
		if err := renderFile(envTpl, dest, ctx); err != nil {
			return res, err
		}
		res.Files = append(res.Files, dest)
	}

	nacosRoot := filepath.Join(opts.PackageDir, "config", "nacos")
	if util.DirExists(nacosRoot) {
		if err := renderNacos(nacosRoot, filepath.Join(outDir, "nacos"), ctx, opts.Site.Profiles, res); err != nil {
			return res, err
		}
	}

	// nginx / redis / prometheus 等非 env 配置
	extraRoots := []struct {
		srcRel string
		dstRel string
	}{
		{"config/nginx", "nginx"},
		{"config/redis", "redis"},
		{"config/prometheus", "prometheus"},
	}
	for _, e := range extraRoots {
		src := filepath.Join(opts.PackageDir, filepath.FromSlash(e.srcRel))
		if !util.DirExists(src) {
			continue
		}
		if err := renderTree(src, filepath.Join(outDir, e.dstRel), ctx, ".tmpl", res); err != nil {
			return res, err
		}
	}

	util.Successf("配置渲染完成 → %s（共 %d 个文件）", outDir, len(res.Files))
	return res, nil
}

func buildContext(site *config.SiteConfig, m *config.Manifest) *Context {
	hostIP := ""
	for _, n := range site.Nodes {
		for _, role := range n.Roles {
			if role == "middleware" || role == "platform" || role == "app" {
				hostIP = n.IP
				break
			}
		}
		if hostIP != "" {
			break
		}
	}
	if hostIP == "" && len(site.Nodes) > 0 {
		hostIP = site.Nodes[0].IP
	}
	pe := map[string]bool{}
	for _, p := range site.Profiles {
		pe[p] = true
	}
	return &Context{
		Site:           site.Site,
		Nodes:          site.Nodes,
		Profiles:       site.Profiles,
		Middleware:     site.Middleware,
		Overrides:      site.Overrides,
		Paths:          site.Paths,
		Manifest:       m,
		Version:        m.Version,
		HostIP:         hostIP,
		ProfileEnabled: pe,
	}
}

func renderTree(srcDir, dstDir string, ctx *Context, suffix string, res *Result) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		destRel := rel
		if strings.HasSuffix(rel, suffix) {
			destRel = strings.TrimSuffix(rel, suffix)
		}
		dest := filepath.Join(dstDir, destRel)
		if err := util.EnsureDir(filepath.Dir(dest)); err != nil {
			return err
		}
		if strings.HasSuffix(path, suffix) || strings.HasSuffix(path, ".tmpl") {
			if err := renderFile(path, dest, ctx); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return err
			}
		}
		res.Files = append(res.Files, dest)
		if strings.HasSuffix(dest, ".yml") || strings.HasSuffix(dest, ".yaml") {
			res.ComposeFiles = append(res.ComposeFiles, dest)
		}
		return nil
	})
}

func renderNacos(src, dst string, ctx *Context, profiles []string, res *Result) error {
	set := map[string]struct{}{}
	for _, p := range profiles {
		set[p] = struct{}{}
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, ok := set[name]; !ok && name != "common" {
			continue
		}
		if err := renderTree(filepath.Join(src, name), filepath.Join(dst, name), ctx, ".tmpl", res); err != nil {
			return err
		}
	}
	return nil
}

func renderFile(tplPath, dest string, ctx *Context) error {
	raw, err := os.ReadFile(tplPath)
	if err != nil {
		return fmt.Errorf("读取模板失败 %s: %w", tplPath, err)
	}
	funcMap := sprig.TxtFuncMap()
	// required：缺省即报错，消灭静默空值
	funcMap["required"] = func(warn string, v interface{}) (interface{}, error) {
		if v == nil {
			return nil, fmt.Errorf("%s", warn)
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) == "" {
				return nil, fmt.Errorf("%s", warn)
			}
		}
		return v, nil
	}
	funcMap["hasProfile"] = func(name string) bool {
		return ctx.ProfileEnabled[name]
	}

	tpl, err := template.New(filepath.Base(tplPath)).Funcs(funcMap).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return fmt.Errorf("解析模板失败 %s: %w", tplPath, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("渲染模板失败 %s: %w", tplPath, err)
	}
	if err := util.EnsureDir(filepath.Dir(dest)); err != nil {
		return err
	}
	return os.WriteFile(dest, buf.Bytes(), 0o644)
}
