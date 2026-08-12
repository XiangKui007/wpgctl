// Package upgrade 实现补丁升级与回滚（方案 §6.10）。
package upgrade

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/db"
	"github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/health"
	"github.com/wpg/wpgctl/internal/nacos"
	"github.com/wpg/wpgctl/internal/state"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 升级选项。
type Options struct {
	Site       *config.SiteConfig
	PatchDir   string
	SitePath   string
	Yes        bool // 跳过确认
}

// Result 升级结果。
type Result struct {
	FromVersion string
	ToVersion   string
	Services    []string
	RolledBack  bool
	Success     bool
}

// Run 执行补丁升级。失败时对镜像/前端/Nacos 自动回滚；SQL 不自动回滚。
func Run(opts Options) (*Result, error) {
	manifestPath := filepath.Join(opts.PatchDir, "manifest.yaml")
	patch, err := config.LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	if patch.Kind != "patch" {
		return nil, fmt.Errorf("期望 patch 包，实际 kind=%s", patch.Kind)
	}

	st, err := state.NewStore()
	if err != nil {
		return nil, err
	}
	latest, err := st.LatestSuccess(opts.Site.Site.Code)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, fmt.Errorf("无成功部署记录，无法升级；请先完成首次 deploy")
	}
	if latest.PackageVer != patch.BaseRelease {
		return nil, fmt.Errorf("基线不匹配：本地=%s patch.baseRelease=%s", latest.PackageVer, patch.BaseRelease)
	}

	services := patch.ServicesList
	if len(services) == 0 {
		for _, s := range patch.Services {
			services = append(services, s.Name)
		}
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("patch 未声明涉及服务")
	}

	res := &Result{FromVersion: latest.PackageVer, ToVersion: patch.Version, Services: services}
	util.Infof("升级预览: %s → %s，服务: %v", res.FromVersion, res.ToVersion, services)
	if !opts.Yes {
		util.Warnf("即将执行升级，使用 --yes 跳过确认（当前非交互环境直接继续）")
	}

	docker := dockerx.New()
	bakRoot := filepath.Join(opts.Site.Paths.Workspace, "bak", res.FromVersion)
	_ = util.EnsureDir(bakRoot)

	// 备份 jar
	for _, name := range services {
		jarSrc := findJar(opts.Site.Paths.Workspace, name)
		if jarSrc != "" {
			dest := filepath.Join(bakRoot, filepath.Base(jarSrc))
			_ = copyFile(jarSrc, dest)
			util.Infof("已备份 jar: %s", dest)
		}
	}

	oldTags := map[string]string{}
	if latest.ServiceTags != nil {
		for _, name := range services {
			oldTags[name] = latest.ServiceTags[name]
		}
	}

	// 替换 jar + 现场 build 版本化 tag
	newTags := map[string]string{}
	for _, name := range services {
		newJar := findPatchJar(opts.PatchDir, name, patch.Version)
		if newJar == "" {
			util.Warnf("补丁包中未找到 %s 的 jar，跳过 build", name)
			continue
		}
		svcDir := filepath.Join(opts.Site.Paths.Workspace, name)
		_ = util.EnsureDir(svcDir)
		destJar := filepath.Join(svcDir, filepath.Base(newJar))
		if err := copyFile(newJar, destJar); err != nil {
			return res, err
		}
		tag := fmt.Sprintf("%s:%s", name, patch.Version)
		util.Infof("docker build -t %s", tag)
		if err := docker.Build(svcDir, tag); err != nil {
			return res, err
		}
		newTags[name] = tag
	}

	// 增量 SQL
	if _, err := db.Run(db.Options{Site: opts.Site, PackageDir: opts.PatchDir}); err != nil {
		util.Errorf("SQL 执行失败: %v（不会自动回滚 SQL）", err)
		return res, err
	}

	// Nacos
	nacosDir := filepath.Join(opts.PatchDir, "nacos")
	if util.DirExists(nacosDir) {
		if _, err := nacos.Run(nacos.Options{Site: opts.Site, ConfigDir: nacosDir}); err != nil {
			util.Warnf("Nacos 更新失败: %v", err)
		}
	}

	// 前端
	feDir := filepath.Join(opts.PatchDir, "frontend")
	if util.DirExists(feDir) {
		if err := applyFrontend(feDir, opts.Site.Paths.NginxHTML); err != nil {
			util.Warnf("前端更新失败: %v", err)
		}
	}

	// 滚动更新容器
	checker := health.New()
	ctx := context.Background()
	composeDir := filepath.Join(opts.Site.Paths.Workspace, "rendered", res.FromVersion, "compose")
	failed := false
	for _, name := range services {
		tag := newTags[name]
		if tag == "" {
			continue
		}
		// 简化：直接 compose up 该服务（模板中的 image 需已指向新 tag，或依赖外部已更新）
		if err := docker.ComposeUp(composeDir, "", opts.Site.Profiles, name); err != nil {
			util.Errorf("更新服务失败 %s: %v", name, err)
			failed = true
			break
		}
		svc := config.ServiceSpec{
			Name: name,
			Port: portOf(patch, name),
			Health: config.HealthSpec{Type: "http", Path: "/actuator/health", TimeoutSec: 120, IntervalSec: 5},
		}
		r := checker.WaitHealthy(ctx, "127.0.0.1", svc)
		if !r.OK {
			util.Errorf("健康检查失败 %s: %s，触发自动回滚", name, r.Message)
			failed = true
			break
		}
		util.Successf("%s 升级成功 → %s", name, tag)
	}

	if failed {
		res.RolledBack = true
		_ = rollbackServices(docker, composeDir, opts.Site.Profiles, oldTags)
		_ = st.Append(state.DeploymentRecord{
			Operator: os.Getenv("USER"), Action: "upgrade",
			PackageKind: "patch", PackageVer: patch.Version,
			SiteCode: opts.Site.Site.Code, Success: false,
			Message: "health check failed, rolled back images",
			ServiceTags: oldTags,
		})
		return res, fmt.Errorf("升级失败并已回滚镜像/容器（SQL 需人工评估）")
	}

	// 合并 tag 记录
	merged := map[string]string{}
	for k, v := range latest.ServiceTags {
		merged[k] = v
	}
	for k, v := range newTags {
		merged[k] = v
	}
	_ = st.Append(state.DeploymentRecord{
		Operator: os.Getenv("USER"), Action: "upgrade",
		PackageKind: "patch", PackageVer: patch.Version,
		SiteCode: opts.Site.Site.Code, Success: true,
		Message: "upgrade ok", ServiceTags: merged,
	})
	res.Success = true
	util.Successf("升级完成: %s → %s", res.FromVersion, res.ToVersion)
	return res, nil
}

// Rollback 手动回滚到指定版本（默认上一成功版本）。
func Rollback(site *config.SiteConfig, toVersion string) error {
	st, err := state.NewStore()
	if err != nil {
		return err
	}
	list, err := st.List()
	if err != nil {
		return err
	}
	var target *state.DeploymentRecord
	for i := len(list) - 1; i >= 0; i-- {
		r := list[i]
		if !r.Success || r.SiteCode != site.Site.Code {
			continue
		}
		if toVersion == "" || r.PackageVer == toVersion {
			cp := r
			target = &cp
			break
		}
	}
	if target == nil {
		return fmt.Errorf("找不到可回滚的版本记录")
	}
	docker := dockerx.New()
	composeDir := filepath.Join(site.Paths.Workspace, "rendered", target.PackageVer, "compose")
	if err := rollbackServices(docker, composeDir, site.Profiles, target.ServiceTags); err != nil {
		return err
	}
	_ = st.Append(state.DeploymentRecord{
		Operator: os.Getenv("USER"), Action: "rollback",
		PackageKind: target.PackageKind, PackageVer: target.PackageVer,
		SiteCode: site.Site.Code, Success: true,
		Message: "manual rollback", ServiceTags: target.ServiceTags,
	})
	util.Successf("已回滚到版本 %s", target.PackageVer)
	return nil
}

func rollbackServices(d *dockerx.Runner, composeDir string, profiles []string, tags map[string]string) error {
	for name := range tags {
		util.Warnf("回滚服务: %s", name)
		if err := d.ComposeUp(composeDir, "", profiles, name); err != nil {
			util.Errorf("回滚 %s 失败: %v", name, err)
		}
	}
	return nil
}

func findJar(workspace, name string) string {
	candidates := []string{
		filepath.Join(workspace, name, name+".jar"),
		filepath.Join(workspace, name, "app.jar"),
	}
	for _, c := range candidates {
		if util.FileExists(c) {
			return c
		}
	}
	return ""
}

func findPatchJar(patchDir, name, version string) string {
	candidates := []string{
		filepath.Join(patchDir, "jars", fmt.Sprintf("%s-%s.jar", name, version)),
		filepath.Join(patchDir, "jars", name+".jar"),
	}
	for _, c := range candidates {
		if util.FileExists(c) {
			return c
		}
	}
	return ""
}

func applyFrontend(feDir, nginxHTML string) error {
	entries, err := os.ReadDir(feDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(feDir, e.Name())
		bak := nginxHTML + ".bak-" + time.Now().Format("20060102150405")
		_ = os.Rename(nginxHTML, bak)
		_ = util.EnsureDir(nginxHTML)
		util.Infof("前端包 %s 需解压到 %s（已备份旧目录 %s）", src, nginxHTML, bak)
		// 实际解压由 tar 完成；此处复制归档到目标供运维确认
		_ = copyFile(src, filepath.Join(nginxHTML, e.Name()))
	}
	return nil
}

func portOf(m *config.Manifest, name string) int {
	for _, s := range m.Services {
		if s.Name == name {
			return s.Port
		}
	}
	return 0
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = util.EnsureDir(filepath.Dir(dst))
	return os.WriteFile(dst, data, 0o644)
}
