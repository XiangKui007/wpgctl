package ui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func textReq(method, filePath string, find bool, body *bytes.Buffer) *http.Request {
	q := "/api/fs/text?path=" + url.QueryEscape(filePath)
	if find {
		q += "&find=1"
	}
	if body == nil {
		return httptest.NewRequest(method, q, nil)
	}
	return httptest.NewRequest(method, q, body)
}

func TestGetFSTextFindsNestedEnv(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "public")
	nested := filepath.Join(mod, "app")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(nested, ".env")
	if err := os.WriteFile(env, []byte("NACOS_HOST=1.2.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{}
	req := textReq(http.MethodGet, filepath.Join(mod, ".env"), true, nil)
	rec := httptest.NewRecorder()
	s.handleFSText(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["exists"] != true {
		t.Fatalf("exists: %v", out["exists"])
	}
	got, _ := out["path"].(string)
	if filepath.Clean(got) != filepath.Clean(env) {
		t.Fatalf("path %q want %q", got, env)
	}
	if !strings.Contains(out["text"].(string), "NACOS_HOST") {
		t.Fatalf("text: %v", out["text"])
	}
}

func TestGetFSTextMissingEnvIsEmptyDraft(t *testing.T) {
	mod := t.TempDir()
	s := &Server{}
	req := textReq(http.MethodGet, filepath.Join(mod, ".env"), true, nil)
	rec := httptest.NewRecorder()
	s.handleFSText(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["exists"] != false {
		t.Fatalf("exists: %v", out["exists"])
	}
	if out["text"] != "" {
		t.Fatalf("text: %v", out["text"])
	}
}

func TestPutFSTextCreatesMissingFile(t *testing.T) {
	mod := t.TempDir()
	target := filepath.Join(mod, ".env")
	s := &Server{}
	body := bytes.NewBufferString(`{"text":"FOO=bar\n"}`)
	req := textReq(http.MethodPut, target, false, body)
	rec := httptest.NewRecorder()
	s.handleFSText(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "FOO=bar\n" {
		t.Fatalf("got %q", data)
	}
}

func TestGetFSTextFindsNginxConfUnderConfD(t *testing.T) {
	root := t.TempDir()
	nginx := filepath.Join(root, "middleware", "nginx")
	real := filepath.Join(nginx, "conf", "conf.d")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(real, "http-web-8877.conf")
	if err := os.WriteFile(want, []byte("server {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 现场 UI 有时会请求 nginx/conf.d/（少一层 conf），且该目录并不存在
	wrong := filepath.Join(nginx, "conf.d", "http-web-8877.conf")
	s := &Server{}
	req := textReq(http.MethodGet, wrong, false, nil)
	rec := httptest.NewRecorder()
	s.handleFSText(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	got, _ := out["path"].(string)
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("path %q want %q", got, want)
	}
	if out["exists"] != true {
		t.Fatalf("exists: %v", out["exists"])
	}
}

func TestGetFSTextFindsNginxConfNestedMiddleware(t *testing.T) {
	root := t.TempDir()
	nginx := filepath.Join(root, "middleware", "middleware", "nginx")
	real := filepath.Join(nginx, "conf", "conf.d")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(real, "http-web-8877.conf")
	if err := os.WriteFile(want, []byte("server {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrong := filepath.Join(root, "middleware", "nginx", "conf", "conf.d", "http-web-8877.conf")
	s := &Server{}
	req := textReq(http.MethodGet, wrong, true, nil)
	rec := httptest.NewRecorder()
	s.handleFSText(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	got, _ := out["path"].(string)
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("path %q want %q body %s", got, want, rec.Body.String())
	}
	if out["exists"] != true {
		t.Fatalf("exists: %v", out["exists"])
	}
}

func TestGetFSTextMissingConfStill404(t *testing.T) {
	dir := t.TempDir()
	s := &Server{}
	req := textReq(http.MethodGet, filepath.Join(dir, "http-web.conf"), false, nil)
	rec := httptest.NewRecorder()
	s.handleFSText(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
