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

func TestEnvPatchHosts_ExtraMiddlewareFromNodes(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	content := strings.Join([]string{
		"NACOS_HOST=10.0.0.1",
		"MINIO_HOST=10.0.0.1",
		"MINIO_PORT=9000",
		"MINIO_ENDPOINT=http://10.0.0.1:9000",
		"INFLUXDB_HOST=10.0.0.1",
		"INFLUXDB_URL=http://10.0.0.1:8086",
		"MQTT_HOST=10.0.0.1",
		"MQTT_PORT=1883",
		"POSTGIS_HOST=10.0.0.1",
		"POSTGIS_PORT=5432",
		"SPRING_DATASOURCE_GIS_URL=jdbc:postgresql://10.0.0.1:5432/gis",
		"MINIO_ACCESS_KEY=keep-me",
		"NACOS_PASSWORD=secret",
	}, "\n") + "\n"
	if err := os.WriteFile(env, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	site := &config.SiteConfig{
		Middleware: config.MiddlewareConfig{
			Nacos: config.NacosConn{Host: "10.10.104.22", Port: 8848},
			PgSQL: config.DBConn{Host: "10.10.104.23", Port: 5433, User: "wpg", Password: "pg-secret"},
		},
		Nodes: []config.Node{
			{Name: "db", IP: "10.10.104.23", Roles: []string{"database"}, Services: []string{"pgsql", "postgis"}},
			{Name: "app", IP: "10.10.104.22", Roles: []string{"middleware"}, Services: []string{"nacos", "minio", "influxdb", "emqx"}},
		},
	}
	changed, err := EnvPatchHosts(env, site)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) < 8 {
		t.Fatalf("expected extra middleware keys, got %v", changed)
	}
	out := string(mustRead(t, env))
	for _, want := range []string{
		"NACOS_HOST=10.10.104.22",
		"MINIO_HOST=10.10.104.22",
		"MINIO_PORT=9000",
		"MINIO_ENDPOINT=http://10.10.104.22:9000",
		"INFLUXDB_HOST=10.10.104.22",
		"INFLUXDB_URL=http://10.10.104.22:8086",
		"MQTT_HOST=10.10.104.22",
		"POSTGIS_HOST=10.10.104.23",
		"POSTGIS_PORT=5432",
		"SPRING_DATASOURCE_GIS_URL=jdbc:postgresql://10.10.104.23:5432/gis",
		"MINIO_ACCESS_KEY=keep-me",
		"NACOS_PASSWORD=secret",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s\n%s", want, out)
		}
	}
}

func TestBinderForKey(t *testing.T) {
	if b := binderForKey("SPRING_DATASOURCE_GIS_URL"); b == nil || b.Service != "postgis" {
		t.Fatalf("gis datasource: %+v", b)
	}
	if b := binderForKey("MINIO_ENDPOINT"); b == nil || b.Service != "minio" {
		t.Fatalf("minio: %+v", b)
	}
	if b := binderForKey("NACOS_HOST"); b != nil {
		t.Fatal("nacos is site.middleware, not envBinders")
	}
	if b := binderForKey("XXLJOB_HOST"); b == nil || b.Service != "waterjob" {
		t.Fatalf("xxljob: %+v", b)
	}
	if b := binderForKey("XXL_JOB_ACCESS_TOKEN"); b == nil || b.Service != "waterjob" {
		t.Fatalf("xxl access token: %+v", b)
	}
}

func TestHostOfServicePrefersServicesList(t *testing.T) {
	site := &config.SiteConfig{
		Nodes: []config.Node{
			{IP: "10.0.0.1", Roles: []string{"middleware"}, Services: []string{"nacos"}},
			{IP: "10.0.0.8", Roles: []string{"middleware"}, Services: []string{"minio"}},
		},
	}
	if got := hostOfService(site, "minio", []string{"middleware"}); got != "10.0.0.8" {
		t.Fatalf("got %s", got)
	}
}

func TestHostOfServiceWaterJobAlias(t *testing.T) {
	site := &config.SiteConfig{
		Nodes: []config.Node{
			{IP: "10.0.0.1", Roles: []string{"middleware"}, Services: []string{"nacos"}},
			{IP: "192.168.200.66", Roles: []string{"middleware"}, Services: []string{"water-job-biz"}},
		},
	}
	if got := hostOfService(site, "waterjob", []string{"middleware"}); got != "192.168.200.66" {
		t.Fatalf("alias: got %s", got)
	}
}

func TestReplaceHostPortInEndpoint(t *testing.T) {
	got := replaceHostPortInEndpoint("http://10.0.0.1:9000/minio", "10.10.104.22", 9000)
	if got != "http://10.10.104.22:9000/minio" {
		t.Fatalf("got %s", got)
	}
	got = replaceHostPortInEndpoint("jdbc:postgresql://10.0.0.1:5432/gis", "10.10.104.23", 5432)
	if got != "jdbc:postgresql://10.10.104.23:5432/gis" {
		t.Fatalf("jdbc: %s", got)
	}
}

func TestEnvPatchHosts_GISPgSQLAndMongo(t *testing.T) {
	root := t.TempDir()
	centerEnv := filepath.Join(root, "gis", "giscenter", ".env")
	defaultEnv := filepath.Join(root, "gis", "gisdefault", ".env")
	otherEnv := filepath.Join(root, "public", ".env")
	raw := strings.Join([]string{
		"WPG_PGSQL_HOST=10.0.0.1",
		"WPG_PGSQL_PORT=5433",
		"WPG_PGSQL_USER=wpg",
		"WPG_PGSQL_PSWD=old",
		"WPG_PGSQL_DBNAME=gis_center",
		"WPG_MONGODB_DBNAME=gis",
		"WPG_MONGODB_HOST=10.0.0.1",
		"WPG_MONGODB_PSWD=old",
		"WPG_MONGODB_PORT=27017",
		"WPG_MONGODB_USER=gis",
		"NACOS_PASSWORD=secret",
	}, "\n") + "\n"
	for _, p := range []string{centerEnv, defaultEnv, otherEnv} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	site := &config.SiteConfig{
		Middleware: config.MiddlewareConfig{
			PgSQL: config.DBConn{Host: "10.10.104.23", Port: 5433, User: "wpg", Password: "pg-secret"},
		},
		Nodes: []config.Node{
			{Name: "gis-pg", IP: "192.168.200.97", Roles: []string{"database"}, Services: []string{"postgis"}},
			{Name: "gis-mongo", IP: "192.168.200.66", Roles: []string{"database"}, Services: []string{"mongodb"}},
		},
	}
	if _, err := EnvPatchHosts(centerEnv, site); err != nil {
		t.Fatal(err)
	}
	center := string(mustRead(t, centerEnv))
	for _, want := range []string{
		"WPG_PGSQL_HOST=192.168.200.97",
		"WPG_PGSQL_PORT=5432",
		"WPG_PGSQL_USER=postgres",
		"WPG_PGSQL_PSWD=GIS@city0418&wg",
		"WPG_PGSQL_DBNAME=gis_center",
		"WPG_MONGODB_HOST=192.168.200.66",
		"WPG_MONGODB_PORT=27017",
		"WPG_MONGODB_USER=gis",
		"WPG_MONGODB_PSWD=wpg87(GISklhjaal",
		"WPG_MONGODB_DBNAME=gis",
		"NACOS_PASSWORD=secret",
	} {
		if !strings.Contains(center, want) {
			t.Fatalf("giscenter missing %s\n%s", want, center)
		}
	}

	if _, err := EnvPatchHosts(defaultEnv, site); err != nil {
		t.Fatal(err)
	}
	def := string(mustRead(t, defaultEnv))
	if !strings.Contains(def, "WPG_PGSQL_DBNAME=gis_default") {
		t.Fatalf("gisdefault dbname: %s", def)
	}
	if !strings.Contains(def, "WPG_PGSQL_HOST=192.168.200.97") || !strings.Contains(def, "WPG_MONGODB_HOST=192.168.200.66") {
		t.Fatalf("gisdefault hosts: %s", def)
	}

	if _, err := EnvPatchHosts(otherEnv, site); err != nil {
		t.Fatal(err)
	}
	other := string(mustRead(t, otherEnv))
	if !strings.Contains(other, "WPG_PGSQL_HOST=10.0.0.1") || !strings.Contains(other, "WPG_PGSQL_PORT=5433") {
		t.Fatalf("non-GIS WPG_PGSQL should stay, got %s", other)
	}
}

func TestEnvPatchHosts_WaterjobXXLJob(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, "device", ".env")
	if err := os.MkdirAll(filepath.Dir(env), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"XXLJOB_TOKEN=old-token",
		"XXLJOB_HOST=10.0.0.1",
		"XXLJOB_PORT=8088",
		"NACOS_PASSWORD=secret",
	}, "\n") + "\n"
	if err := os.WriteFile(env, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	site := &config.SiteConfig{
		Middleware: config.MiddlewareConfig{
			Nacos: config.NacosConn{Host: "10.10.104.22", Port: 8848},
		},
		Nodes: []config.Node{
			{Name: "app", IP: "10.10.104.22", Roles: []string{"middleware"}, Services: []string{"nacos"}},
			{Name: "job", IP: "192.168.200.66", Roles: []string{"middleware"}, Services: []string{"waterjob"}},
		},
	}
	changed, err := EnvPatchHosts(env, site)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) < 3 {
		t.Fatalf("expected XXLJOB keys, got %v", changed)
	}
	out := string(mustRead(t, env))
	for _, want := range []string{
		"XXLJOB_TOKEN=wpg@2020",
		"XXLJOB_HOST=192.168.200.66",
		"XXLJOB_PORT=11005",
		"NACOS_PASSWORD=secret",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s\n%s", want, out)
		}
	}
}

func TestGisEnvFlavor(t *testing.T) {
	if gisEnvFlavor(`/workspace/platform/gis/giscenter/.env`) != "center" {
		t.Fatal("giscenter")
	}
	if gisEnvFlavor(`/workspace/platform/gis/gisdefault/.env`) != "default" {
		t.Fatal("gisdefault")
	}
	if gisEnvFlavor(`/workspace/platform/public/.env`) != "" {
		t.Fatal("public is not GIS")
	}
}

func TestEnvPatchHosts_WaterworkBothEnvsUsePgSQL(t *testing.T) {
	root := t.TempDir()
	centerEnv := filepath.Join(root, "waterwork-center", ".env")
	deviceEnv := filepath.Join(root, "waterwork-device", ".env")
	raw := "MYSQL_HOST=10.0.0.1\nMYSQL_PORT=3306\nMYSQL_USER=root\nMYSQL_PASSWORD=old\nNACOS_HOST=10.0.0.1\nNACOS_PASSWORD=secret\n"
	for _, p := range []string{centerEnv, deviceEnv} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	site := &config.SiteConfig{
		Middleware: config.MiddlewareConfig{
			Nacos: config.NacosConn{Host: "10.10.104.22", Port: 8848},
			MySQL: config.DBConn{Host: "10.10.104.80", Port: 3306, User: "mysql", Password: "mysql-pass"},
			PgSQL: config.DBConn{Host: "10.10.104.23", Port: 5433, User: "wpg", Password: "pg-secret"},
		},
		Nodes: []config.Node{
			{IP: "10.10.104.23", Roles: []string{"database"}, Services: []string{"pgsql"}},
		},
	}
	changed, err := PatchEnvTree(root, site)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) == 0 {
		t.Fatal("expected both waterwork .env patched")
	}
	for _, p := range []string{centerEnv, deviceEnv} {
		out := string(mustRead(t, p))
		for _, want := range []string{
			"MYSQL_HOST=10.10.104.23",
			"MYSQL_PORT=5433",
			"MYSQL_USER=wpg",
			"MYSQL_PASSWORD=pg-secret",
			"NACOS_HOST=10.10.104.22",
			"NACOS_PASSWORD=secret",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("%s missing %s\n%s", p, want, out)
			}
		}
	}
}

func TestEnvPatchWalkRoots_FromCenterUsesParent(t *testing.T) {
	root := t.TempDir()
	center := filepath.Join(root, "waterwork-center")
	writeComposeForEnv(t, center)
	writeComposeForEnv(t, filepath.Join(root, "waterwork-device"))
	got := envPatchWalkRoots(center)
	if len(got) != 1 || filepath.Clean(got[0]) != filepath.Clean(root) {
		t.Fatalf("got %v want parent %s", got, root)
	}
}

func writeComposeForEnv(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
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
