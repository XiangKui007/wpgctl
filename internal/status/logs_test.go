package status

import (
	"strings"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestNewLogPullerRejectsBadName(t *testing.T) {
	_, err := NewLogPuller(LogsOptions{Service: "foo; rm -rf /"})
	if err == nil || !strings.Contains(err.Error(), "非法容器名") {
		t.Fatalf("got %v", err)
	}
}

func TestNewLogPullerRemoteMissingNode(t *testing.T) {
	_, err := NewLogPuller(LogsOptions{
		Site:    &config.SiteConfig{Nodes: []config.Node{{Name: "a", IP: "192.0.2.8"}}},
		Service: "nacos",
		NodeIP:  "192.0.2.9",
	})
	if err == nil || !strings.Contains(err.Error(), "没有节点") {
		t.Fatalf("got %v", err)
	}
}

func TestNewLogPullerLocalSkipsSSH(t *testing.T) {
	p, err := NewLogPuller(LogsOptions{Service: "nacos-nacos-1", NodeIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.local == nil || p.remote != nil {
		t.Fatal("loopback must use local Docker")
	}
}

func TestDockerLogsCmd(t *testing.T) {
	got := dockerLogsCmd("waterwork-center", 80, false)
	if got != "docker logs --tail=80 waterwork-center" {
		t.Fatalf("got %q", got)
	}
	got = dockerLogsCmd("waterwork-center", 200, true)
	if got != "docker logs --tail=200 -f waterwork-center" {
		t.Fatalf("follow %q", got)
	}
}
