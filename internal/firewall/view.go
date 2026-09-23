package firewall

import "runtime"

// View 给控制台展示的防火墙快照：能否启动、能否 reload，以及已运行时当前放行的端口。
type View struct {
	Tool      string     `json:"tool"`
	Running   bool       `json:"running"`
	Detail    string     `json:"detail"`
	CanStart  bool       `json:"canStart"`  // firewalld / ufw 可点「启动」
	CanReload bool       `json:"canReload"` // 仅 firewalld 已运行时需要 reload
	SSHPort   int        `json:"sshPort"`
	UIPort    int        `json:"uiPort"` // 控制台 HTTP 端口，启动防火墙时一并放行
	Hint      string     `json:"hint"`
	OpenPorts []OpenPort `json:"openPorts,omitempty"` // 已运行时当前放行的端口/服务
}

// Snapshot 检测当前防火墙并生成向导可用的状态。
func Snapshot() View {
	st := Inspect()
	v := viewFromStatus(st, runtime.GOOS)
	v.OpenPorts = ListOpenPorts(st)
	return v
}

// ViewFromStatus 由检测结果生成向导快照（不含 OpenPorts，调用方按需补）。
func ViewFromStatus(st Status, goos string) View {
	return viewFromStatus(st, goos)
}

func viewFromStatus(st Status, goos string) View {
	v := View{
		Tool:    st.Tool,
		Running: st.Running,
		Detail:  st.Detail,
		SSHPort: SSHPort,
		UIPort:  UIPort,
	}
	switch st.Tool {
	case "firewalld":
		v.CanStart = true
		v.CanReload = st.Running
		if st.Running {
			v.Hint = "firewalld 已运行"
		} else {
			v.Hint = "未运行，启动前会放行 SSH 22 和控制台 9527"
		}
	case "ufw":
		v.CanStart = true
		v.CanReload = false
		if st.Running {
			v.Hint = "ufw 已启用"
		} else {
			v.Hint = "未启用，启动前会放行 SSH 22 和控制台 9527"
		}
	case "iptables":
		v.Hint = "当前为 iptables，请手工维护规则"
	default:
		if goos == "windows" {
			v.Hint = "Windows 本机跳过防火墙"
		} else {
			v.Hint = "未检测到 firewalld / ufw / iptables"
		}
	}
	return v
}
