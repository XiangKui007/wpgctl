package initenv

import (
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
)

const defaultDockerDataRoot = "/workspace/docker_data/docker/lib"

// DockerDataRoot 返回 dockerd --graph 数据目录。
// 现场工作簿根为 /workspace，其下并列 platform、middleware（或 middle）、sz-waterwork 与 docker_data。
// 对应：ExecStart=/usr/bin/dockerd --graph /workspace/docker_data/docker/lib/
func DockerDataRoot(site *config.SiteConfig) string {
	if site == nil || strings.TrimSpace(site.Paths.Workspace) == "" {
		return defaultDockerDataRoot
	}
	root := workbookRoot(filepath.Clean(site.Paths.Workspace))
	if root == "" || root == "." || root == string(filepath.Separator) {
		return defaultDockerDataRoot
	}
	return filepath.Join(root, "docker_data", "docker", "lib")
}

// workbookRoot 解析工作簿挂载根：目录名 workspace 即本身；旧 site.yaml 写成 /workspace/waterwork 时上溯一层。
func workbookRoot(ws string) string {
	if strings.EqualFold(filepath.Base(ws), "workspace") {
		return ws
	}
	parent := filepath.Dir(ws)
	if strings.EqualFold(filepath.Base(parent), "workspace") {
		return parent
	}
	return ws
}
