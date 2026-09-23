package moduledeploy

import (
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
)

// IsNacosModule 目录名是否为 nacos（兼容 nacos-server）。
func IsNacosModule(moduleDir string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(moduleDir)))
	return base == "nacos" || strings.HasPrefix(base, "nacos-")
}

// NacosListenPorts 返回 Nacos HTTP + gRPC 端口（默认 8848 / 9848）。
func NacosListenPorts(site *config.SiteConfig) []int {
	port := 8848
	if site != nil && site.Middleware.Nacos.Port > 0 {
		port = site.Middleware.Nacos.Port
	}
	return []int{port, port + 1000}
}

// OpenNacosFirewall 按 site.yaml 放行 Nacos HTTP + gRPC 端口（默认 8848/9848）。
// 模块部署请走 OpenModuleFirewall（会解析 compose 并包含 gRPC +1000）。
func OpenNacosFirewall(site *config.SiteConfig) (steps []string, err error) {
	return openListenPorts("Nacos", NacosListenPorts(site))
}
