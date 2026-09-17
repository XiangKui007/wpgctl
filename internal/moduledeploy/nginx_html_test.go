package moduledeploy

import (
	"os"
	"path/filepath"
	"testing"
)

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
