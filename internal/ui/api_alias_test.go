package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
)

// /wpg-deploy-api/* 必须与 /api/* 等价：现场经业务 Nginx 8877 反代时只转发带产品名的前缀。
func TestAPIAliasEqualsAPI(t *testing.T) {
	s := &Server{
		mux:  http.NewServeMux(),
		up:   websocket.Upgrader{},
		jobs: map[string]*Job{},
	}
	s.routes()

	for _, path := range []string{"/api/health", APIAlias + "/health"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: code=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}

	// 带查询串与子路径也要正确改写
	req := httptest.NewRequest(http.MethodGet, APIAlias+"/jobs/does-not-exist", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound && rec.Body.String() == "404 page not found\n" {
		t.Fatalf("alias not routed to /api/jobs/: %d %s", rec.Code, rec.Body.String())
	}
}
