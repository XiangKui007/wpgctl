package initenv

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wpg/wpgctl/internal/util"
)

// ResolveWorkbookPath 把「开始前」所选父目录解析成工作簿根。
// 选 / 或盘符根 → {根}/workspace；路径里已有名为 workspace 的祖先则用那一层（避免把交付包目录当工作簿）。
// 相对路径（例如只填了包名）忽略，回落默认 /workspace。空输入同默认。
func ResolveWorkbookPath(parent string) string {
	p := strings.TrimSpace(parent)
	if p == "" {
		return defaultWorkbookPath()
	}
	p = filepath.Clean(p)
	if isPathRoot(p) {
		return filepath.Join(p, "workspace")
	}
	if !filepath.IsAbs(p) {
		return defaultWorkbookPath()
	}
	if ws := workbookAncestor(p); ws != "" {
		return ws
	}
	return filepath.Join(p, "workspace")
}

func defaultWorkbookPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Clean("D:/workspace")
	}
	return "/workspace"
}

func isPathRoot(p string) bool {
	if p == string(filepath.Separator) || p == "/" {
		return true
	}
	vol := filepath.VolumeName(p)
	if vol == "" {
		return false
	}
	rest := strings.TrimPrefix(p, vol)
	return rest == "" || rest == `\` || rest == "/"
}

// workbookAncestor 从 path 向上找到名为 workspace 的目录；没有则返回空串。
func workbookAncestor(path string) string {
	cur := path
	for i := 0; i < 32; i++ {
		base := filepath.Base(cur)
		if strings.EqualFold(base, "workspace") {
			return cur
		}
		if isPathRoot(cur) {
			return ""
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
	return ""
}

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

// workspaceDirList 开始前只探测/创建工作簿根本身，不建交付包、不建 rendered/bak。
func workspaceDirList(workspace string) []DirStatus {
	ws := strings.TrimSpace(workspace)
	if ws == "" {
		return nil
	}
	return []DirStatus{{Path: ws, Label: "workspace"}}
}

// ProbeWorkspace 检查工作簿根是否已存在（仅探测，不创建）。
func ProbeWorkspace(workspace, _ string) (*WorkspaceProbe, error) {
	ws := strings.TrimSpace(workspace)
	if ws == "" {
		return nil, fmt.Errorf("workspace 路径不能为空")
	}
	probe := &WorkspaceProbe{
		Workspace: ws,
		Dirs:      workspaceDirList(ws),
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

// InitWorkspaceDirs 开始前只创建工作簿根（如 /workspace）；已有则跳过。
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
