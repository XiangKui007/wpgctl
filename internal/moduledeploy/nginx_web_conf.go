package moduledeploy

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/wpg/wpgctl/internal/util"
)

// NginxProxyIPs 写入 http-web-8877.conf 的 proxy_pass 目标 IP。
type NginxProxyIPs struct {
	Gateway string // /main/ → :18094
	App     string // opc-ua :18089、msgCenter :9006 等
	Graph   string // 组态编辑器 :5588
}

// nginxProxyPortRole 仅替换现场需改的内网 proxy 端口；外网地址（如 47.114.x）不动。
var nginxProxyPortRole = map[int]string{
	18094: "gateway",
	18089: "app",
	5588:  "graph",
	9006:  "app",
	10103: "app", // /bzSocket/
}

var reProxyPass = regexp.MustCompile(`(?i)(proxy_pass\s+https?://)(\d{1,3}(?:\.\d{1,3}){3})(:\d+)([^;]*);`)

// PatchHTTPWeb8877 按端口替换 http-web-8877.conf 内 proxy_pass IP（对齐现场手工改网关）。
func PatchHTTPWeb8877(confPath string, ips NginxProxyIPs) ([]string, error) {
	if !util.FileExists(confPath) {
		return nil, fmt.Errorf("未找到 %s", confPath)
	}
	if ips.Gateway == "" {
		return nil, fmt.Errorf("网关 IP 不能为空")
	}
	if ips.App == "" {
		ips.App = ips.Gateway
	}
	if ips.Graph == "" {
		ips.Graph = ips.Gateway
	}
	data, err := os.ReadFile(confPath)
	if err != nil {
		return nil, err
	}
	orig := string(data)
	var notes []string
	out := reProxyPass.ReplaceAllStringFunc(orig, func(line string) string {
		m := reProxyPass.FindStringSubmatch(line)
		if len(m) < 5 {
			return line
		}
		portStr := strings.TrimPrefix(m[3], ":")
		var port int
		fmt.Sscanf(portStr, "%d", &port)
		role, ok := nginxProxyPortRole[port]
		if !ok {
			return line
		}
		var newIP string
		switch role {
		case "gateway":
			newIP = ips.Gateway
		case "graph":
			newIP = ips.Graph
		default:
			newIP = ips.App
		}
		if m[2] == newIP {
			return line
		}
		notes = append(notes, fmt.Sprintf(":%d %s→%s", port, m[2], newIP))
		return m[1] + newIP + m[3] + m[4] + ";"
	})
	if out == orig {
		return notes, nil
	}
	if err := os.WriteFile(confPath, []byte(out), 0o644); err != nil {
		return nil, err
	}
	return notes, nil
}
