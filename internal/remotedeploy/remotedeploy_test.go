package remotedeploy

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

// TestRunModuleSSHFailureSurfaces 目标机不可连接时错误应可读地回传，且日志先写出目标机器信息。
func TestRunModuleSSHFailureSurfaces(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no loopback listener:", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close() // 立即断开，模拟 SSH 握手失败
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	dir := t.TempDir()
	site := filepath.Join(dir, "site.yaml")
	_ = os.WriteFile(site, []byte("site: {name: t, code: t}\n"), 0o644)
	mod := filepath.Join(dir, "kafka")
	_ = os.MkdirAll(mod, 0o755)

	var logs []string
	_, err = RunModule(
		Target{
			Node:        config.Node{Name: "app-node", IP: "127.0.0.1", SSH: config.SSHAuth{User: "root", Port: port}},
			SSHPassword: "placeholder",
			SitePath:    site,
		},
		ModuleOptions{ModuleDir: mod, Expand: true, Load: true, ComposeUp: true, SyncFiles: true},
		func(s string) { logs = append(logs, s) },
	)
	if err == nil {
		t.Fatal("expected ssh failure")
	}
	if !strings.Contains(err.Error(), "ssh") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) == 0 || !strings.Contains(logs[0], "app-node") {
		t.Fatalf("expected target log first, got %v", logs)
	}
}

func TestFindNodeAndIsRemote(t *testing.T) {
	site := &config.SiteConfig{Nodes: []config.Node{
		{Name: "app-node", IP: "127.0.0.1"},
		{Name: "db-node", IP: "192.0.2.50"},
	}}
	if _, ok := FindNode(site, ""); ok {
		t.Fatal("empty key must not match")
	}
	n, ok := FindNode(site, "db-node")
	if !ok || n.IP != "192.0.2.50" {
		t.Fatalf("find by name: %+v %v", n, ok)
	}
	n, ok = FindNode(site, "192.0.2.50")
	if !ok || n.Name != "db-node" {
		t.Fatalf("find by ip: %+v %v", n, ok)
	}
	if !IsRemote(n) {
		t.Fatal("192.0.2.50 should be remote")
	}
	local, _ := FindNode(site, "app-node")
	if IsRemote(local) {
		t.Fatal("127.0.0.1 should be local")
	}
}
