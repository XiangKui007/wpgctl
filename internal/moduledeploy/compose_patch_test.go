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
