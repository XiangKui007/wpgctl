package initenv

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestProbeAndInitWorkspace(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "waterwork")
	html := filepath.Join(root, "nginx", "html")

	probe, err := ProbeWorkspace(ws, html)
	if err != nil {
		t.Fatal(err)
	}
	if probe.Ready {
		t.Fatal("expected not ready before create")
	}

	probe2, created, err := InitWorkspaceDirs(ws, html)
	if err != nil {
		t.Fatal(err)
	}
	if !probe2.Ready {
		t.Fatal("expected ready after init")
	}
	if len(created) == 0 {
		t.Fatal("expected created dirs")
	}
	// 再跑一次应幂等
	_, created2, err := InitWorkspaceDirs(ws, html)
	if err != nil {
		t.Fatal(err)
	}
	if len(created2) != 0 {
		t.Fatalf("second init should create nothing, got %v", created2)
	}
	if runtime.GOOS == "linux" {
		found := false
		for _, d := range probe2.Dirs {
			if d.Label == "docker_data" && d.Exists {
				found = true
			}
		}
		if !found {
			t.Fatal("linux should create docker_data")
		}
	}
}
