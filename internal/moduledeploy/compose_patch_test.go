package moduledeploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestPatchComposeEnv_kafka(t *testing.T) {
	dir := t.TempDir()
	compose := filepath.Join(dir, "docker-compose.yml")
	content := `version: '3'
services:
  zookeeper:
    image: zookeeper:3.4.13
  kafka:
    environment:
      - KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181
      # 修改为本机的物理IP地址
      - KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://10.10.15.107:9092
      - KAFKA_LISTENERS=PLAINTEXT://0.0.0.0:9092
`
	if err := os.WriteFile(compose, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	site := &config.SiteConfig{
		Nodes: []config.Node{
			{Name: "db", IP: "10.10.102.70", Services: []string{"mysql"}},
			{Name: "app", IP: "10.10.102.71", Services: []string{"kafka", "nacos"}},
		},
		Middleware: config.MiddlewareConfig{
			Kafka: config.KafkaConn{Host: "10.10.102.71", Port: 9092},
		},
	}
	changed, err := PatchComposeEnvForSite(dir, site)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0] != "KAFKA_ADVERTISED_LISTENERS" {
		t.Fatalf("unexpected changed: %v", changed)
	}
	out, _ := os.ReadFile(compose)
	if !strings.Contains(string(out), "PLAINTEXT://10.10.102.71:9092") {
		t.Fatalf("compose not patched: %s", out)
	}
	if strings.Contains(string(out), "10.10.15.107") {
		t.Fatalf("old IP still present: %s", out)
	}
}

// 平台 / GIS 等模块的 compose 不能被改：即便 environment 里写着 WPG_PGSQL_HOST 之类，也只动 .env。
func TestPatchComposeEnvForSite_LeavesPlatformComposeAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "giscenter")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	compose := filepath.Join(dir, "docker-compose.yml")
	content := `services:
  giscenter:
    environment:
      - WPG_PGSQL_HOST=10.10.15.107
      - WPG_PGSQL_DBNAME=gis_center
      - MYSQL_HOST=10.10.15.107
`
	if err := os.WriteFile(compose, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	site := &config.SiteConfig{
		Nodes:      []config.Node{{Name: "app", IP: "10.10.102.71", Services: []string{"kafka", "postgis"}}},
		Middleware: config.MiddlewareConfig{Kafka: config.KafkaConn{Host: "10.10.102.71", Port: 9092}},
	}
	changed, err := PatchComposeEnvForSite(dir, site)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 0 {
		t.Fatalf("platform compose must not change, got %v", changed)
	}
	out, _ := os.ReadFile(compose)
	if string(out) != content {
		t.Fatalf("compose content altered:\n%s", out)
	}
}

func TestKafkaAdvertiseHostFromServices(t *testing.T) {
	site := &config.SiteConfig{
		Nodes: []config.Node{
			{IP: "1.1.1.1", Services: []string{"mysql"}},
			{IP: "2.2.2.2", Services: []string{"kafka"}},
		},
	}
	if got := kafkaAdvertiseHost(site); got != "2.2.2.2" {
		t.Fatalf("got %q", got)
	}
}
