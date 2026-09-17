package initenv

import (
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

func isLocalNodeIP(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false
	}
	info := util.DetectHostInfo()
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

func remoteNodes(all []config.Node) []config.Node {
	out := make([]config.Node, 0, len(all))
	for _, n := range all {
		if isLocalNodeIP(n.IP) {
			continue
		}
		out = append(out, n)
	}
	return out
}
