// Package firewall 在现场 Linux 上放行服务端口（firewalld / ufw / iptables）。
// Init 启动防火墙时放行 SSH 与控制台端口；业务端口在各模块 compose up 前按 compose/.env 解析后再放行。
package firewall

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/util"
)

// SSHPort 现场 SSH 端口。启用 ufw/firewalld 前必须先放行，否则可能立刻锁死远程会话。
const SSHPort = 22

// UIPort 控制台默认 HTTP 端口。启动防火墙前必须放行，否则现场浏览器立刻打不开向导。
const UIPort = 9527

// OpenOptions 放行端口选项。
type OpenOptions struct {
	Ports     []int
	AutoStart bool // 防火墙未运行时尝试启动（启动前会写入 SSH 22 与控制台端口）
}

// Status 防火墙类型与运行状态。
type Status struct {
	Tool    string `json:"tool"` // firewalld | ufw | iptables | none
	Running bool   `json:"running"`
	Detail  string `json:"detail"` // running / inactive / not running ...
}

// Result 放行结果。
type Result struct {
	Firewall string   `json:"firewall"`
	Status   Status   `json:"status"`
	Ports    []int    `json:"ports"`
	Opened   []int    `json:"opened"`
	Skipped  []int    `json:"skipped"` // 已存在规则
	Failed   []int    `json:"failed"`
	Messages []string `json:"messages,omitempty"`
	Reloaded bool     `json:"reloaded"`
}

// Inspect 检测防火墙工具及是否已开启。
func Inspect() Status {
	if dockerx.Which("firewall-cmd") {
		out, err := exec.Command("firewall-cmd", "--state").CombinedOutput()
		state := strings.TrimSpace(string(out))
		if err != nil && state == "" {
			state = err.Error()
		}
		running := err == nil && strings.EqualFold(state, "running")
		return Status{Tool: "firewalld", Running: running, Detail: state}
	}
	if dockerx.Which("ufw") {
		out, _ := exec.Command("ufw", "status").CombinedOutput()
		text := string(out)
		if strings.Contains(text, "Status: active") {
			return Status{Tool: "ufw", Running: true, Detail: "active"}
		}
		detail := "inactive"
		if strings.Contains(text, "Status: inactive") {
			detail = "inactive"
		} else if trimmed := strings.TrimSpace(text); trimmed != "" {
			detail = trimmed
		}
		return Status{Tool: "ufw", Running: false, Detail: detail}
	}
	if dockerx.Which("iptables") {
		return Status{Tool: "iptables", Running: true, Detail: "available"}
	}
	return Status{Tool: "none", Running: false, Detail: "not found"}
}

// Detect 检测当前防火墙工具（兼容旧调用）。
func Detect() string {
	return Inspect().Tool
}

// ProtectPorts 启动防火墙前必须放行的端口：SSH 与控制台，避免远程会话和向导同时断。
func ProtectPorts() []int {
	return []int{SSHPort, UIPort}
}

// ParseListenPort 从 --listen 地址取出 TCP 端口；无效则返回 0。
func ParseListenPort(addr string) int {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return 0
	}
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		n, convErr := strconv.Atoi(addr)
		if convErr != nil || n <= 0 || n > 65535 {
			return 0
		}
		return n
	}
	n, convErr := strconv.Atoi(p)
	if convErr != nil || n <= 0 || n > 65535 {
		return 0
	}
	return n
}

// EnsureReady 检查并启动防火墙，且放行 SSH 22 与控制台 9527。Init 使用；不放行业务端口。
func EnsureReady() (*Result, error) {
	return EnsureReadyPorts(nil)
}

// EnsureReadyPorts 在 ProtectPorts 之外再放行 extra（例如自定义 --listen 端口）。
func EnsureReadyPorts(extra []int) (*Result, error) {
	return OpenPortsWithOptions(OpenOptions{
		Ports:     uniquePorts(append(ProtectPorts(), extra...)),
		AutoStart: true,
	})
}

// Start 启动 firewalld / ufw（需 root 或 sudo）。
// 启用前写入 SSH 22 与控制台 9527，避免默认拒绝入站后远程会话和向导立刻断开。
func Start() (*Status, error) {
	return StartWithExtra(nil)
}

// StartWithExtra 启动防火墙，并额外放行 extra 中的 TCP 端口。
func StartWithExtra(extra []int) (*Status, error) {
	st := Inspect()
	if st.Tool == "none" {
		return &st, fmt.Errorf("未检测到 firewalld / ufw")
	}
	ports := uniquePorts(append(ProtectPorts(), extra...))
	if st.Running {
		writeProtectPorts(st.Tool, ports)
		after := Inspect()
		return &after, nil
	}
	writeProtectPorts(st.Tool, ports)
	switch st.Tool {
	case "firewalld":
		if out, err := util.RunPrivileged("systemctl", "start", "firewalld"); err != nil {
			return &st, fmt.Errorf("启动 firewalld 失败: %w\n%s", err, strings.TrimSpace(string(out)))
		}
		_, _ = util.RunPrivileged("systemctl", "enable", "firewalld")
	case "ufw":
		if out, err := util.RunPrivileged("ufw", "--force", "enable"); err != nil {
			return &st, fmt.Errorf("启用 ufw 失败: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	default:
		return &st, fmt.Errorf("iptables 模式请手工启动/配置")
	}
	after := Inspect()
	writeProtectPorts(after.Tool, ports)
	if err := dockerx.RestartDaemon(); err != nil {
		util.Warnf("防火墙已启动，但重启 docker 失败（compose 可能报 SKIP DNAT）: %v", err)
	}
	return &after, nil
}

func writeProtectPorts(tool string, ports []int) {
	for _, p := range ports {
		if p <= 0 {
			continue
		}
		if err := addPort(tool, p); err != nil && !isAlready(err) {
			util.Warnf("写入 %s 放行 %d/tcp: %v", tool, p, err)
		}
	}
}

// Reload 让 firewalld 的 permanent 规则生效。ufw 规则即时生效，调用成功但不改配置。
// firewalld reload 会冲掉 Docker 的 nat/DOCKER 链，完成后必须重启 docker。
func Reload() (*Status, error) {
	st := Inspect()
	switch st.Tool {
	case "firewalld":
		if !st.Running {
			return &st, fmt.Errorf("firewalld 未运行，请先启动再 reload")
		}
		out, err := util.RunPrivileged("firewall-cmd", "--reload")
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return &st, fmt.Errorf("firewall-cmd --reload 失败: %s", msg)
		}
		if err := dockerx.RestartDaemon(); err != nil {
			after := Inspect()
			return &after, fmt.Errorf("firewalld 已 reload，但重启 docker 失败: %w", err)
		}
		after := Inspect()
		return &after, nil
	case "ufw":
		return &st, nil
	case "iptables":
		return &st, fmt.Errorf("iptables 无统一 reload，请用 iptables-save 持久化")
	default:
		return &st, fmt.Errorf("未检测到 firewalld / ufw")
	}
}

// OpenPorts 检查防火墙已开启 → 幂等放行 TCP 端口 → firewalld 最后统一 reload。
func OpenPorts(ports []int) (*Result, error) {
	return OpenPortsWithOptions(OpenOptions{Ports: ports})
}

// OpenPortsWithOptions 支持自动启动防火墙后再放行。
// 未运行且 AutoStart 时：先把 SSH 22、控制台端口与待放行端口写入规则，再启动。
func OpenPortsWithOptions(opts OpenOptions) (*Result, error) {
	st := Inspect()
	res := &Result{
		Ports:    uniquePorts(opts.Ports),
		Firewall: st.Tool,
		Status:   st,
	}
	res.Messages = append(res.Messages,
		fmt.Sprintf("检查防火墙: %s (%s)", st.Tool, st.Detail))

	if st.Tool == "none" {
		return res, fmt.Errorf("未检测到 firewalld / ufw / iptables，请手工放行端口 %v", res.Ports)
	}
	if !st.Running {
		if opts.AutoStart {
			res.Ports = uniquePorts(append(ProtectPorts(), res.Ports...))
			res.Messages = append(res.Messages, "防火墙未运行，先写入 SSH 22、控制台端口与待放行端口再启动…")
			writePortsBeforeStart(st.Tool, res.Ports, res)
			started, err := Start()
			if err != nil {
				return res, fmt.Errorf("防火墙未开启且启动失败: %w", err)
			}
			st = *started
			res.Firewall = st.Tool
			res.Status = st
			res.Messages = append(res.Messages, fmt.Sprintf("防火墙已启动: %s", st.Detail))
		} else {
			return res, fmt.Errorf("防火墙 %s 未开启（%s），请先启动后再放行（如 systemctl start firewalld 或 ufw enable）",
				st.Tool, st.Detail)
		}
	}

	res.Messages = append(res.Messages, "防火墙已开启，开始放行端口…")
	for _, p := range res.Ports {
		if p <= 0 {
			continue
		}
		if err := addPort(st.Tool, p); err != nil {
			if strings.Contains(err.Error(), "ALREADY") {
				res.Skipped = append(res.Skipped, p)
				continue
			}
			res.Failed = append(res.Failed, p)
			res.Messages = append(res.Messages, fmt.Sprintf("端口 %d: %v", p, err))
			util.Warnf("放行端口 %d 失败: %v", p, err)
			continue
		}
		res.Opened = append(res.Opened, p)
		util.Infof("已添加放行规则 %d (%s)", p, st.Tool)
	}
	if len(res.Failed) > 0 {
		return res, fmt.Errorf("%d 个端口放行失败", len(res.Failed))
	}

	// firewalld 已运行时用 --add-port 即时生效，避免 --reload 冲掉 Docker 的 nat/DOCKER 链。
	// 刚启动的情况由 Start() 负责重启 docker。
	if st.Tool == "firewalld" {
		res.Messages = append(res.Messages, "firewalld 已写入 permanent + 运行时规则（未 reload，以免打断 Docker 网络）")
	}

	return res, nil
}

// writePortsBeforeStart 在启用防火墙前写入规则。ufw 的 allow 在 inactive 时即可生效；
// firewalld --permanent 在部分系统上服务未启动也会失败，失败只记日志，启动后再补写。
func writePortsBeforeStart(tool string, ports []int, res *Result) {
	if tool != "ufw" && tool != "firewalld" {
		return
	}
	for _, p := range ports {
		if p <= 0 {
			continue
		}
		if err := addPort(tool, p); err != nil && !strings.Contains(err.Error(), "ALREADY") {
			res.Messages = append(res.Messages, fmt.Sprintf("启动前写入端口 %d: %v", p, err))
		}
	}
}

// addPort 添加放行规则。firewalld 同时写 permanent 与运行时，避免 --reload。
func addPort(fw string, port int) error {
	switch fw {
	case "firewalld":
		permErr := addFirewalldPort(port, true)
		runErr := addFirewalldPort(port, false)
		if isAlready(permErr) && (runErr == nil || isAlready(runErr)) {
			return fmt.Errorf("ALREADY")
		}
		if permErr != nil && !isAlready(permErr) {
			return permErr
		}
		return nil
	case "ufw":
		if out, _ := util.RunPrivileged("ufw", "status"); strings.Contains(string(out), fmt.Sprintf("%d/tcp", port)) {
			return fmt.Errorf("ALREADY")
		}
		out, err := util.RunPrivileged("ufw", "allow", fmt.Sprintf("%d/tcp", port))
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("%s", msg)
		}
		return nil
	case "iptables":
		if _, err := util.RunPrivileged("iptables", "-C", "INPUT", "-p", "tcp", "--dport",
			fmt.Sprintf("%d", port), "-j", "ACCEPT"); err == nil {
			return fmt.Errorf("ALREADY")
		}
		out, err := util.RunPrivileged("iptables", "-A", "INPUT", "-p", "tcp", "--dport",
			fmt.Sprintf("%d", port), "-j", "ACCEPT")
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("%s", msg)
		}
		return nil
	default:
		return fmt.Errorf("未知防火墙: %s", fw)
	}
}

func addFirewalldPort(port int, permanent bool) error {
	args := []string{}
	if permanent {
		args = append(args, "--permanent")
	}
	args = append(args, fmt.Sprintf("--add-port=%d/tcp", port))
	out, err := util.RunPrivileged("firewall-cmd", args...)
	if err != nil {
		s := string(out)
		if strings.Contains(s, "ALREADY_ENABLED") {
			return fmt.Errorf("ALREADY")
		}
		msg := strings.TrimSpace(s)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func isAlready(err error) bool {
	return err != nil && strings.Contains(err.Error(), "ALREADY")
}

func uniquePorts(in []int) []int {
	seen := map[int]struct{}{}
	var out []int
	for _, p := range in {
		if p <= 0 {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
