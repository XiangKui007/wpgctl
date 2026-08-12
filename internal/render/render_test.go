package render_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/render"
)

func TestRenderEnvTemplate(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	_ = os.MkdirAll(filepath.Join(pkg, "config"), 0o755)

	tmpl, err := os.ReadFile(filepath.Join("..", "..", "configs", "examples", "env.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "config", "env.tmpl"), tmpl, 0o644); err != nil {
		t.Fatal(err)
	}
	mfContent := `
kind: release
version: 4.0.2
arch: [amd64]
services:
  - name: mysql
    image: mysql:5.7
    layer: 1
    port: 3306
    health: { type: tcp }
`
	if err := os.WriteFile(filepath.Join(pkg, "manifest.yaml"), []byte(mfContent), 0o644); err != nil {
		t.Fatal(err)
	}

	site := &config.SiteConfig{
		Site:     config.SiteInfo{Name: "demo", Code: "plant-demo"},
		Nodes:    []config.Node{{Name: "n1", IP: "10.10.104.22", SSH: config.SSHAuth{User: "root", Port: 22}, Roles: []string{"middleware"}}},
		Profiles: []string{"platform"},
		Middleware: config.MiddlewareConfig{
			Nacos: config.NacosConn{Host: "10.10.104.22", Port: 8848, Namespace: "plant-demo", Username: "nacos", Password: "p"},
			MySQL: config.DBConn{Host: "10.10.104.23", Port: 3306, User: "wpg", Password: "p"},
			Redis: config.RedisConn{Host: "10.10.104.23", Port: 6377, Password: "p"},
			Kafka: config.KafkaConn{Host: "10.10.104.22", Port: 9092},
		},
		Paths: config.PathsConfig{Workspace: filepath.Join(root, "ws"), Logs: filepath.Join(root, "logs"), NginxHTML: filepath.Join(root, "html")},
	}
	mf, err := config.LoadManifest(filepath.Join(pkg, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := render.Run(render.Options{Site: site, Manifest: mf, PackageDir: pkg, OutputDir: filepath.Join(root, "out")})
	if err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(res.OutputDir, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "NACOS_HOST=10.10.104.22") {
		t.Fatalf("env not rendered: %s", data)
	}
	if !contains(string(data), "KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://10.10.104.22:9092") {
		t.Fatalf("host ip not injected: %s", data)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || find(s, sub))
}

func find(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
