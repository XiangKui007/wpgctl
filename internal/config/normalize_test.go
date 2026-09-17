package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestLoadSite_migrateSmartwaterProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.yaml")
	content := `
site: { name: demo, code: plant-demo }
nodes:
  - name: n1
    ip: 10.0.0.1
    ssh: { user: root, port: 22 }
    roles: [middleware, platform, smartwater]
profiles: [platform, smartwater, monitor]
middleware:
  nacos: { host: 10.0.0.1, port: 8848, namespace: ns, username: nacos, password: p }
  mysql: { host: 10.0.0.1, port: 3306, user: u, password: p }
  redis: { host: 10.0.0.1, port: 6379, password: p }
  kafka: { host: 10.0.0.1, port: 9092 }
paths: { workspace: /w, logs: /l, nginxHtml: /n }
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadSite(path)
	if err != nil {
		t.Fatalf("LoadSite: %v", err)
	}
	for _, p := range cfg.Profiles {
		if p == "smartwater" {
			t.Fatalf("smartwater should be migrated, got profiles=%v", cfg.Profiles)
		}
	}
	if !contains(cfg.Profiles, "waterwork") {
		t.Fatalf("expected waterwork profile, got %v", cfg.Profiles)
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
