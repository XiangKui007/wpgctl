package moduledeploy

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/fetch"
	"github.com/wpg/wpgctl/internal/util"
)

// NginxPatchOptions nginx 模块现场部署：模块 tar load → html zip → 改 conf → compose up。
type NginxPatchOptions struct {
	Layout         NginxLayout
	ExpandArchives bool
	ProxyIPs       NginxProxyIPs
	ComposeUp      bool
	SkipProxyPatch bool // true：不改 conf 内 proxy_pass IP（由用户手工编辑保存）
}

// NginxPatchResult nginx 步骤执行摘要。
type NginxPatchResult struct {
	WebConf    string              `json:"webConf"`
	HTMLDir    string              `json:"htmlDir"`
	Steps      []string            `json:"steps"`
	ExpandHTML *fetch.ExpandResult `json:"expandHtml,omitempty"`
	ProxyNotes []string            `json:"proxyNotes"`
	Compose    *Result             `json:"compose,omitempty"`
}

// RunNginxPatch 执行 nginx 模块完整现场流程。
func RunNginxPatch(opts NginxPatchOptions) (*NginxPatchResult, error) {
	moduleDir := opts.Layout.ModuleDir
	if found := FindNginxModuleDir(moduleDir); found != "" {
		moduleDir = found
	} else if strings.EqualFold(filepath.Base(moduleDir), "nginx") {
		if alt := ModulePath(filepath.Dir(moduleDir), "nginx"); util.DirExists(alt) {
			moduleDir = alt
		}
	}
	opts.Layout.ModuleDir = moduleDir
	if opts.Layout.HTMLDir == "" || !util.DirExists(opts.Layout.HTMLDir) {
		opts.Layout.HTMLDir = filepath.Join(moduleDir, "html")
	}
	out := &NginxPatchResult{
		WebConf: opts.Layout.WebConf,
		HTMLDir: opts.Layout.HTMLDir,
		Steps:   []string{"nginx 模块目录: " + moduleDir},
	}

	var loadedRefs []string
	if opts.ExpandArchives {
		steps, err := PrepareModuleArchives(moduleDir)
		out.Steps = append(out.Steps, steps...)
		if err != nil {
			if errors.Is(err, fetch.ErrNoArchivesFound) {
				out.Steps = append(out.Steps, "模块目录无 zip/tar（多半已展开过），跳过解压")
			} else {
				tars, _ := findImageTars(moduleDir)
				if len(tars) == 0 {
					return out, fmt.Errorf("解压 nginx 模块失败: %w", err)
				}
				out.Steps = append(out.Steps, "模块目录无新压缩包，沿用已有 .tar")
			}
		}

		htmlRes, err := ExpandNginxHTML(opts.Layout.HTMLDir)
		if err != nil {
			return out, err
		}
		out.ExpandHTML = htmlRes
		if htmlRes != nil && htmlRes.ZipExtracted > 0 {
			out.Steps = append(out.Steps, fmt.Sprintf("html 解压: zip=%d", htmlRes.ZipExtracted))
		} else {
			out.Steps = append(out.Steps, "html 无待解压 zip（已跳过）")
		}

		tars, _ := findImageTars(moduleDir)
		if len(tars) == 0 {
			out.Steps = append(out.Steps, "未发现镜像 .tar，跳过 docker load（使用本机已有镜像）")
		} else {
			loadRes, err := Run(Options{ModuleDir: moduleDir, Load: true})
			if err != nil {
				return out, err
			}
			out.Steps = append(out.Steps, loadRes.Steps...)
			loadedRefs = append(loadedRefs, loadRes.LoadedRefs...)
		}
	}

	if opts.SkipProxyPatch {
		out.Steps = append(out.Steps, "跳过 proxy_pass IP 同步（使用已保存的 conf）")
	} else {
		notes, err := PatchHTTPWeb8877(opts.Layout.WebConf, opts.ProxyIPs)
		if err != nil {
			return out, err
		}
		out.ProxyNotes = notes
	}

	if opts.ComposeUp {
		composeRes, err := Run(Options{
			ModuleDir:  moduleDir,
			ComposeUp:  true,
			LoadedRefs: loadedRefs,
		})
		if err != nil {
			return out, err
		}
		out.Compose = composeRes
		out.Steps = append(out.Steps, composeRes.Steps...)
	}

	return out, nil
}
