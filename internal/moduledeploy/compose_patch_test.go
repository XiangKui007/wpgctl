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
	content := `services:
  kafka:
    environment:
      - KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://10.10.15.107:9092
      - KAFKA_BROKER_ID=1
`
	if err := os.WriteFile(compose, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	site := &config.SiteConfig{
		Nodes: []config.Node{{IP: "192.168.1.10"}},
		Middleware: config.MiddlewareConfig{
			Kafka: config.KafkaConn{Host: "192.168.1.10", Port: 9092},
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
	if !strings.Contains(string(out), "PLAINTEXT://192.168.1.10:9092") {
		t.Fatalf("compose not patched: %s", out)
	}
}
