package initenv

import (
	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

func isLocalNodeIP(ip string) bool {
	return util.IsLocalIP(ip)
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
