package moduledeploy

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	fw "github.com/wpg/wpgctl/internal/firewall"
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

// OpenNacosFirewall 部署 Nacos 后立即放行 8848/9848，便于控制台访问与配置导入。
// 无防火墙或 Windows 时跳过并返回说明；放行失败返回 error（调用方可降级为 WARN）。
func OpenNacosFirewall(site *config.SiteConfig) (steps []string, err error) {
	ports := NacosListenPorts(site)
	if runtime.GOOS == "windows" {
		return []string{fmt.Sprintf("跳过防火墙放行（Windows）；请确认本机已放行 TCP %v", ports)}, nil
	}
	st := fw.Inspect()
	if st.Tool == "none" {
		return []string{fmt.Sprintf("未检测到 firewalld/ufw/iptables，请手工放行 TCP %v 后再导入配置", ports)}, nil
	}
	steps = append(steps, fmt.Sprintf("放行 Nacos 端口 %v（%s）…", ports, st.Tool))
	res, err := fw.OpenPortsWithOptions(fw.OpenOptions{Ports: ports, AutoStart: true})
	if res != nil {
		for _, m := range res.Messages {
			steps = append(steps, m)
		}
		if len(res.Opened) > 0 {
			steps = append(steps, fmt.Sprintf("已放行: %v", res.Opened))
		}
		if len(res.Skipped) > 0 {
			steps = append(steps, fmt.Sprintf("已存在规则跳过: %v", res.Skipped))
		}
	}
	if err != nil {
		return steps, fmt.Errorf("放行 Nacos 端口失败: %w", err)
	}
	return steps, nil
}
