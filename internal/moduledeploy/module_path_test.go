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

func TestModulePathNestedMiddlewareNginx(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "sz-waterwork-4.1.1-pg")
	nginx := filepath.Join(pkg, "middleware", "middleware", "nginx")
	confd := filepath.Join(nginx, "conf", "conf.d")
	if err := os.MkdirAll(confd, 0o755); err != nil {
		t.Fatal(err)
	}
	got := ModulePath(pkg, "nginx")
	if got != nginx {
		t.Fatalf("from package root: got %q want %q", got, nginx)
	}
	outer := filepath.Join(pkg, "middleware")
	got = ModulePath(outer, "nginx")
	if got != nginx {
		t.Fatalf("from outer middleware: got %q want %q", got, nginx)
	}
}

func TestResolveNginxLayoutByComposeWithoutConf(t *testing.T) {
	root := t.TempDir()
	nginx := filepath.Join(root, "middleware", "middleware", "nginx")
	if err := os.MkdirAll(nginx, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nginx, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrong := filepath.Join(root, "middleware", "nginx")
	lay := ResolveNginxLayout(wrong)
	if filepath.Clean(lay.ModuleDir) != filepath.Clean(nginx) {
		t.Fatalf("ModuleDir %q want %q", lay.ModuleDir, nginx)
	}
}
