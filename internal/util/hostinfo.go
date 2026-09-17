package util

import (
	"net"
	"os"
	"sort"
	"strings"
)

// HostInfo 本机网络信息，供配站自动填充。
type HostInfo struct {
	Hostname  string   `json:"hostname"`
	Preferred string   `json:"preferred"` // 推荐用于现场交付的地址
	Loopback  string   `json:"loopback"`  // 通常 127.0.0.1
	Addresses []string `json:"addresses"` // 全部可用 IPv4（不含 link-local）
}

// DetectHostInfo 探测本机主机名与 IPv4 地址。
func DetectHostInfo() HostInfo {
	info := HostInfo{
		Loopback: "127.0.0.1",
	}
	if h, err := os.Hostname(); err == nil {
		info.Hostname = h
	}

	seen := map[string]struct{}{}
	add := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" || ip == "0.0.0.0" {
			return
		}
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			return
		}
		if parsed.IsLinkLocalUnicast() {
			return
		}
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		info.Addresses = append(info.Addresses, ip)
	}

	// 枚举网卡
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil {
				add(ip.String())
			}
		}
	}

	// UDP 出站探测：通常能拿到连外网/内网网关的主网卡 IP
	if pref := preferredOutboundIPv4(); pref != "" {
		add(pref)
		info.Preferred = pref
	}

	sort.Strings(info.Addresses)

	if info.Preferred == "" {
		for _, ip := range info.Addresses {
			if !isLoopbackIP(ip) {
				info.Preferred = ip
				break
			}
		}
	}
	if info.Preferred == "" {
		info.Preferred = info.Loopback
	}
	return info
}

func preferredOutboundIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// 无外网时尝试常见内网探测地址（不会真正发包建立连接以外的依赖）
		conn, err = net.Dial("udp", "192.168.0.1:80")
		if err != nil {
			return ""
		}
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return ""
	}
	ip := addr.IP.To4()
	if ip == nil {
		return ""
	}
	return ip.String()
}

func isLoopbackIP(ip string) bool {
	p := net.ParseIP(ip)
	return p != nil && p.IsLoopback()
}
