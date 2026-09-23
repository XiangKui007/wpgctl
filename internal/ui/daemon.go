package ui

// 本文件把 Web 控制台从「前台占着终端」封装成可后台启停。
// Linux 已安装 systemd 单元时走 systemctl；否则拉起独立进程并写 ~/.wpgctl/ui.pid。

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/wpg/wpgctl/internal/util"
)

const (
	// systemdUnitName 现场开机自启单元名。
	systemdUnitName = "wpgctl-ui"
	systemdUnitPath = "/etc/systemd/system/wpgctl-ui.service"
	vaultEnvFile    = "/etc/wpgctl/ui.env"
)

// DaemonOptions 后台启停控制台时用的监听地址与 site.yaml。
type DaemonOptions struct {
	Listen   string // 空则 DefaultListen()
	SitePath string // 空则 site.yaml；后台启动会转成绝对路径
}

// DaemonState 后台控制台快照（status / 幂等 start）。
type DaemonState struct {
	Running bool
	PID     int
	Listen  string
	LogPath string
	Via     string // process：独立进程；systemd：系统服务
}

// StartDaemon 后台启动 Web 控制台（已在跑则直接提示访问地址）。
// Linux 若已 wpgctl ui install，走 systemctl；否则拉起独立进程并写 pid 文件。
func StartDaemon(opts DaemonOptions) error {
	opts = normalizeDaemonOpts(opts)
	st, err := InspectDaemon()
	if err != nil {
		return err
	}
	if st.Running {
		util.Infof("Web 控制台已在后台运行 (pid=%d, %s)", st.PID, st.Via)
		printAccessHints(st.Listen)
		printStopHint(st)
		return nil
	}

	if systemdManaged() {
		if out, err := util.RunPrivileged("systemctl", "start", systemdUnitName); err != nil {
			return fmt.Errorf("systemctl start %s 失败: %s: %w", systemdUnitName, strings.TrimSpace(string(out)), err)
		}
		util.Successf("已通过 systemd 启动 %s", systemdUnitName)
		printAccessHints(opts.Listen)
		printStopHint(DaemonState{Running: true, Via: "systemd", Listen: opts.Listen})
		return nil
	}

	return startPidDaemon(opts)
}

// StopDaemon 停止后台控制台（systemd 或 pid 进程）；未运行视为成功。
func StopDaemon() error {
	st, err := InspectDaemon()
	if err != nil {
		return err
	}
	if !st.Running {
		util.Infof("Web 控制台未在后台运行")
		return nil
	}
	if st.Via == "systemd" {
		if out, err := util.RunPrivileged("systemctl", "stop", systemdUnitName); err != nil {
			return fmt.Errorf("systemctl stop %s 失败: %s: %w", systemdUnitName, strings.TrimSpace(string(out)), err)
		}
		util.Successf("已停止 systemd 服务 %s", systemdUnitName)
		return nil
	}
	if err := stopProcess(st.PID); err != nil {
		return err
	}
	_ = os.Remove(pidPath())
	_ = os.Remove(listenMetaPath())
	util.Successf("已停止后台控制台 (pid=%d)", st.PID)
	return nil
}

// InspectDaemon 探测后台控制台是否在跑，以及托管方式。
func InspectDaemon() (DaemonState, error) {
	st := DaemonState{Listen: DefaultListen(), LogPath: daemonLogPath()}
	if systemdManaged() && systemdActive() {
		st.Running = true
		st.Via = "systemd"
		st.PID = systemdMainPID()
		if listen := strings.TrimSpace(readListenMeta()); listen != "" {
			st.Listen = listen
		}
		st.LogPath = "journalctl -u " + systemdUnitName + " -f"
		return st, nil
	}
	pid, ok := runningFromPidFile()
	if !ok {
		return st, nil
	}
	st.Running = true
	st.Via = "process"
	st.PID = pid
	if listen := strings.TrimSpace(readListenMeta()); listen != "" {
		st.Listen = listen
	}
	return st, nil
}

// PrintDaemonStatus 把 InspectDaemon 结果打成现场可读的中文状态。
func PrintDaemonStatus() error {
	st, err := InspectDaemon()
	if err != nil {
		return err
	}
	if !st.Running {
		util.Infof("Web 控制台未在后台运行。启动: wpgctl ui start")
		if systemdUnitInstalled() {
			util.Infof("已安装 systemd 单元 %s，可用 wpgctl ui start 拉起", systemdUnitName)
		}
		return nil
	}
	util.Successf("Web 控制台后台运行中 (pid=%d, %s)", st.PID, st.Via)
	printAccessHints(st.Listen)
	printStopHint(st)
	return nil
}

// InstallDaemon 写入 systemd 单元、enable --now，开机自启。仅 Linux。
func InstallDaemon(opts DaemonOptions) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("仅 Linux 支持 systemd；Windows 请用 wpgctl ui start 后台运行")
	}
	opts = normalizeDaemonOpts(opts)
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法解析当前二进制路径: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	site, err := filepath.Abs(opts.SitePath)
	if err != nil {
		return fmt.Errorf("无法解析 site.yaml 路径: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		wd = filepath.Dir(site)
	}

	// 避免 pid 进程与 systemd 抢 9527。
	if pid, ok := runningFromPidFile(); ok {
		util.Infof("先停止已有后台进程 pid=%d，再交给 systemd", pid)
		_ = stopProcess(pid)
		_ = os.Remove(pidPath())
	}

	unit := systemdUnit(exe, opts.Listen, site, wd)
	tmp := filepath.Join(os.TempDir(), systemdUnitName+".service")
	if err := os.WriteFile(tmp, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("写入临时单元文件失败: %w", err)
	}
	if out, err := util.RunPrivileged("install", "-m", "0644", tmp, systemdUnitPath); err != nil {
		return fmt.Errorf("安装 %s 失败（需要 root/sudo）: %s: %w", systemdUnitPath, strings.TrimSpace(string(out)), err)
	}
	_ = os.Remove(tmp)
	if err := writeListenMeta(opts.Listen); err != nil {
		util.Warnf("记录监听地址失败: %v", err)
	}

	if out, err := util.RunPrivileged("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload 失败: %s: %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := util.RunPrivileged("systemctl", "enable", "--now", systemdUnitName); err != nil {
		return fmt.Errorf("systemctl enable --now %s 失败: %s: %w", systemdUnitName, strings.TrimSpace(string(out)), err)
	}
	util.Successf("已安装并启动 systemd 服务 %s（开机自启）", systemdUnitName)
	util.Infof("如需 WPGCTL_VAULT_KEY，写入 %s 后执行: systemctl restart %s", vaultEnvFile, systemdUnitName)
	printAccessHints(opts.Listen)
	printStopHint(DaemonState{Running: true, Via: "systemd", Listen: opts.Listen})
	return nil
}

// UninstallDaemon 停掉并删除 systemd 单元；未安装视为成功。
func UninstallDaemon() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("仅 Linux 支持 systemd")
	}
	if !systemdUnitInstalled() {
		util.Infof("未安装 systemd 单元 %s", systemdUnitName)
		return nil
	}
	_, _ = util.RunPrivileged("systemctl", "disable", "--now", systemdUnitName)
	if out, err := util.RunPrivileged("rm", "-f", systemdUnitPath); err != nil {
		return fmt.Errorf("删除 %s 失败: %s: %w", systemdUnitPath, strings.TrimSpace(string(out)), err)
	}
	_, _ = util.RunPrivileged("systemctl", "daemon-reload")
	_ = os.Remove(listenMetaPath())
	util.Successf("已卸载 systemd 服务 %s", systemdUnitName)
	return nil
}

func normalizeDaemonOpts(opts DaemonOptions) DaemonOptions {
	if strings.TrimSpace(opts.Listen) == "" {
		opts.Listen = DefaultListen()
	}
	if strings.TrimSpace(opts.SitePath) == "" {
		opts.SitePath = "site.yaml"
	}
	return opts
}

func startPidDaemon(opts DaemonOptions) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法解析当前二进制路径: %w", err)
	}
	site, err := filepath.Abs(opts.SitePath)
	if err != nil {
		return fmt.Errorf("无法解析 site.yaml 路径: %w", err)
	}
	if err := util.EnsureDir(filepath.Dir(daemonLogPath())); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	logF, err := os.OpenFile(daemonLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer logF.Close()

	cmd := exec.Command(exe, "ui", "--listen", opts.Listen, "--site", site)
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.Stdin = nil
	cmd.Env = os.Environ()
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("后台启动失败: %w", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	if err := writePID(pid); err != nil {
		_ = stopProcess(pid)
		return fmt.Errorf("写入 pid 文件失败: %w", err)
	}
	if err := writeListenMeta(opts.Listen); err != nil {
		util.Warnf("记录监听地址失败: %v", err)
	}
	if err := waitDaemonUp(pid); err != nil {
		_ = os.Remove(pidPath())
		return err
	}
	util.Successf("Web 控制台已在后台运行 (pid=%d)", pid)
	printAccessHints(opts.Listen)
	printStopHint(DaemonState{Running: true, Via: "process", Listen: opts.Listen, PID: pid, LogPath: daemonLogPath()})
	return nil
}

func waitDaemonUp(pid int) error {
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return fmt.Errorf("后台进程已退出，请查看日志: %s\n%s", daemonLogPath(), tailFile(daemonLogPath(), 12))
		}
		time.Sleep(80 * time.Millisecond)
	}
	if !processAlive(pid) {
		return fmt.Errorf("后台进程已退出，请查看日志: %s\n%s", daemonLogPath(), tailFile(daemonLogPath(), 12))
	}
	return nil
}

func printStopHint(st DaemonState) {
	util.Infof("停止: wpgctl ui stop")
	if st.Via == "systemd" {
		util.Infof("日志: journalctl -u %s -f", systemdUnitName)
		return
	}
	if st.LogPath != "" {
		util.Infof("日志: %s", st.LogPath)
		return
	}
	util.Infof("日志: %s", daemonLogPath())
}

func systemdManaged() bool {
	return runtime.GOOS == "linux" && systemdUnitInstalled()
}

func systemdUnitInstalled() bool {
	_, err := os.Stat(systemdUnitPath)
	return err == nil
}

func systemdActive() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	return exec.Command("systemctl", "is-active", "--quiet", systemdUnitName).Run() == nil
}

func systemdMainPID() int {
	out, err := exec.Command("systemctl", "show", systemdUnitName, "-p", "MainPID", "--value").Output()
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return pid
}

func runningFromPidFile() (int, bool) {
	pid, err := readPID()
	if err != nil || pid <= 0 {
		return 0, false
	}
	if !processAlive(pid) || !isOurDaemonProcess(pid) {
		_ = os.Remove(pidPath())
		return 0, false
	}
	return pid, true
}

func pidPath() string {
	return filepath.Join(util.HomeDir(), "ui.pid")
}

func listenMetaPath() string {
	return filepath.Join(util.HomeDir(), "ui.listen")
}

func daemonLogPath() string {
	return filepath.Join(util.HomeDir(), "logs", "ui.log")
}

func writePID(pid int) error {
	if err := util.EnsureDir(util.HomeDir()); err != nil {
		return err
	}
	return os.WriteFile(pidPath(), []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

func readPID() (int, error) {
	b, err := os.ReadFile(pidPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

func writeListenMeta(listen string) error {
	if err := util.EnsureDir(util.HomeDir()); err != nil {
		return err
	}
	return os.WriteFile(listenMetaPath(), []byte(listen+"\n"), 0o644)
}

func readListenMeta() string {
	b, err := os.ReadFile(listenMetaPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// systemdUnit 生成 wpgctl-ui.service 正文（供 install 与单测）。
func systemdUnit(exe, listen, site, workDir string) string {
	var b strings.Builder
	b.WriteString("# 由 wpgctl ui install 生成。口令解密可把 WPGCTL_VAULT_KEY 写入 " + vaultEnvFile + "\n")
	b.WriteString("[Unit]\n")
	b.WriteString("Description=wpgctl Web 控制台\n")
	b.WriteString("After=network.target\n\n")
	b.WriteString("[Service]\n")
	b.WriteString("Type=simple\n")
	b.WriteString("ExecStart=" + systemdQuote(exe) + " ui --listen " + systemdQuote(listen) + " --site " + systemdQuote(site) + "\n")
	if strings.TrimSpace(workDir) != "" {
		b.WriteString("WorkingDirectory=" + systemdQuote(workDir) + "\n")
	}
	b.WriteString("EnvironmentFile=-" + vaultEnvFile + "\n")
	b.WriteString("Restart=on-failure\n")
	b.WriteString("RestartSec=3\n\n")
	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=multi-user.target\n")
	return b.String()
}

func systemdQuote(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"'\\") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func tailFile(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return "(无日志)"
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
