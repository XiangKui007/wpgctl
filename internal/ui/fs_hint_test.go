package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJudgePickLevel_MiddlewareReady(t *testing.T) {
	h := judgePickLevel("middlewareRoot", `/workspace/middleware`, []FSEntry{
		{Name: "mysql", Path: `/workspace/middleware/mysql`, IsDir: true},
		{Name: "redis", Path: `/workspace/middleware/redis`, IsDir: true},
		{Name: "nacos", Path: `/workspace/middleware/nacos`, IsDir: true},
		{Name: "readme.txt", Path: `/workspace/middleware/readme.txt`},
	})
	if h.Level != "ready" {
		t.Fatalf("level=%s msg=%s", h.Level, h.Message)
	}
	if len(h.Marks) < 2 {
		t.Fatalf("marks=%v", h.Marks)
	}
}

func TestJudgePickLevel_MiddlewareNested(t *testing.T) {
	h := judgePickLevel("middlewareRoot", `/data/pkg`, []FSEntry{
		{Name: "middleware", Path: `/data/pkg/middleware`, IsDir: true},
		{Name: "readme.txt", Path: `/data/pkg/readme.txt`},
	})
	if h.Level != "deeper" || len(h.Enter) == 0 || h.Enter[0] != "middleware" {
		t.Fatalf("%+v", h)
	}
}

func TestJudgePickLevel_MiddlewareTooDeep(t *testing.T) {
	h := judgePickLevel("middlewareRoot", `/workspace/middleware/mysql`, []FSEntry{
		{Name: "docker-compose.yml", Path: `/workspace/middleware/mysql/docker-compose.yml`},
		{Name: ".env", Path: `/workspace/middleware/mysql/.env`},
		{Name: "data", Path: `/workspace/middleware/mysql/data`, IsDir: true},
	})
	if h.Level != "up" {
		t.Fatalf("level=%s %+v", h.Level, h)
	}
	h = judgePickLevel("middlewareRoot", `/workspace/middleware/middleware/mysql/data`, []FSEntry{
		{Name: "ibdata1", Path: `/workspace/middleware/middleware/mysql/data/ibdata1`},
	})
	if h.Level != "up" {
		t.Fatalf("nested data level=%s %+v", h.Level, h)
	}
	h = judgePickLevel("middlewareRoot", `/workspace/middleware/middleware/docker_package`, []FSEntry{
		{Name: "offline_install_docker.sh", Path: `/workspace/middleware/middleware/docker_package/offline_install_docker.sh`},
	})
	if h.Level != "up" {
		t.Fatalf("docker_package level=%s %+v", h.Level, h)
	}
}

func TestJudgePickLevel_PackageManifest(t *testing.T) {
	h := judgePickLevel("package", `/opt/rel`, []FSEntry{
		{Name: "manifest.yaml", Path: `/opt/rel/manifest.yaml`},
		{Name: "images", Path: `/opt/rel/images`, IsDir: true},
	})
	if h.Level != "ready" {
		t.Fatalf("%+v", h)
	}
	h = judgePickLevel("package", `/opt`, []FSEntry{
		{Name: "rel  [manifest]", Path: `/opt/rel`, IsDir: true, HasManifest: true},
	})
	if h.Level != "deeper" || h.Enter[0] != "rel" {
		t.Fatalf("%+v", h)
	}
}

func TestJudgePickLevel_Waterwork(t *testing.T) {
	h := judgePickLevel("waterworkDir", `/pkg/waterwork`, []FSEntry{
		{Name: "waterwork-center", Path: `/pkg/waterwork/waterwork-center`, IsDir: true},
		{Name: "waterwork-device", Path: `/pkg/waterwork/waterwork-device`, IsDir: true},
	})
	if h.Level != "ready" {
		t.Fatalf("%+v", h)
	}
	h = judgePickLevel("waterworkDir", `/pkg/waterwork/waterwork-center`, []FSEntry{
		{Name: "docker-compose.yml", Path: `/pkg/waterwork/waterwork-center/docker-compose.yml`},
	})
	if h.Level != "up" {
		t.Fatalf("%+v", h)
	}
}

func TestListFS_HintUsesTarget(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"mysql", "redis", "nacos"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	res, err := listFS(dir, "dir", "middlewareRoot")
	if err != nil {
		t.Fatal(err)
	}
	if res.Hint.Level != "ready" {
		t.Fatalf("hint=%+v", res.Hint)
	}
}
