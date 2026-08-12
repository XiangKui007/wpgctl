// Package deploy 实现镜像加载、分层启动与健康检查（方案 §6.6 / §6.7）。
package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/health"
	"github.com/wpg/wpgctl/internal/render"
	"github.com/wpg/wpgctl/internal/state"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 部署选项。
type Options struct {
	Site       *config.SiteConfig
	Manifest   *config.Manifest
	PackageDir string
	SitePath   string // 用于计算 site hash
	Concurrency int   // docker load 并发度，默认 3
	SkipRender bool
	DryRun     bool
}

// SmokeReport 冒烟报告条目。
type SmokeEntry struct {
	Name    string        `json:"name"`
	Version string        `json:"version"`
	Port    int           `json:"port"`
	OK      bool          `json:"ok"`
	Message string        `json:"message"`
	Elapsed time.Duration `json:"elapsed"`
}

// Result 部署结果。
type Result struct {
	RenderDir string
	Smoke     []SmokeEntry
	Success   bool
	Duration  time.Duration
}

// Run 执行完整部署流程：渲染 → 导镜像 → 分层启动 → 健康检查 → 写台账。
func Run(opts Options) (*Result, error) {
	start := time.Now()
	if opts.Site == nil || opts.Manifest == nil {
		return nil, fmt.Errorf("site / manifest 不能为空")
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 3
	}

	res := &Result{}

	// 1. 渲染配置
	var renderDir string
	if !opts.SkipRender {
		rr, err := render.Run(render.Options{
			Site:       opts.Site,
			Manifest:   opts.Manifest,
			PackageDir: opts.PackageDir,
		})
		if err != nil {
			return res, fmt.Errorf("配置渲染失败: %w", err)
		}
		renderDir = rr.OutputDir
	} else {
		renderDir = filepath.Join(opts.Site.Paths.Workspace, "rendered", opts.Manifest.Version)
	}
	res.RenderDir = renderDir

	if opts.DryRun {
		util.Infof("dry-run：跳过镜像加载与启动")
		res.Success = true
		res.Duration = time.Since(start)
		return res, nil
	}

	services := opts.Manifest.EnabledServices(opts.Site.Profiles)

	// 2. 并行加载镜像
	if err := loadImages(opts.PackageDir, services, opts.Concurrency); err != nil {
		return res, err
	}

	// 3. 校验 requiredFiles
	for _, s := range services {
		for _, f := range s.RequiredFiles {
			p := filepath.Join(opts.PackageDir, f)
			if !util.FileExists(p) && !util.FileExists(filepath.Join(renderDir, f)) {
				return res, fmt.Errorf("服务 %s 缺少必需文件: %s", s.Name, f)
			}
		}
	}

	// 4. 分层启动
	docker := dockerx.New()
	checker := health.New()
	byLayer := map[int][]config.ServiceSpec{}
	for _, s := range services {
		byLayer[s.Layer] = append(byLayer[s.Layer], s)
	}

	ctx := context.Background()
	host := "127.0.0.1"
	var smoke []SmokeEntry

	for layer := 1; layer <= 4; layer++ {
		svcs := byLayer[layer]
		if len(svcs) == 0 {
			continue
		}
		util.Infof("======== 启动第 %d 层（%d 个服务）========", layer, len(svcs))

		composeDir := filepath.Join(renderDir, "compose")
		for _, s := range svcs {
			file := findComposeFile(composeDir, s.Name)
			profiles := opts.Site.Profiles
			if err := docker.ComposeUp(composeDir, file, profiles, s.Name); err != nil {
				util.Warnf("启动 %s 失败: %v（继续健康检查以汇总）", s.Name, err)
			}
		}

		// 层内并行健康检查
		type pair struct {
			svc config.ServiceSpec
			r   health.Result
		}
		ch := make(chan pair, len(svcs))
		var wg sync.WaitGroup
		for _, s := range svcs {
			wg.Add(1)
			go func(svc config.ServiceSpec) {
				defer wg.Done()
				r := checker.WaitHealthy(ctx, host, svc)
				ch <- pair{svc: svc, r: r}
			}(s)
		}
		wg.Wait()
		close(ch)

		layerOK := true
		for p := range ch {
			entry := SmokeEntry{
				Name:    p.svc.Name,
				Version: opts.Manifest.Version,
				Port:    p.svc.Port,
				OK:      p.r.OK,
				Message: p.r.Message,
				Elapsed: p.r.Elapsed,
			}
			smoke = append(smoke, entry)
			if p.r.OK {
				util.Successf("L%d %s 健康 (耗时 %s)", layer, p.svc.Name, p.r.Elapsed.Round(time.Second))
			} else {
				layerOK = false
				util.Errorf("L%d %s 失败: %s", layer, p.svc.Name, p.r.Message)
				// 附带日志
				logs, _ := docker.Logs(p.svc.Name, 200, false)
				if logs != "" {
					util.Errorf("---- %s 最近日志 ----\n%s", p.svc.Name, truncate(logs, 4000))
				}
			}
		}
		if !layerOK {
			res.Smoke = smoke
			res.Success = false
			res.Duration = time.Since(start)
			_ = writeDeployRecord(opts, res, false, "分层启动失败")
			return res, fmt.Errorf("第 %d 层存在不健康服务，已中止后续层启动", layer)
		}

		if layer == 1 {
			util.Infof("L1 完成：请确认 nacos import / db apply 是否需要执行")
		}
	}

	res.Smoke = smoke
	res.Success = true
	res.Duration = time.Since(start)
	printSmoke(smoke)
	_ = writeDeployRecord(opts, res, true, "deploy ok")
	util.Successf("部署完成，耗时 %s", res.Duration.Round(time.Second))
	return res, nil
}

func loadImages(pkgDir string, services []config.ServiceSpec, concurrency int) error {
	imgDir := filepath.Join(pkgDir, "images")
	if !util.DirExists(imgDir) {
		util.Warnf("包内无 images/ 目录，跳过 docker load（可能已预装）")
		return nil
	}
	docker := dockerx.New()
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	errCh := make(chan error, len(services))

	for _, s := range services {
		if docker.ImageExists(s.Image) {
			util.Infof("镜像已存在，跳过: %s", s.Image)
			continue
		}
		tarName := imageTarName(s.Image)
		tarPath := filepath.Join(imgDir, tarName)
		if !util.FileExists(tarPath) {
			// 尝试 name-version.tar
			alt := filepath.Join(imgDir, s.Name+".tar")
			if util.FileExists(alt) {
				tarPath = alt
			} else {
				util.Warnf("找不到镜像包 %s，跳过 load", tarName)
				continue
			}
		}
		wg.Add(1)
		go func(path, image string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			util.Infof("docker load: %s", path)
			if err := docker.LoadImage(path); err != nil {
				errCh <- err
			}
		}(tarPath, s.Image)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func imageTarName(image string) string {
	// waterwork-center:4.0.2 → waterwork-center-4.0.2.tar
	name := image
	for i := 0; i < len(image); i++ {
		if image[i] == '/' {
			name = image[i+1:]
		}
	}
	out := make([]byte, 0, len(name)+4)
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == ':' {
			out = append(out, '-')
		} else {
			out = append(out, c)
		}
	}
	return string(out) + ".tar"
}

func findComposeFile(composeDir, service string) string {
	candidates := []string{
		filepath.Join(composeDir, service+".yml"),
		filepath.Join(composeDir, service+".yaml"),
	}
	for _, c := range candidates {
		if util.FileExists(c) {
			return c
		}
	}
	// 分层文件：由 compose 多文件场景，返回空让 compose 用默认
	return ""
}

func writeDeployRecord(opts Options, res *Result, ok bool, msg string) error {
	st, err := state.NewStore()
	if err != nil {
		return err
	}
	siteHash := ""
	if opts.SitePath != "" {
		if data, err := os.ReadFile(opts.SitePath); err == nil {
			siteHash = util.SHA256Bytes(data)
		}
	}
	tags := map[string]string{}
	for _, s := range opts.Manifest.EnabledServices(opts.Site.Profiles) {
		tags[s.Name] = s.Image
	}
	return st.Append(state.DeploymentRecord{
		Operator:    os.Getenv("USER"),
		Action:      "deploy",
		PackageKind: opts.Manifest.Kind,
		PackageVer:  opts.Manifest.Version,
		SiteCode:    opts.Site.Site.Code,
		SiteHash:    siteHash,
		Success:     ok,
		DurationSec: int64(res.Duration.Seconds()),
		Message:     msg,
		ServiceTags: tags,
	})
}

func printSmoke(entries []SmokeEntry) {
	fmt.Println()
	fmt.Println("======== 冒烟报告 ========")
	fmt.Printf("%-24s %-10s %-8s %-6s %s\n", "服务", "版本", "端口", "状态", "说明")
	for _, e := range entries {
		st := "FAIL"
		if e.OK {
			st = "OK"
		}
		fmt.Printf("%-24s %-10s %-8d %-6s %s\n", e.Name, e.Version, e.Port, st, e.Message)
	}
	fmt.Println("==========================")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n... (truncated)"
}
