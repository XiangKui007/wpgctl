package moduledeploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveNginxLayoutFindsConfD(t *testing.T) {
	root := t.TempDir()
	nginx := filepath.Join(root, "nginx")
	confd := filepath.Join(nginx, "conf.d")
	if err := os.MkdirAll(confd, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(confd, "http-web-8877.conf")
	if err := os.WriteFile(want, []byte("server{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lay := ResolveNginxLayout(nginx)
	if filepath.Clean(lay.WebConf) != filepath.Clean(want) {
		t.Fatalf("WebConf %q want %q", lay.WebConf, want)
	}
}

func TestResolveNginxLayoutStandardConfConfD(t *testing.T) {
	root := t.TempDir()
	nginx := filepath.Join(root, "nginx")
	dir := filepath.Join(nginx, "conf", "conf.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "http-web-8877.conf")
	if err := os.WriteFile(want, []byte("server{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lay := ResolveNginxLayout(nginx)
	if filepath.Clean(lay.WebConf) != filepath.Clean(want) {
		t.Fatalf("WebConf %q want %q", lay.WebConf, want)
	}
}

func TestResolveNginxLayoutMissingDirectNginxUsesNested(t *testing.T) {
	root := t.TempDir()
	nginx := filepath.Join(root, "middleware", "middleware", "nginx")
	dir := filepath.Join(nginx, "conf", "conf.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "http-web-8877.conf")
	if err := os.WriteFile(want, []byte("server{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrong := filepath.Join(root, "middleware", "nginx")
	lay := ResolveNginxLayout(wrong)
	if filepath.Clean(lay.ModuleDir) != filepath.Clean(nginx) {
		t.Fatalf("ModuleDir %q want %q", lay.ModuleDir, nginx)
	}
	if filepath.Clean(lay.WebConf) != filepath.Clean(want) {
		t.Fatalf("WebConf %q want %q", lay.WebConf, want)
	}
}

func TestExpandNginxHTML_emptyDirOptional(t *testing.T) {
	dir := t.TempDir()
	html := filepath.Join(dir, "html")
	if err := os.MkdirAll(html, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := ExpandNginxHTML(html)
	if err != nil {
		t.Fatalf("empty html should not error: %v", err)
	}
	if res == nil || res.ZipExtracted != 0 {
		t.Fatalf("expected no zip extracted, got %+v", res)
	}
}

func TestExpandNginxHTML_missingDirOptional(t *testing.T) {
	dir := t.TempDir()
	html := filepath.Join(dir, "html")
	res, err := ExpandNginxHTML(html)
	if err != nil {
		t.Fatalf("missing html dir should not error: %v", err)
	}
	if res == nil || res.RootDir != html {
		t.Fatalf("unexpected result: %+v", res)
	}
}
