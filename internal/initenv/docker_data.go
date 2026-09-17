package initenv

import (
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
)

// DockerDataRoot 返回 dockerd --graph 数据目录（与 workspace 同挂载盘）。
func DockerDataRoot(site *config.SiteConfig) string {
	if site != nil && strings.TrimSpace(site.Paths.Workspace) != "" {
		ws := filepath.Clean(site.Paths.Workspace)
		parent := filepath.Dir(ws)
		if parent != "" && parent != "." {
			return filepath.Join(parent, "docker_data", "docker", "lib")
		}
	}
	return "/workspace/docker_data/docker/lib"
}
