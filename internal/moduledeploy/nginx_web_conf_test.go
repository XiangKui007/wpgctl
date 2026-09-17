package moduledeploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchHTTPWeb8877(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "http-web-8877.conf")
	content := `
location /main/ {
    proxy_pass http://10.10.102.105:18094/;
}
location /aisinobill/ {
    proxy_pass http://47.114.89.147:65393/aisinobill/;
}
`
	if err := os.WriteFile(conf, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	notes, err := PatchHTTPWeb8877(conf, NginxProxyIPs{Gateway: "192.168.1.10"})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 change, got %v", notes)
	}
	out, _ := os.ReadFile(conf)
	s := string(out)
	if !strings.Contains(s, "192.168.1.10:18094") {
		t.Fatal("gateway not patched")
	}
	if !strings.Contains(s, "47.114.89.147") {
		t.Fatal("external proxy should not change")
	}
}
