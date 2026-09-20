package verify

import (
	"net"
	"testing"
	"time"

	"github.com/wpg/wpgctl/internal/config"
)

func TestCheckPorts_dialLocal(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	site := &config.SiteConfig{}
	site.Middleware.Nacos.Host = "127.0.0.1"
	site.Middleware.Nacos.Port = port
	site.Middleware.MySQL.Disabled = true

	rows := checkPorts(site, time.Second)
	found := false
	for _, r := range rows {
		if r.Name == "Nacos" {
			found = true
			if !r.OK {
				t.Fatalf("expected Nacos port open: %+v", r)
			}
		}
	}
	if !found {
		t.Fatalf("nacos row missing: %+v", rows)
	}
}

func TestDialPort_closed(t *testing.T) {
	r := dialPort(portTarget{Name: "x", Host: "127.0.0.1", Port: 1}, 200*time.Millisecond)
	if r.OK {
		t.Fatal("expected closed")
	}
}
