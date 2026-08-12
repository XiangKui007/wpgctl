package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestLoadSiteExample(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "examples", "site.example.yaml")
	cfg, err := config.LoadSite(path)
	if err != nil {
		t.Fatalf("LoadSite: %v", err)
	}
	if cfg.Site.Code != "plant-demo" {
		t.Fatalf("unexpected code: %s", cfg.Site.Code)
	}
	if len(cfg.Profiles) == 0 {
		t.Fatal("profiles empty")
	}
}

func TestLoadManifestExample(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "examples", "manifest.release.example.yaml")
	m, err := config.LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	svcs := m.EnabledServices([]string{"platform", "smartwater"})
	if len(svcs) < 3 {
		t.Fatalf("expected middleware + platform services, got %d", len(svcs))
	}
	ports := m.Ports([]string{"platform"})
	if len(ports) == 0 {
		t.Fatal("ports empty")
	}
}

func TestSiteRejectUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.yaml")
	content := `
site: { name: x, code: plant-x }
nodes:
  - name: n1
    ip: 10.0.0.1
    ssh: { user: root, port: 22 }
    roles: [platform]
profiles: [platform]
middleware:
  nacos: { host: 10.0.0.1, port: 8848, namespace: ns, username: nacos, password: p }
  mysql: { host: 10.0.0.1, port: 3306, user: u, password: p }
  redis: { host: 10.0.0.1, port: 6379, password: p }
  kafka: { host: 10.0.0.1, port: 9092 }
paths: { workspace: /w, logs: /l, nginxHtml: /n }
unknownField: true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.LoadSite(path); err == nil {
		t.Fatal("expected unknown field error")
	}
}
