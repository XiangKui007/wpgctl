package moduledeploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModulePathDirect(t *testing.T) {
	root := t.TempDir()
	public := filepath.Join(root, "platform", "public")
	if err := os.MkdirAll(public, 0o755); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, "platform")
	got := ModulePath(base, "public")
	if got != public {
		t.Fatalf("expected %q, got %q", public, got)
	}
}

func TestModulePathNested(t *testing.T) {
	root := t.TempDir()
	outer := filepath.Join(root, "platform")
	inner := filepath.Join(outer, "platform")
	public := filepath.Join(inner, "public")
	if err := os.MkdirAll(public, 0o755); err != nil {
		t.Fatal(err)
	}
	got := ModulePath(outer, "public")
	if got != public {
		t.Fatalf("expected nested %q, got %q", public, got)
	}
}

func TestModulePathNoTripleNest(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "platform", "platform")
	public := filepath.Join(base, "public")
	if err := os.MkdirAll(public, 0o755); err != nil {
		t.Fatal(err)
	}
	got := ModulePath(base, "public")
	if got != public {
		t.Fatalf("expected %q, got %q (must not triple-nest platform)", public, got)
	}
}
