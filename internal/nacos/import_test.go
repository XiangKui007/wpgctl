package nacos

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsNacosConfigZip(t *testing.T) {
	ok := []string{"nacos.zip", "nacos-config-export.zip", `D:\pkg\Nacos_2024.zip`}
	for _, p := range ok {
		if !IsNacosConfigZip(p) {
			t.Fatalf("want true: %s", p)
		}
	}
	bad := []string{"kafka.zip", "nacos.tar.zip", "readme.txt", "nacos.yml"}
	for _, p := range bad {
		if IsNacosConfigZip(p) {
			t.Fatalf("want false: %s", p)
		}
	}
}

func TestClientImportConfigZip(t *testing.T) {
	var gotImport, gotNS, gotPolicy, gotFile string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/nacos/v1/auth/login":
			_ = json.NewEncoder(w).Encode(loginResp{AccessToken: "tok", TokenTTL: 18000})
		case r.URL.Path == "/nacos/v1/cs/configs" && r.URL.Query().Get("import") == "true":
			gotImport = "true"
			gotNS = r.URL.Query().Get("namespace")
			_ = r.ParseMultipartForm(4 << 20)
			gotPolicy = r.FormValue("policy")
			_, hdr, err := r.FormFile("file")
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			gotFile = hdr.Filename
			_, _ = w.Write([]byte(`{"code":200,"message":"success"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "nacos-demo.zip")
	if err := os.WriteFile(zipPath, []byte("PK\x03\x04fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Client{Base: srv.URL, Username: "nacos", Password: "p", HTTP: srv.Client()}
	if err := c.login(); err != nil {
		t.Fatal(err)
	}
	if err := c.ImportConfigZip("intergrate", zipPath, "OVERWRITE"); err != nil {
		t.Fatal(err)
	}
	if gotImport != "true" || gotNS != "intergrate" || gotPolicy != "OVERWRITE" || gotFile != "nacos-demo.zip" {
		t.Fatalf("got import=%s ns=%s policy=%s file=%s", gotImport, gotNS, gotPolicy, gotFile)
	}
}

func TestLoginWithRetryEventuallySucceeds(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nacos/v1/auth/login" {
			http.NotFound(w, r)
			return
		}
		n++
		if n < 3 {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(loginResp{AccessToken: "tok", TokenTTL: 18000})
	}))
	defer srv.Close()
	c := &Client{Base: srv.URL, Username: "nacos", Password: "p", HTTP: srv.Client()}
	if err := c.loginWithRetry(8 * time.Second); err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Fatalf("retries=%d", n)
	}
}

func TestClientLoginAndPublish(t *testing.T) {
	const token = "test-token-abc"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nacos/v1/auth/login":
			_ = r.ParseForm()
			if r.Form.Get("username") != "nacos" || r.Form.Get("password") != "secret" {
				http.Error(w, "bad creds", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(loginResp{AccessToken: token, TokenTTL: 18000})
		case "/nacos/v1/cs/configs":
			if r.URL.Query().Get("accessToken") != token && r.FormValue("accessToken") != token {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"user not found!"}`))
				return
			}
			if r.Method == http.MethodPost {
				w.Write([]byte("true"))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{
		Base:     srv.URL,
		Username: "nacos",
		Password: "secret",
		HTTP:     srv.Client(),
	}
	if err := c.login(); err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := c.PublishConfig("intergrate", "waterwork-center.yml", "DEFAULT_GROUP", "k: v"); err != nil {
		t.Fatalf("publish: %v", err)
	}
}
