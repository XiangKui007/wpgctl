package moduledeploy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// nginx 现场包用 host 网络，compose 里没有 ports；真正监听的端口写在 conf/**/*.conf 的 listen 指令里。
// 只靠 compose/.env 解析会得到「无端口，跳过放行」，前端 8877 就被防火墙挡住。

// NginxWebPort 前端入口端口：http-web-8877.conf 固定监听 8877，不论 conf 里怎么写都必须放行。
const NginxWebPort = 8877

// reNginxListen 匹配 listen 8877; / listen 0.0.0.0:8877; / listen [::]:8877 ssl; / listen *:80 default_server;
var reNginxListen = regexp.MustCompile(`(?im)^\s*listen\s+(?:\*:|\d{1,3}(?:\.\d{1,3}){3}:|\[[0-9a-f:]*\]:)?(\d{2,5})\b`)

// IsNginxModule 目录名为 nginx，或目录下能找到 http-web-8877.conf。
func IsNginxModule(moduleDir string) bool {
	dir := strings.TrimSpace(moduleDir)
	if dir == "" {
		return false
	}
	if strings.EqualFold(filepath.Base(dir), "nginx") {
		return true
	}
	return findNginxWebConfExact(dir) != ""
}

// NginxListenPorts 收集 nginx 模块 conf 里全部 listen 端口，并保证含 8877。
// 跳过 html/data/logs，避免遍历几万个静态文件。
func NginxListenPorts(moduleDir string) []int {
	ports := []int{NginxWebPort}
	dir := strings.TrimSpace(moduleDir)
	if dir == "" {
		return ports
	}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			switch strings.ToLower(info.Name()) {
			case "html", "data", "logs", "log", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(info.Name()), ".conf") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, m := range reNginxListen.FindAllStringSubmatch(string(data), -1) {
			if len(m) > 1 {
				if p := parsePortNumber(m[1]); p > 0 {
					ports = append(ports, p)
				}
			}
		}
		return nil
	})
	return uniqueInts(ports)
}
