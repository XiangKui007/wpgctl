// Package firewall 在现场 Linux 上放行服务端口（firewalld / ufw / iptables）。
package firewall

import (
	"fmt"
	"os/exec"
	"strings"

	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/util"
)

// OpenOptions 放行端口选项。
type OpenOptions struct {
	Ports     []int
	AutoStart bool // 防火墙未运行时尝试启动
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

// Start 启动 firewalld / ufw（需 root 或 sudo）。
func Start() (*Status, error) {
	st := Inspect()
	if st.Tool == "none" {
		return &st, fmt.Errorf("未检测到 firewalld / ufw")
	}
	if st.Running {
		return &st, nil
	}
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
	return &after, nil
}

// OpenPorts 检查防火墙已开启 → 幂等放行 TCP 端口 → firewalld 最后统一 reload。
func OpenPorts(ports []int) (*Result, error) {
	return OpenPortsWithOptions(OpenOptions{Ports: ports})
}

// OpenPortsWithOptions 支持自动启动防火墙后再放行。
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
			res.Messages = append(res.Messages, "防火墙未运行，尝试启动…")
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

	if st.Tool == "firewalld" {
		res.Messages = append(res.Messages, "执行 firewall-cmd --reload …")
		if out, err := util.RunPrivileged("firewall-cmd", "--reload"); err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return res, fmt.Errorf("端口规则已写入但 reload 失败: %s", msg)
		}
		res.Reloaded = true
		res.Messages = append(res.Messages, "firewall-cmd --reload 完成")
		util.Successf("firewalld 已 reload")
	}

	return res, nil
}

// addPort 添加放行规则（firewalld 仅 --permanent，不在此步 reload；非 root 走 sudo）。
func addPort(fw string, port int) error {
	switch fw {
	case "firewalld":
		out, err := util.RunPrivileged("firewall-cmd", "--permanent",
			fmt.Sprintf("--add-port=%d/tcp", port))
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
