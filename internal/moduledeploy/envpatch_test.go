package moduledeploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestEnvPatchHosts(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	content := "NACOS_HOST=10.0.0.1\nNACOS_PASSWORD=secret\nREDIS_HOST=10.0.0.2\n"
	if err := os.WriteFile(env, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	site := &config.SiteConfig{
		Site: config.SiteInfo{Code: "demo"},
		Middleware: config.MiddlewareConfig{
			Nacos: config.NacosConn{Host: "192.168.1.22", Port: 8848, Namespace: "intergrate"},
			Redis: config.RedisConn{Host: "192.168.1.23", Port: 6377},
		},
	}
	changed, err := EnvPatchHosts(env, site)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) < 2 {
		t.Fatalf("expected host keys changed, got %v", changed)
	}
	out, _ := os.ReadFile(env)
	if string(out) == content {
		t.Fatal("env not updated")
	}
	if !strings.Contains(string(out), "secret") {
		t.Fatal("password should remain")
	}
}
