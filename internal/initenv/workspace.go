package initenv

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

// DirStatus 单个目录探测结果。
type DirStatus struct {
	Path   string `json:"path"`
	Label  string `json:"label"`
	Exists bool   `json:"exists"`
}

// WorkspaceProbe 工作簿目录探测摘要。
type WorkspaceProbe struct {
	Workspace string      `json:"workspace"`
	NginxHTML string      `json:"nginxHtml,omitempty"`
	Ready     bool        `json:"ready"` // 关键目录均已存在
	Dirs      []DirStatus `json:"dirs"`
}

// workspaceDirList 返回工作簿相关目录（探测 / 创建共用）。
func workspaceDirList(workspace, nginxHTML string) []DirStatus {
	ws := strings.TrimSpace(workspace)
	html := strings.TrimSpace(nginxHTML)
	out := []DirStatus{
		{Path: ws, Label: "workspace"},
		{Path: filepath.Join(ws, "rendered"), Label: "rendered"},
		{Path: filepath.Join(ws, "bak"), Label: "bak"},
	}
	if html != "" {
		out = append(out, DirStatus{Path: html, Label: "nginxHtml"})
	}
	if runtime.GOOS == "linux" && ws != "" {
		site := &config.SiteConfig{Paths: config.PathsConfig{Workspace: ws}}
		out = append(out, DirStatus{Path: DockerDataRoot(site), Label: "docker_data"})
	}
	return out
}

// ProbeWorkspace 检查工作簿目录是否已初始化（仅探测，不创建）。
func ProbeWorkspace(workspace, nginxHTML string) (*WorkspaceProbe, error) {
	ws := strings.TrimSpace(workspace)
	if ws == "" {
		return nil, fmt.Errorf("workspace 路径不能为空")
	}
	probe := &WorkspaceProbe{
		Workspace: ws,
		NginxHTML: strings.TrimSpace(nginxHTML),
		Dirs:      workspaceDirList(ws, nginxHTML),
		Ready:     true,
	}
	for i := range probe.Dirs {
		probe.Dirs[i].Exists = util.DirExists(probe.Dirs[i].Path)
		if !probe.Dirs[i].Exists {
			probe.Ready = false
		}
	}
	return probe, nil
}

// InitWorkspaceDirs 仅创建工作簿相关目录（不装 Docker、不开防火墙）。
func InitWorkspaceDirs(workspace, nginxHTML string) (*WorkspaceProbe, []string, error) {
	probe, err := ProbeWorkspace(workspace, nginxHTML)
	if err != nil {
		return nil, nil, err
	}
	var created []string
	for i := range probe.Dirs {
		d := &probe.Dirs[i]
		if d.Exists {
			continue
		}
		if err := util.EnsureDir(d.Path); err != nil {
			return probe, created, fmt.Errorf("创建目录失败 %s: %w", d.Path, err)
		}
		d.Exists = true
		created = append(created, d.Path)
	}
	probe.Ready = true
	return probe, created, nil
}
