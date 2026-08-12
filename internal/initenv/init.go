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
	"github.com/wpg/wpgctl/internal/sshx"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 初始化选项。
type Options struct {
	Site        *config.SiteConfig
	Manifest    *config.Manifest
	BasePackage string // base 包解压目录，含 docker-install/
	LocalOnly   bool   // 仅本机，不做 SSH 分发
	SitePath    string // 用于分发到远端的 site.yaml 路径
	SSHPassword string
	SSHKeyPath  string
}

// Result 初始化结果摘要。
type Result struct {
	DockerInstalled bool
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
	remoteNodes := make([]config.Node, 0, len(opts.Site.Nodes))
	for _, n := range opts.Site.Nodes {
		// 跳过本机 IP 粗判：调用方也可传 LocalOnly
		remoteNodes = append(remoteNodes, n)
	}
	siteRemote := "/tmp/wpgctl-site.yaml"
	args := []string{"init", "--site", siteRemote, "--local"}
	if opts.BasePackage != "" {
		args = append(args, "--base", opts.BasePackage)
	}
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
		site.Paths.Logs,
		site.Paths.NginxHTML,
		filepath.Join(site.Paths.Workspace, "rendered"),
		filepath.Join(site.Paths.Workspace, "bak"),
		util.PackagesDir(),
		util.StateDir(),
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
		util.Infof("Docker 已就绪: %s，跳过安装", ver)
		res.Skipped = append(res.Skipped, "docker-install")
		return nil
	}

	if runtime.GOOS != "linux" {
		msg := fmt.Sprintf("当前 OS=%s，跳过 Docker 离线安装（仅 Linux 现场执行）", runtime.GOOS)
		util.Warnf(msg)
		res.Skipped = append(res.Skipped, "docker-install")
		res.Messages = append(res.Messages, msg)
		return nil
	}

	if opts.BasePackage == "" {
		return fmt.Errorf("未安装 Docker 且未提供 --base 包路径，无法离线安装")
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
	util.Successf("Docker 离线安装完成")
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
	fw := detectFirewall()
	if fw == "none" {
		util.Warnf("未检测到 firewalld/iptables/ufw，跳过端口放行")
		res.Skipped = append(res.Skipped, "firewall")
		return nil
	}
	for _, p := range ports {
		if err := openPort(fw, p); err != nil {
			util.Warnf("放行端口 %d 失败: %v", p, err)
			continue
		}
		res.PortsOpened = append(res.PortsOpened, p)
		util.Infof("已放行端口 %d (%s)", p, fw)
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

func detectFirewall() string {
	if dockerx.Which("firewall-cmd") {
		if exec.Command("firewall-cmd", "--state").Run() == nil {
			return "firewalld"
		}
	}
	if dockerx.Which("ufw") {
		return "ufw"
	}
	if dockerx.Which("iptables") {
		return "iptables"
	}
	return "none"
}

func openPort(fw string, port int) error {
	switch fw {
	case "firewalld":
		cmd := exec.Command("firewall-cmd", "--permanent",
			fmt.Sprintf("--add-port=%d/tcp", port))
		if out, err := cmd.CombinedOutput(); err != nil {
			if strings.Contains(string(out), "ALREADY_ENABLED") {
				return nil
			}
			return fmt.Errorf("%s", strings.TrimSpace(string(out)))
		}
		return exec.Command("firewall-cmd", "--reload").Run()
	case "ufw":
		return exec.Command("ufw", "allow", fmt.Sprintf("%d/tcp", port)).Run()
	case "iptables":
		// 幂等：先查后加
		check := exec.Command("iptables", "-C", "INPUT", "-p", "tcp", "--dport",
			fmt.Sprintf("%d", port), "-j", "ACCEPT")
		if check.Run() == nil {
			return nil
		}
		return exec.Command("iptables", "-A", "INPUT", "-p", "tcp", "--dport",
			fmt.Sprintf("%d", port), "-j", "ACCEPT").Run()
	default:
		return fmt.Errorf("未知防火墙: %s", fw)
	}
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
