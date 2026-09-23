package ui

import (
	"fmt"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	fw "github.com/wpg/wpgctl/internal/firewall"
	"github.com/wpg/wpgctl/internal/sshx"
)

// snapshotRemoteFirewall 通过 SSH 查看从机防火墙状态与已放行端口。
func snapshotRemoteFirewall(node config.Node, password, keyPath string) (fw.View, error) {
	sess, err := sshx.Open(node, password, keyPath, nil)
	if err != nil {
		return fw.View{}, fmt.Errorf("SSH 连接 %s 失败: %w", node.IP, err)
	}
	defer sess.Close()

	run := func(cmd string) string {
		var b strings.Builder
		_ = sess.RunPrivileged(cmd, func(line string) {
			b.WriteString(line)
			b.WriteByte('\n')
		})
		return strings.TrimSpace(b.String())
	}

	state := run("firewall-cmd --state 2>/dev/null || true")
	if state != "" && !strings.Contains(strings.ToLower(state), "not found") &&
		!strings.Contains(strings.ToLower(state), "command not found") {
		running := strings.EqualFold(firstLine(state), "running")
		st := fw.Status{Tool: "firewalld", Running: running, Detail: firstLine(state)}
		v := fw.ViewFromStatus(st, "linux")
		if running {
			ports := fw.ParseFirewalldListPorts(run("firewall-cmd --list-ports 2>/dev/null || true"))
			svcs := fw.ParseFirewalldServices(run("firewall-cmd --list-services 2>/dev/null || true"))
			v.OpenPorts = fw.MergeOpenPorts(ports, svcs)
			v.Hint = "从机 firewalld 已运行"
		} else {
			v.Hint = "从机 firewalld 未运行，请执行 Init"
		}
		v.CanStart = false
		v.CanReload = false
		return v, nil
	}

	ufw := run("ufw status 2>/dev/null || true")
	if strings.Contains(ufw, "Status:") {
		running := strings.Contains(ufw, "Status: active")
		detail := "inactive"
		if running {
			detail = "active"
		}
		st := fw.Status{Tool: "ufw", Running: running, Detail: detail}
		v := fw.ViewFromStatus(st, "linux")
		if running {
			v.OpenPorts = fw.ParseUfwStatus(ufw)
			v.Hint = "从机 ufw 已启用"
		} else {
			v.Hint = "从机 ufw 未启用，请执行 Init"
		}
		v.CanStart = false
		v.CanReload = false
		return v, nil
	}

	v := fw.ViewFromStatus(fw.Status{Tool: "none", Running: false, Detail: "not found"}, "linux")
	v.CanStart = false
	v.Hint = "从机未检测到 firewalld / ufw"
	return v, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
