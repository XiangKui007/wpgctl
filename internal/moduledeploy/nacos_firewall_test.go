package moduledeploy

import (
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestIsNacosModule(t *testing.T) {
	if !IsNacosModule("/workspace/middleware/nacos") {
		t.Fatal("nacos")
	}
	if !IsNacosModule(`D:\pkg\nacos-server`) {
		t.Fatal("nacos-server")
	}
	if IsNacosModule("/workspace/middleware/kafka") {
		t.Fatal("kafka should be false")
	}
}

func TestNacosListenPorts(t *testing.T) {
	got := NacosListenPorts(nil)
	if len(got) != 2 || got[0] != 8848 || got[1] != 9848 {
		t.Fatalf("default: %v", got)
	}
	site := &config.SiteConfig{}
	site.Middleware.Nacos.Port = 8849
	got = NacosListenPorts(site)
	if got[0] != 8849 || got[1] != 9849 {
		t.Fatalf("custom: %v", got)
	}
}
