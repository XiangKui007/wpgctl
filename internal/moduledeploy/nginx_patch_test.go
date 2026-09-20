package moduledeploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNginxPatch_noArchivesSkipsExtract(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "html"), 0o755); err != nil {
		t.Fatal(err)
	}
	confd := filepath.Join(dir, "conf", "conf.d")
	if err := os.MkdirAll(confd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confd, "http-web-8877.conf"), []byte("server {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := RunNginxPatch(NginxPatchOptions{
		Layout:         ResolveNginxLayout(dir),
		ExpandArchives: true,
		SkipProxyPatch: true,
		ComposeUp:      false,
	})
	if err != nil {
		t.Fatalf("already-extracted nginx dir should not fail: %v", err)
	}
	joined := strings.Join(res.Steps, "\n")
	if !strings.Contains(joined, "跳过解压") && !strings.Contains(joined, "跳过 docker load") {
		t.Fatalf("expected skip notes, got %v", res.Steps)
	}
}
