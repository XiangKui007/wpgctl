// Package initenv 实现环境初始化（方案 §6.3），要求幂等。
//
// 步骤：安装 Docker 静态二进制 → 建目录 → 防火墙放行 → 内核参数。
package initenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/docker"
	fw "github.com/wpg/wpgctl/internal/firewall"
	"github.com/wpg/wpgctl/internal/sshx"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 初始化选项。
type Options struct {
	Site        *config.SiteConfig
	Manifest    *config.Manifest
	BasePackage   string // base 包解压目录，含 docker-install/
	DockerPackage string // legacy：含 offline_install_docker.sh 的 docker_package 目录
	LocalOnly     bool   // 仅本机，不做 SSH 分发
	SitePath    string // 用于分发到远端的 site.yaml 路径
	SSHPassword string
	SSHKeyPath  string
}

// Result 初始化结果摘要。
type Result struct {
	DockerInstalled bool
	DockerVersion   string `json:"dockerVersion,omitempty"`
	DirsCreated     []string
	PortsOpened     []int
	Skipped         []string
	Messages        []string
	NodeResults     []sshx.NodeResult `json:"nodeResults,omitempty"`
}

// Run 执行幂等初始化。可重复执行，已完成步骤自动跳过。
func Run(opts Options) (*Result, error) {
	res := &Result{}

	if opts.Site == nil {
		return nil, fmt.Errorf("site 配置不能为空")
	}

	if err := ensureDirs(opts.Site, res); err != nil {
		return res, err
	}
	if err := ensureDocker(opts, res); err != nil {
		return res, err
	}
	if err := ensureFirewall(opts, res); err != nil {
		util.Warnf("防火墙配置未完成: %v（可稍后手工放行）", err)
		res.Messages = append(res.Messages, "防火墙: "+err.Error())
	}
	if err := ensureSysctl(res); err != nil {
		util.Warnf("内核参数调整跳过: %v", err)
		res.Skipped = append(res.Skipped, "sysctl")
	}

	// 多机：将自身二进制 + site.yaml 分发到其他节点执行 init --local
	if !opts.LocalOnly && len(opts.Site.Nodes) > 1 {
		if err := distributeInit(opts, res); err != nil {
			util.Warnf("多机分发部分失败: %v", err)
			res.Messages = append(res.Messages, "ssh: "+err.Error())
		}
	}

	util.Successf("环境初始化完成")
	return res, nil
}

func distributeInit(opts Options, res *Result) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("定位本机二进制失败: %w", err)
	}
	remoteNodes := remoteNodes(opts.Site.Nodes)
	if len(remoteNodes) == 0 {
		return nil
	}
	siteRemote := "/tmp/wpgctl-site.yaml"
	// 远程节点仅做目录/防火墙等 init；Docker 离线包路径为主控机本地路径，不下发
	args := []string{"init", "--site", siteRemote, "--local"}
	// 先上传 site.yaml 到每台，再分发二进制执行
	results := make([]sshx.NodeResult, 0, len(remoteNodes))
	for _, n := range remoteNodes {
		cli, err := sshx.DialNode(n, opts.SSHPassword, opts.SSHKeyPath)
		if err != nil {
			results = append(results, sshx.NodeResult{Name: n.Name, IP: n.IP, OK: false, Message: err.Error()})
			continue
		}
		if opts.SitePath != "" {
			if err := cli.Upload(opts.SitePath, siteRemote); err != nil {
				_ = cli.Close()
				results = append(results, sshx.NodeResult{Name: n.Name, IP: n.IP, OK: false, Message: "上传 site.yaml: " + err.Error()})
				continue
			}
		}
		_ = cli.Close()
	}
	dist, err := sshx.DistributeSelf(remoteNodes, opts.SSHPassword, opts.SSHKeyPath, self, "/usr/local/bin/wpgctl", args)
	if err != nil {
		return err
	}
	results = append(results, dist...)
	res.NodeResults = results
	fail := 0
	for _, r := range results {
		if !r.OK {
			fail++
			util.Errorf("节点 %s(%s) 失败: %s", r.Name, r.IP, r.Message)
		} else {
			util.Successf("节点 %s(%s) init 完成", r.Name, r.IP)
		}
	}
	if fail > 0 {
		return fmt.Errorf("%d/%d 节点失败", fail, len(results))
	}
	return nil
}

func ensureDirs(site *config.SiteConfig, res *Result) error {
	dirs := []string{
		site.Paths.Workspace,
		site.Paths.NginxHTML,
		filepath.Join(site.Paths.Workspace, "rendered"),
		filepath.Join(site.Paths.Workspace, "bak"),
		util.PackagesDir(),
		util.StateDir(),
	}
	if runtime.GOOS == "linux" {
		dirs = append(dirs, DockerDataRoot(site))
	}
	if strings.TrimSpace(site.Paths.Logs) != "" {
		dirs = append(dirs, site.Paths.Logs)
	}
	for _, d := range dirs {
		if util.DirExists(d) {
			res.Skipped = append(res.Skipped, "dir:"+d)
			continue
		}
		if err := util.EnsureDir(d); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", d, err)
		}
		res.DirsCreated = append(res.DirsCreated, d)
		util.Infof("已创建目录: %s", d)
	}
	return nil
}

func ensureDocker(opts Options, res *Result) error {
	r := dockerx.New()
	if dockerx.Which("docker") && r.Available() {
		ver, _ := r.Version()
		res.DockerVersion = ver
		if runtime.GOOS == "windows" {
			util.Infof("Docker Desktop 已就绪: %s，跳过安装", ver)
		} else {
			util.Infof("Docker 已就绪: %s，跳过安装", ver)
		}
		res.Skipped = append(res.Skipped, "docker-install")
		return nil
	}

	if runtime.GOOS == "windows" {
		return fmt.Errorf("未检测到可用的 Docker Desktop：请安装并启动后再执行 init（Windows 不支持从 base 包离线安装 dockerd）")
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("当前 OS=%s 且 Docker 不可用，请先安装 Docker", runtime.GOOS)
	}

	dockerDir := opts.DockerPackage
	if dockerDir == "" && opts.BasePackage != "" {
		for _, p := range []string{
			filepath.Join(opts.BasePackage, "docker_package", "docker_package"),
			filepath.Join(opts.BasePackage, "docker_package"),
		} {
			if IsLegacyDockerPackage(p) {
				dockerDir = p
				util.Infof("在 base 包内发现 Docker 离线目录: %s", p)
				break
			}
		}
	}
	if dockerDir != "" {
		return installLegacyDocker(dockerDir, opts.Site, res)
	}

	if opts.BasePackage == "" {
		return fmt.Errorf("未安装 Docker：请提供 --docker-package（middleware docker_package 目录）或 --base（含 docker-install/）")
	}

	arch := runtime.GOARCH
	src := filepath.Join(opts.BasePackage, "docker-install", arch)
	if !util.DirExists(src) {
		return fmt.Errorf("base 包中找不到 docker-install/%s", arch)
	}

	bins := []string{"docker", "dockerd", "containerd", "containerd-shim-runc-v2", "runc", "ctr"}
	destBin := "/usr/local/bin"
	for _, b := range bins {
		srcFile := filepath.Join(src, b)
		if !util.FileExists(srcFile) {
			util.Warnf("缺少二进制 %s，跳过", srcFile)
			continue
		}
		dest := filepath.Join(destBin, b)
		if err := copyFile(srcFile, dest); err != nil {
			return err
		}
		_ = os.Chmod(dest, 0o755)
	}

	// compose plugin
	pluginSrc := filepath.Join(src, "docker-compose")
	pluginDestDir := "/usr/local/lib/docker/cli-plugins"
	if util.FileExists(pluginSrc) {
		_ = util.EnsureDir(pluginDestDir)
		dest := filepath.Join(pluginDestDir, "docker-compose")
		if err := copyFile(pluginSrc, dest); err != nil {
			return err
		}
		_ = os.Chmod(dest, 0o755)
	}

	// systemd unit
	unitSrc := filepath.Join(opts.BasePackage, "docker-install", "systemd")
	if util.DirExists(unitSrc) {
		_ = copyGlob(unitSrc, "/etc/systemd/system")
		_ = exec.Command("systemctl", "daemon-reload").Run()
		_ = exec.Command("systemctl", "enable", "--now", "docker").Run()
	}

	// daemon.json
	dataRoot := filepath.Join(opts.Site.Paths.Workspace, "docker-data")
	_ = util.EnsureDir(dataRoot)
	daemonJSON := fmt.Sprintf(`{
  "data-root": "%s",
  "log-driver": "json-file",
  "log-opts": {"max-size": "100m", "max-file": "3"}
}`, dataRoot)
	_ = util.EnsureDir("/etc/docker")
	if err := os.WriteFile("/etc/docker/daemon.json", []byte(daemonJSON), 0o644); err != nil {
		util.Warnf("写入 daemon.json 失败: %v", err)
	}

	res.DockerInstalled = true
	if ver, err := r.Version(); err == nil && ver != "" {
		res.DockerVersion = ver
		util.Successf("Docker 离线安装完成: %s", ver)
	} else {
		util.Successf("Docker 离线安装完成")
	}
	return nil
}

func ensureFirewall(opts Options, res *Result) error {
	if opts.Manifest == nil || opts.Site == nil {
		res.Skipped = append(res.Skipped, "firewall")
		return nil
	}
	if runtime.GOOS != "linux" {
		res.Skipped = append(res.Skipped, "firewall")
		return nil
	}
	ports := opts.Manifest.Ports(opts.Site.Profiles)
	fr, err := fw.OpenPorts(ports)
	if err != nil {
		util.Warnf("防火墙配置未完成: %v", err)
		res.Messages = append(res.Messages, "firewall: "+err.Error())
	}
	if fr != nil {
		res.PortsOpened = append(res.PortsOpened, fr.Opened...)
		res.PortsOpened = append(res.PortsOpened, fr.Skipped...)
	}
	if fr != nil && fr.Firewall == "none" {
		res.Skipped = append(res.Skipped, "firewall")
	}
	return nil
}

func ensureSysctl(res *Result) error {
	if runtime.GOOS != "linux" {
		res.Skipped = append(res.Skipped, "sysctl")
		return nil
	}
	content := "net.core.somaxconn = 65535\nvm.max_map_count = 262144\n"
	path := "/etc/sysctl.d/99-wpgctl.conf"
	if util.FileExists(path) {
		res.Skipped = append(res.Skipped, "sysctl")
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	_ = exec.Command("sysctl", "--system").Run()
	util.Infof("已写入内核参数: %s", path)
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

func copyGlob(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	_ = util.EnsureDir(destDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(srcDir, e.Name()), filepath.Join(destDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
