package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fw "github.com/wpg/wpgctl/internal/firewall"
)

func TestHandleFirewallStatus(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/firewall/status", nil)
	rec := httptest.NewRecorder()
	s.handleFirewallStatus(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out fw.View
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.SSHPort != fw.SSHPort {
		t.Fatalf("sshPort=%d", out.SSHPort)
	}
	if out.UIPort != fw.UIPort {
		t.Fatalf("uiPort=%d", out.UIPort)
	}
	if out.Tool == "" {
		t.Fatal("tool empty")
	}
	if out.Hint == "" {
		t.Fatal("hint empty")
	}
}

func TestHandleFirewallStatusPostEmptyNode(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/firewall/status", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	s.handleFirewallStatus(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
