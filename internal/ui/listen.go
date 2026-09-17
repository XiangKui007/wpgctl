package ui

import (
	"net"
	"runtime"

	"github.com/wpg/wpgctl/internal/util"
)

// DefaultListen 返回 UI 默认监听地址。
// Linux 现场默认 0.0.0.0，便于用服务器 IP 访问；Windows 联调仍仅本机。
func DefaultListen() string {
	if runtime.GOOS == "linux" {
		return "0.0.0.0:9527"
	}
	return "127.0.0.1:9527"
}

func printAccessHints(listen string) {
	util.Successf("Web 控制台已启动 (监听 %s)", listen)
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		util.Infof("浏览器访问: http://%s", listen)
		return
	}
	if isAllInterfaces(host) {
		util.Infof("本机: http://127.0.0.1:%s", port)
		info := util.DetectHostInfo()
		if info.Preferred != "" && info.Preferred != "127.0.0.1" {
			util.Infof("推荐: http://%s:%s", info.Preferred, port)
		}
		for _, ip := range info.Addresses {
			if ip == info.Preferred {
				continue
			}
			util.Infof("      http://%s:%s", ip, port)
		}
		util.Infof("远程浏览器用上述 IP 访问；仅本机使用时请加 --listen 127.0.0.1:%s", port)
		return
	}
	util.Infof("浏览器访问: http://%s", net.JoinHostPort(host, port))
	if runtime.GOOS == "windows" {
		util.Infof("适配向日葵/ToDesk：远程桌面内打开浏览器访问上述地址")
	}
}

func isAllInterfaces(host string) bool {
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		return true
	default:
		return false
	}
}
