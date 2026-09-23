package moduledeploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestParsePublishedPort(t *testing.T) {
	cases := map[string]int{
		"3306:3306":           3306,
		"127.0.0.1:5432:5432": 5432,
		"8080":                8080,
		"8080/tcp":            8080,
		"5353/udp":            0,
		"[::]:8877:80":        8877,
		"8000-8010:8000-8010": 0,
	}
	for in, want := range cases {
		if got := parsePublishedPort(in); got != want {
			t.Errorf("%q: got %d want %d", in, got, want)
		}
	}
}

func TestIsListenEnvKey(t *testing.T) {
	if !isListenEnvKey("SERVER_PORT") || !isListenEnvKey("NACOS_APPLICATION_PORT") {
		t.Fatal("listen keys")
	}
	if isListenEnvKey("MYSQL_PORT") || isListenEnvKey("REDIS_PASSWORD") {
		t.Fatal("client/password keys must be ignored")
	}
}

func TestCollectListenPortsBridgeAndEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docker-compose.yaml"), `
services:
  db:
    image: mysql
    ports:
      - "3306:3306"
      - 5432:5432
      - target: 80
        published: 8877
        protocol: tcp
      - "5353:53/udp"
`)
	got := CollectListenPorts(dir, nil, nil)
	assertPorts(t, got, 3306, 5432, 8877)
}

func TestCollectListenPortsHostNetworkEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "SERVER_PORT=18090\nMYSQL_PORT=3306\n")
	writeFile(t, filepath.Join(dir, "docker-compose.yaml"), `
services:
  app:
    network_mode: host
    env_file: .env
    environment:
      - SERVER_PORT=${SERVER_PORT:-18091}
`)
	got := CollectListenPorts(dir, nil, nil)
	assertPorts(t, got, 18090)
}

func TestCollectListenPortsEnvDefaultAndSiteFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docker-compose.yaml"), `
services:
  kafka:
    network_mode: host
    ports:
      - "${KAFKA_PORT:-9092}:9092"
`)
	got := CollectListenPorts(dir, nil, nil)
	assertPorts(t, got, 9092)

	mysqlDir := filepath.Join(t.TempDir(), "mysql")
	if err := os.MkdirAll(mysqlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(mysqlDir, "docker-compose.yml"), `
services:
  mysql:
    network_mode: host
    environment:
      MYSQL_ROOT_PASSWORD: secret
`)
	site := &config.SiteConfig{}
	site.Middleware.MySQL.Port = 3307
	got = CollectListenPorts(mysqlDir, nil, site)
	assertPorts(t, got, 3307)
}

func TestCollectListenPortsNacosGrpc(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nacos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "docker-compose.yaml"), `
services:
  nacos:
    network_mode: host
    environment:
      NACOS_APPLICATION_PORT: 8849
`)
	site := &config.SiteConfig{}
	site.Middleware.Nacos.Port = 8849
	got := CollectListenPorts(dir, nil, site)
	assertPorts(t, got, 8849, 9849)
}

// nginx 用 host 网络、compose 无 ports：端口来自 conf 的 listen，且 8877 必须在。
func TestCollectListenPortsNginxConf(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nginx")
	confd := filepath.Join(dir, "conf", "conf.d")
	if err := os.MkdirAll(confd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "html", "alarm"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "docker-compose.yaml"), `
services:
  nginx:
    image: nginx
    network_mode: host
`)
	writeFile(t, filepath.Join(confd, "http-web-8877.conf"), "server {\n    listen 8877;\n    location / {}\n}\n")
	writeFile(t, filepath.Join(confd, "http-web-8878.conf"), "server {\n  listen 0.0.0.0:8878 default_server;\n  listen [::]:8879 ssl;\n}\n")
	// html 里的 conf 不算监听配置
	writeFile(t, filepath.Join(dir, "html", "alarm", "fake.conf"), "listen 9999;\n")
	got := CollectListenPorts(dir, nil, nil)
	assertPorts(t, got, 8877, 8878, 8879)

	// conf 里没写 8877 也要放行 8877
	empty := filepath.Join(t.TempDir(), "nginx")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	assertPorts(t, CollectListenPorts(empty, nil, nil), 8877)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertPorts(t *testing.T, got []int, want ...int) {
	t.Helper()
	set := map[int]struct{}{}
	for _, p := range got {
		set[p] = struct{}{}
	}
	for _, p := range want {
		if _, ok := set[p]; !ok {
			t.Fatalf("missing port %d in %v", p, got)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("extra ports: got %v want %v", got, want)
	}
}
