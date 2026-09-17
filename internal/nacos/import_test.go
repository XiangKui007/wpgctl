package nacos

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
