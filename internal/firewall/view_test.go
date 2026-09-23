package firewall

import "testing"

func TestViewFromStatusFirewalld(t *testing.T) {
	idle := viewFromStatus(Status{Tool: "firewalld", Running: false, Detail: "not running"}, "linux")
	if !idle.CanStart || idle.CanReload || idle.SSHPort != 22 || idle.UIPort != 9527 {
		t.Fatalf("idle: %+v", idle)
	}
	run := viewFromStatus(Status{Tool: "firewalld", Running: true, Detail: "running"}, "linux")
	if !run.CanStart || !run.CanReload {
		t.Fatalf("running: %+v", run)
	}
}

func TestViewFromStatusUfwAndNone(t *testing.T) {
	ufw := viewFromStatus(Status{Tool: "ufw", Running: true, Detail: "active"}, "linux")
	if !ufw.CanStart || ufw.CanReload {
		t.Fatalf("ufw should not reload: %+v", ufw)
	}
	win := viewFromStatus(Status{Tool: "none", Running: false, Detail: "not found"}, "windows")
	if win.CanStart || win.CanReload {
		t.Fatalf("windows none: %+v", win)
	}
	if win.Hint == "" || win.Hint == "未检测到 firewalld / ufw / iptables，请确认系统已安装防火墙" {
		t.Fatalf("windows hint: %q", win.Hint)
	}
}
