package ui

import (
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestProbeOneDocker_EmptyIP(t *testing.T) {
	st := probeOneDocker(config.Node{Name: "node-a"}, "", "")
	if st.OK {
		t.Fatal("empty IP should not be ok")
	}
	if st.Message != "未填 IP" {
		t.Fatalf("msg=%q", st.Message)
	}
	if st.Name != "node-a" {
		t.Fatalf("name=%q", st.Name)
	}
}

func TestFirstDockerVersion(t *testing.T) {
	if got := firstDockerVersion("24.0.7\n"); got != "24.0.7" {
		t.Fatalf("got %q", got)
	}
	if got := firstDockerVersion("Cannot connect to the Docker daemon\n"); got != "" {
		t.Fatalf("noise=%q", got)
	}
	if got := firstDockerVersion("bash: docker: command not found\n"); got != "" {
		t.Fatalf("missing=%q", got)
	}
}
