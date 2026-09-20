package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/wpg/wpgctl/internal/fetch"
	"github.com/wpg/wpgctl/internal/util"
)

// FSEntry 目录浏览条目。
type FSEntry struct {
	Name             string `json:"name"`
	Path             string `json:"path"`
	IsDir            bool   `json:"isDir"`
	HasManifest      bool   `json:"hasManifest,omitempty"`      // 目录内是否含 manifest.yaml
	HasDockerInstall bool   `json:"hasDockerInstall,omitempty"` // 含 offline_install_docker.sh
	IsArchive        bool   `json:"isArchive,omitempty"`        // .zip / .tar / .tar.gz
	ArchiveKind      string `json:"archiveKind,omitempty"`      // zip | tar | tar.gz
}

// FSListResult 目录列表结果。
type FSListResult struct {
	Path    string    `json:"path"`
	Parent  string    `json:"parent"`
	Entries []FSEntry `json:"entries"`
	Roots   []string  `json:"roots,omitempty"` // Windows 盘符等
}

// listFS 列出目录内容，供网页路径选择器使用。
//
// mode: "dir" 只返回目录；"file" 返回目录+文件；"yaml" 返回目录+yaml/yml；
// "nacos-zip" 返回目录 + 文件名以 nacos 开头的 .zip（排除 .tar.zip）；
// "sql" 返回目录 + .sql 文件。
func listFS(path, mode string) (*FSListResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return listRoots()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("路径无效: %w", err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("无法访问: %w", err)
	}
	if !fi.IsDir() {
		abs = filepath.Dir(abs)
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	out := &FSListResult{
		Path:   abs,
		Parent: parentPath(abs),
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(abs, name)
		isDir := e.IsDir()
		if !isDir {
			if mode == "dir" {
				if ak := archiveKind(name); ak != "" {
					out.Entries = append(out.Entries, FSEntry{
						Name: name + "  [" + ak + "]", Path: full, IsArchive: true, ArchiveKind: ak,
					})
				}
				continue
			}
			if mode == "yaml" {
				lower := strings.ToLower(name)
				if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
					continue
				}
			}
			if mode == "nacos-zip" {
				if !isNacosConfigZipName(name) {
					continue
				}
			}
			if mode == "sql" {
				if !strings.HasSuffix(strings.ToLower(name), ".sql") {
					continue
				}
			}
		}
		entry := FSEntry{Name: name, Path: full, IsDir: isDir}
		if isDir {
			if util.FileExists(filepath.Join(full, "manifest.yaml")) {
				entry.HasManifest = true
				entry.Name = name + "  [manifest]"
			}
			if util.FileExists(filepath.Join(full, "offline_install_docker.sh")) ||
				util.FileExists(filepath.Join(full, "docker.service")) {
				entry.HasDockerInstall = true
				if !entry.HasManifest {
					entry.Name = name + "  [docker]"
				}
			}
		}
		out.Entries = append(out.Entries, entry)
	}
	sort.Slice(out.Entries, func(i, j int) bool {
		if out.Entries[i].IsDir != out.Entries[j].IsDir {
			return out.Entries[i].IsDir
		}
		return strings.ToLower(out.Entries[i].Name) < strings.ToLower(out.Entries[j].Name)
	})
	return out, nil
}

func archiveKind(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.zip"):
		return "tar.zip"
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".tar"):
		return "tar"
	default:
		return ""
	}
}

func isNacosConfigZipName(name string) bool {
	lower := strings.ToLower(name)
	if !strings.HasPrefix(lower, "nacos") {
		return false
	}
	if strings.HasSuffix(lower, ".tar.zip") {
		return false
	}
	return strings.HasSuffix(lower, ".zip")
}

func listRoots() (*FSListResult, error) {
	res := &FSListResult{Path: "", Parent: ""}
	if runtime.GOOS == "windows" {
		for _, letter := range "CDEFGHIJKLMNOPQRSTUVWXYZ" {
			p := string(letter) + `:\`
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				res.Roots = append(res.Roots, p)
				res.Entries = append(res.Entries, FSEntry{Name: p, Path: p, IsDir: true})
			}
		}
		// 也加入当前工作目录方便开发
		if wd, err := os.Getwd(); err == nil {
			res.Entries = append([]FSEntry{{Name: "当前目录 · " + wd, Path: wd, IsDir: true}}, res.Entries...)
		}
		return res, nil
	}
	res.Roots = []string{"/"}
	res.Entries = []FSEntry{{Name: "/", Path: "/", IsDir: true}}
	if wd, err := os.Getwd(); err == nil {
		res.Entries = append(res.Entries, FSEntry{Name: "当前目录 · " + wd, Path: wd, IsDir: true})
	}
	if home, err := os.UserHomeDir(); err == nil {
		res.Entries = append(res.Entries, FSEntry{Name: "Home · " + home, Path: home, IsDir: true})
	}
	return res, nil
}

func (s *Server) handleFSExpand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求体无效"})
		return
	}
	if strings.TrimSpace(body.Dir) == "" {
		s.writeJSON(w, 400, map[string]string{"error": "dir 不能为空"})
		return
	}
	res, err := fetch.ExpandArchives(body.Dir)
	if err != nil {
		s.writeJSON(w, 400, map[string]any{"error": err.Error(), "result": res})
		return
	}
	s.writeJSON(w, 200, res)
}

func parentPath(path string) string {
	parent := filepath.Dir(path)
	if parent == path {
		return ""
	}
	// Windows 盘符根 C:\ 的 parent 仍是 C:\
	vol := filepath.VolumeName(path)
	if vol != "" && filepath.Clean(path) == vol+`\` {
		return ""
	}
	return parent
}
