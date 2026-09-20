package util

import "strings"

// IsLocalIP 判断 ip 是否属于本机（回环或任一网卡地址）。
func IsLocalIP(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false
	}
	if ip == "localhost" {
		return true
	}
	info := DetectHostInfo()
	if ip == info.Loopback || ip == info.Preferred {
		return true
	}
	for _, a := range info.Addresses {
		if a == ip {
			return true
		}
	}
	return false
}
