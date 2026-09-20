package moduledeploy

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCompose(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(p, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFindComposeFileNestedChild(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "waterwork-4.1.1-pg")
	inner := filepath.Join(pkg, "waterwork-4.1.1-pg")
	want := writeCompose(t, inner)
	got, err := findComposeFile(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindComposeFileParentSameName(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "waterwork-4.1.1-pg")
	inner := filepath.Join(pkg, "waterwork-4.1.1-pg")
	want := writeCompose(t, pkg)
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := findComposeFile(inner)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindComposeFilePrefersDockerfileSibling(t *testing.T) {
	root := t.TempDir()
	other := writeCompose(t, filepath.Join(root, "docs"))
	_ = other
	app := filepath.Join(root, "app")
	want := writeCompose(t, app)
	if err := os.WriteFile(filepath.Join(app, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findComposeFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindComposeFileNestedMiddlewareNginx(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "sz-waterwork-4.1.1-pg")
	real := filepath.Join(pkg, "middleware", "middleware", "nginx")
	want := writeCompose(t, real)
	wrong := filepath.Join(pkg, "middleware", "nginx")
	got, err := findComposeFile(wrong)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindComposeFileMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := findComposeFile(dir); err == nil {
		t.Fatal("expected error")
	}
}

func TestFindComposeProjects_WaterworkCenterAndDevice(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "sz-waterwork-4.1.1-pg")
	center := writeCompose(t, filepath.Join(pkg, "waterwork-center"))
	device := writeCompose(t, filepath.Join(pkg, "waterwork-device"))
	got, err := findComposeProjects(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 projects, got %v", got)
	}
	if filepath.Clean(got[0]) != filepath.Clean(center) {
		t.Fatalf("center first: %v", got)
	}
	if filepath.Clean(got[1]) != filepath.Clean(device) {
		t.Fatalf("device second: %v", got)
	}
}

func TestFindComposeProjects_NestedZipLayer(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "waterwork-4.1.1-pg")
	inner := filepath.Join(pkg, "waterwork-4.1.1-pg")
	center := writeCompose(t, filepath.Join(inner, "waterwork-center"))
	device := writeCompose(t, filepath.Join(inner, "waterwork-device"))
	got, err := findComposeProjects(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if filepath.Clean(got[0]) != filepath.Clean(center) || filepath.Clean(got[1]) != filepath.Clean(device) {
		t.Fatalf("got %v", got)
	}
}
