package moduledeploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestPatchPrometheusSD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prometheus.yml")
	raw := "remote_write:\n  - url: \"http://1.2.3.4:29090/api/v1/write\"\nscrape_configs:\n  - job_name: 'http-sd-proName'\n    http_sd_configs:\n     - url: 'http://10.10.102.228:18099/prometheus/hp'\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PatchPrometheusSD(path, "10.1.1.8"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "http://10.1.1.8:18099/prometheus/hp") {
		t.Fatalf("http_sd 未改: %s", got)
	}
	if !strings.Contains(string(got), "http://1.2.3.4:29090/api/v1/write") {
		t.Fatalf("remote_write 被改动: %s", got)
	}
}

func TestPatchExporterCompose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	raw := "services:\n  kafka-exporter:\n    image: 10.10.102.75/exporter/wpg-kafka-exporter:1.8.0\n    command:\n      - '--kafka.server=10.100.20.11:9092'\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PatchExporterCompose(path, "kafka", "10.8.8.8", 9092, ""); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "--kafka.server=10.8.8.8:9092") {
		t.Fatalf("kafka 地址未改: %s", got)
	}
	if !strings.Contains(string(got), "10.10.102.75/exporter") {
		t.Fatalf("镜像地址被改: %s", got)
	}
}

func TestPatchMonitorEnvExtras(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	raw := "NACOS_HOST=1.1.1.1\nPROMETHEUS_HOST=9.9.9.9\nPROMETHEUS_PORT=29090\nMETRIC_URL=1.1.1.1:8877\nREDIS_PASSWORD=old\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	site := &config.SiteConfig{
		Site: config.SiteInfo{Name: "t", Code: "testsite"},
		Nodes: []config.Node{{
			Name: "a", IP: "10.0.0.8",
			SSH:      config.SSHAuth{User: "root", Port: 22},
			Roles:    []string{"app"},
			Services: []string{"nginx"},
		}},
		Middleware: config.MiddlewareConfig{
			Nacos: config.NacosConn{Host: "10.0.0.2", Port: 8848, Namespace: "testsite", Username: "nacos", Password: "np"},
			PgSQL: config.DBConn{Host: "10.0.0.3", Port: 5433, User: "wpg", Password: "pp"},
			Redis: config.RedisConn{Host: "10.0.0.4", Port: 6377, Password: "rp"},
			Kafka: config.KafkaConn{Host: "10.0.0.5", Port: 9092},
		},
	}
	if _, err := PatchMonitorEnv(path, site, "10.0.0.8"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	text := string(got)
	for _, want := range []string{
		"PROMETHEUS_HOST=10.0.0.8",
		"PROMETHEUS_PORT=29090",
		"METRIC_URL=10.0.0.8:8877",
		"REDIS_PASSWORD=rp",
		"NACOS_HOST=10.0.0.2",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("缺少 %s\n%s", want, text)
		}
	}
}
