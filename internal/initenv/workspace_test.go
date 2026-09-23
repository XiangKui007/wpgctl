package initenv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveWorkbookPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		got := ResolveWorkbookPath(`D:\`)
		want := filepath.Join(`D:\`, "workspace")
		if got != want {
			t.Fatalf("盘符根 got %q want %q", got, want)
		}
		already := filepath.Clean(`D:\workspace`)
		if got := ResolveWorkbookPath(`D:\workspace`); got != already {
			t.Fatalf("已是 workspace got %q want %q", got, already)
		}
		if got := ResolveWorkbookPath(""); got != filepath.Clean("D:/workspace") {
			t.Fatalf("空输入 got %q", got)
		}
		parent := ResolveWorkbookPath(`D:\data`)
		wantParent := filepath.Join(`D:\data`, "workspace")
		if parent != wantParent {
			t.Fatalf("父目录 got %q want %q", parent, wantParent)
		}
		nested := filepath.Join(`D:\workspace`, "pkg-from-upload")
		if got := ResolveWorkbookPath(nested); got != filepath.Clean(`D:\workspace`) {
			t.Fatalf("包目录应上溯到工作簿根 got %q", got)
		}
		if got := ResolveWorkbookPath("pkg-from-upload"); got != filepath.Clean("D:/workspace") {
			t.Fatalf("相对包名 got %q", got)
		}
		return
	}
	if got := ResolveWorkbookPath("/"); got != "/workspace" {
		t.Fatalf("/ got %q", got)
	}
	if got := ResolveWorkbookPath("/workspace"); got != "/workspace" {
		t.Fatalf("/workspace got %q", got)
	}
	if got := ResolveWorkbookPath("/workspace/"); got != "/workspace" {
		t.Fatalf("带斜杠 got %q", got)
	}
	if got := ResolveWorkbookPath("/mnt/data"); got != "/mnt/data/workspace" {
		t.Fatalf("父目录 got %q", got)
	}
	if got := ResolveWorkbookPath(""); got != "/workspace" {
		t.Fatalf("空输入 got %q", got)
	}
	if got := ResolveWorkbookPath("/workspace/pkg-from-upload"); got != "/workspace" {
		t.Fatalf("包目录应上溯到工作簿根 got %q", got)
	}
	if got := ResolveWorkbookPath("pkg-from-upload"); got != "/workspace" {
		t.Fatalf("相对包名 got %q", got)
	}
}

func TestProbeAndInitWorkspace(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "workspace")

	probe, err := ProbeWorkspace(ws, "")
	if err != nil {
		t.Fatal(err)
	}
	if probe.Ready {
		t.Fatal("expected not ready before create")
	}

	probe2, created, err := InitWorkspaceDirs(ws, "")
	if err != nil {
		t.Fatal(err)
	}
	if !probe2.Ready {
		t.Fatal("expected ready after init")
	}
	if len(created) != 1 || created[0] != ws {
		t.Fatalf("expected only workbook root, got %v", created)
	}
	if _, err := os.Stat(filepath.Join(ws, "rendered")); err == nil {
		t.Fatal("preflight must not create rendered")
	}
	if _, err := os.Stat(filepath.Join(ws, "bak")); err == nil {
		t.Fatal("preflight must not create bak")
	}
	// 再跑一次应幂等
	_, created2, err := InitWorkspaceDirs(ws, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(created2) != 0 {
		t.Fatalf("second init should create nothing, got %v", created2)
	}
}
