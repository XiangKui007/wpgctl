package moduledeploy

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	fw "github.com/wpg/wpgctl/internal/firewall"
)

// OpenModuleFirewall 在 compose up 之前，按本模块 compose/.env 解析出的 TCP 端口幂等放行。
// Windows / 未装防火墙 / 解析不到端口时跳过并写说明；放行失败返回 error（调用方记 WARN，不阻断部署）。
func OpenModuleFirewall(moduleDir string, composeFiles []string, site *config.SiteConfig) (steps []string, err error) {
	ports := CollectListenPorts(moduleDir, composeFiles, site)
	if len(ports) == 0 {
		return []string{"未从 compose/.env 解析到监听端口，跳过防火墙放行"}, nil
	}
	label := filepath.Base(strings.TrimSpace(moduleDir))
	return openListenPorts("模块 "+label, ports)
}

func openListenPorts(label string, ports []int) (steps []string, err error) {
	if runtime.GOOS == "windows" {
		return []string{fmt.Sprintf("跳过防火墙放行（Windows）；请确认本机已放行 TCP %v", ports)}, nil
	}
	st := fw.Inspect()
	if st.Tool == "none" {
		return []string{fmt.Sprintf("未检测到 firewalld/ufw/iptables，请手工放行 TCP %v", ports)}, nil
	}
	steps = append(steps, fmt.Sprintf("放行 %s 端口 %v（%s）…", label, ports, st.Tool))
	res, err := fw.OpenPortsWithOptions(fw.OpenOptions{Ports: ports, AutoStart: true})
	if res != nil {
		steps = append(steps, res.Messages...)
		if len(res.Opened) > 0 {
			steps = append(steps, fmt.Sprintf("已放行: %v", res.Opened))
		}
		if len(res.Skipped) > 0 {
			steps = append(steps, fmt.Sprintf("已存在规则跳过: %v", res.Skipped))
		}
	}
	if err != nil {
		return steps, fmt.Errorf("放行 %s 端口失败: %w", label, err)
	}
	return steps, nil
}
