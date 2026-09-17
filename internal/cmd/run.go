package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/db"
	"github.com/wpg/wpgctl/internal/deploy"
	fw "github.com/wpg/wpgctl/internal/firewall"
	"github.com/wpg/wpgctl/internal/moduledeploy"
	"github.com/wpg/wpgctl/internal/fetch"
	"github.com/wpg/wpgctl/internal/initenv"
	"github.com/wpg/wpgctl/internal/nacos"
	"github.com/wpg/wpgctl/internal/pack"
	"github.com/wpg/wpgctl/internal/render"
	"github.com/wpg/wpgctl/internal/status"
	"github.com/wpg/wpgctl/internal/upgrade"
	"github.com/wpg/wpgctl/internal/util"
)

func runInit(site *config.SiteConfig, mf *config.Manifest, basePkg, dockerPkg string, localOnly bool) (*initenv.Result, error) {
	return initenv.Run(initenv.Options{
		Site:          site,
		Manifest:      mf,
		BasePackage:   basePkg,
		DockerPackage: dockerPkg,
		LocalOnly:     localOnly,
		SitePath:      flagSitePath,
	})
}

func runFetch(name, from, local, dest string) error {
	res, err := fetch.Run(fetch.Options{Name: name, FromURL: from, LocalDir: local, DestDir: dest})
	if err != nil {
		// 本地已是完整包目录
		if strings.HasPrefix(err.Error(), "LOCAL_READY:") {
			src := strings.TrimPrefix(err.Error(), "LOCAL_READY:")
			util.Infof("检测到完整包目录，直接使用: %s", src)
			return nil
		}
		return err
	}
	util.Infof("包目录: %s", res.PackageDir)
	return nil
}

func runDeploy(sitePath, packageDir string, dryRun bool, concurrency int) error {
	site, err := config.LoadSite(sitePath)
	if err != nil {
		return err
	}
	mf, err := config.LoadManifest(filepath.Join(packageDir, "manifest.yaml"))
	if err != nil {
		return err
	}
	_, err = deploy.Run(deploy.Options{
		Site:        site,
		Manifest:    mf,
		PackageDir:  packageDir,
		SitePath:    sitePath,
		Concurrency: concurrency,
		DryRun:      dryRun,
	})
	return err
}

func runRender(sitePath, packageDir string) error {
	site, err := config.LoadSite(sitePath)
	if err != nil {
		return err
	}
	mf, err := config.LoadManifest(filepath.Join(packageDir, "manifest.yaml"))
	if err != nil {
		return err
	}
	res, err := render.Run(render.Options{Site: site, Manifest: mf, PackageDir: packageDir})
	if err != nil {
		return err
	}
	util.Infof("输出目录: %s", res.OutputDir)
	return nil
}

func runDBApply(sitePath, packageDir string, dryRun bool) error {
	site, err := config.LoadSite(sitePath)
	if err != nil {
		return err
	}
	if dryRun {
		list, err := db.ListPending(packageDir)
		if err != nil {
			return err
		}
		for _, s := range list {
			fmt.Printf("  %s  db=%s  %s\n", filepath.Base(s.Path), s.Database, s.Desc)
		}
		return nil
	}
	_, err = db.Run(db.Options{Site: site, PackageDir: packageDir})
	return err
}

func runNacosImport(sitePath, configDir string) error {
	site, err := config.LoadSite(sitePath)
	if err != nil {
		return err
	}
	_, err = nacos.Run(nacos.Options{Site: site, ConfigDir: configDir})
	return err
}

func runFirewallOpen(sitePath, manifestPath string) error {
	site, err := config.LoadSite(sitePath)
	if err != nil {
		return err
	}
	var mf *config.Manifest
	if manifestPath != "" {
		mf, err = config.LoadManifest(manifestPath)
		if err != nil {
			return err
		}
	}
	ports := moduledeploy.PortsForSite(site, mf)
	util.Infof("将放行 %d 个 TCP 端口…", len(ports))
	res, err := fw.OpenPorts(ports)
	if res != nil {
		util.Infof("防火墙: %s，新开 %d，已有 %d，失败 %d",
			res.Firewall, len(res.Opened), len(res.Skipped), len(res.Failed))
	}
	return err
}

func runUpgrade(sitePath, patchDir string, yes bool) error {
	site, err := config.LoadSite(sitePath)
	if err != nil {
		return err
	}
	_, err = upgrade.Run(upgrade.Options{Site: site, PatchDir: patchDir, SitePath: sitePath, Yes: yes})
	return err
}

func runRollback(sitePath, to string) error {
	site, err := config.LoadSite(sitePath)
	if err != nil {
		return err
	}
	return upgrade.Rollback(site, to)
}

func runStatus(site *config.SiteConfig, composeDir string) error {
	return status.ShowStatus(site, composeDir)
}

func runLogs(service string, tail int, follow bool) error {
	return status.ShowLogs(service, tail, follow)
}

func runDiag(sitePath, manifest, composeDir, renderDir, output string) error {
	site, err := config.LoadSite(sitePath)
	if err != nil {
		return err
	}
	var mf *config.Manifest
	if manifest != "" {
		mf, err = config.LoadManifest(manifest)
		if err != nil {
			return err
		}
	}
	_, err = status.CollectDiag(status.DiagOptions{
		Site: site, Manifest: mf, ComposeDir: composeDir, RenderDir: renderDir, Output: output,
	})
	return err
}

func runPack(kind, version, manifest, output string, services []string, baseRelease, workDir string, volumeSizeGB int) error {
	_, err := pack.Run(pack.Options{
		Kind: kind, Version: version, ManifestPath: manifest, OutputDir: output,
		Services: services, BaseRelease: baseRelease, WorkDir: workDir, VolumeSizeGB: volumeSizeGB,
	})
	return err
}

func runPackScan(dir, out, kind, version, arch string, write bool) error {
	res, err := pack.ScanDir(pack.ScanOptions{
		Dir: dir, Kind: kind, Version: version, Arch: arch,
	})
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		util.Warnf("%s", w)
	}
	util.Infof("扫描到 %d 个镜像", len(res.Images))
	for _, img := range res.Images {
		util.Infof("  L%d  %-16s  %s", img.Layer, img.Name, img.Image)
	}

	outPath := out
	if write && outPath == "" {
		outPath = filepath.Join(dir, "manifest.yaml")
	}
	if outPath != "" {
		if err := pack.WriteManifest(outPath, res.YAML); err != nil {
			return err
		}
		util.Successf("已写入 %s", outPath)
		return nil
	}
	fmt.Print(res.YAML)
	return nil
}
