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

func TestEnvPatchHosts_PgSQLIntoMySQLKeys(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	content := "MYSQL_HOST=10.0.0.1\nMYSQL_PORT=3306\nMYSQL_USER=root\nMYSQL_PASSWORD=old\nNACOS_PASSWORD=secret\n"
	if err := os.WriteFile(env, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	site := &config.SiteConfig{
		Middleware: config.MiddlewareConfig{
			MySQL: config.DBConn{Disabled: true},
			PgSQL: config.DBConn{Host: "10.10.102.70", Port: 5433, User: "wpg", Password: "pg-secret"},
		},
	}
	changed, err := EnvPatchHosts(env, site)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) < 4 {
		t.Fatalf("expected mysql keys rewritten from pgsql, got %v", changed)
	}
	out := string(mustRead(t, env))
	if !strings.Contains(out, "MYSQL_HOST=10.10.102.70") || !strings.Contains(out, "MYSQL_PORT=5433") {
		t.Fatalf("host/port: %s", out)
	}
	if !strings.Contains(out, "MYSQL_USER=wpg") || !strings.Contains(out, "MYSQL_PASSWORD=pg-secret") {
		t.Fatalf("user/pass: %s", out)
	}
	if !strings.Contains(out, "NACOS_PASSWORD=secret") {
		t.Fatal("unrelated password changed")
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
