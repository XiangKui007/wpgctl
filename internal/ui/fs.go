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
	AlreadyExpanded  bool   `json:"alreadyExpanded,omitempty"`  // zip/tar.zip 的目标已存在
}

// FSListResult 目录列表结果。
type FSListResult struct {
	Path    string     `json:"path"`
	Parent  string     `json:"parent"`
	Entries []FSEntry  `json:"entries"`
	Roots   []string   `json:"roots,omitempty"` // Windows 盘符等
	Hint    FSPickHint `json:"hint,omitempty"`  // 当前层能不能选
}

// listFS 列出目录内容，供网页路径选择器使用。
//
// mode: "dir" 返回目录、普通文件和压缩包；文件只用来对照当前层有没有 compose/.env，点文件不会选中为目录；
// "file" 返回目录+文件；"yaml" 返回目录+yaml/yml；
// "nacos-zip" 返回目录 + 文件名以 nacos 开头的 .zip（排除 .tar.zip）；
// "sql" 返回目录 + .sql 文件。
func listFS(path, mode, target string) (*FSListResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		res, err := listRoots()
		if err != nil {
			return nil, err
		}
		if mode == "dir" {
			res.Hint = judgePickLevel(target, "", nil)
		}
		return res, nil
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
		if strings.HasPrefix(name, ".") && !isListedDotName(name, mode, e.IsDir()) {
			continue
		}
		full := filepath.Join(abs, name)
		isDir := e.IsDir()
		if !isDir {
			if mode == "dir" {
				if ak := archiveKind(name); ak != "" {
					out.Entries = append(out.Entries, FSEntry{
						Name:            name,
						Path:            full,
						IsArchive:       true,
						ArchiveKind:     ak,
						AlreadyExpanded: fetch.ArchiveExpanded(full),
					})
					continue
				}
				// 普通文件继续加入列表，前端只展示、不当作目录选中。
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
	if mode == "dir" {
		out.Hint = judgePickLevel(target, out.Path, out.Entries)
	}
	return out, nil
}

// isListedDotName 目录浏览里放出 .env，方便对照是否选到模块根；其它点文件 / 点目录仍隐藏。
func isListedDotName(name, mode string, isDir bool) bool {
	if isDir || (mode != "dir" && mode != "file") {
		return false
	}
	lower := strings.ToLower(name)
	return lower == ".env" || strings.HasPrefix(lower, ".env.")
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

// handleFSExpand 异步解压目录或单个 zip，返回 job（前端轮询进度日志）。
func (s *Server) handleFSExpand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Dir  string `json:"dir"`
		File string `json:"file"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求体无效"})
		return
	}
	dir := strings.TrimSpace(body.Dir)
	file := strings.TrimSpace(body.File)
	if dir == "" && file == "" {
		s.writeJSON(w, 400, map[string]string{"error": "请指定要解压的目录或压缩包"})
		return
	}
	job := s.newJob("fs-expand")
	go s.runFSExpand(job, dir, file)
	s.writeJSON(w, 200, job)
}

// expandJobState 解压任务进度，写入 Job.Result 供前端进度条。
type expandJobState struct {
	RootDir      string   `json:"rootDir"`
	File         string   `json:"file,omitempty"`
	Steps        []string `json:"steps"`
	ZipExtracted int      `json:"zipExtracted"`
	TarUnwrapped int      `json:"tarUnwrapped"`
	TarFiles     []string `json:"tarFiles,omitempty"`
	TarGzFiles   []string `json:"tarGzFiles,omitempty"`
	Done         int      `json:"done"`
	Total        int      `json:"total"`
	Percent      int      `json:"percent"`
	PackPercent  int      `json:"packPercent,omitempty"`
	Current      string   `json:"current,omitempty"`
	Entry        string   `json:"entry,omitempty"`
	FileDone     int      `json:"fileDone,omitempty"`
	FileTotal    int      `json:"fileTotal,omitempty"`
	BytesDone    int64    `json:"bytesDone,omitempty"`
	BytesTotal   int64    `json:"bytesTotal,omitempty"`
}

func (s *Server) runFSExpand(job *Job, dir, file string) {
	if file != "" {
		s.appendLog(job, "解压压缩包: "+file)
	} else {
		s.appendLog(job, "解压目录: "+dir)
	}
	state := &expandJobState{File: file, RootDir: dir}
	s.setJobResult(job, state)
	res, err := fetch.Expand(fetch.ExpandOptions{
		Root: dir,
		File: file,
		Log:  func(line string) { s.appendLog(job, line) },
		Progress: func(p fetch.ExpandProgress) {
			s.mu.Lock()
			state.Done = p.Done
			state.Total = p.Total
			state.Percent = p.Percent
			state.PackPercent = p.PackPercent
			state.Current = p.Current
			state.Entry = p.Entry
			state.FileDone = p.FileDone
			state.FileTotal = p.FileTotal
			state.BytesDone = p.BytesDone
			state.BytesTotal = p.BytesTotal
			s.mu.Unlock()
		},
	})
	if res != nil {
		s.mu.Lock()
		state.RootDir = res.RootDir
		state.Steps = res.Steps
		state.ZipExtracted = res.ZipExtracted
		state.TarUnwrapped = res.TarUnwrapped
		state.TarFiles = res.TarFiles
		state.TarGzFiles = res.TarGzFiles
		if err == nil {
			state.Percent = 100
			state.Current = ""
		}
		s.mu.Unlock()
	}
	if err != nil {
		s.failJob(job, err.Error())
		return
	}
	msg := fmt.Sprintf("解压完成：zip=%d tar.zip=%d .tar=%d → %s",
		state.ZipExtracted, state.TarUnwrapped, len(state.TarFiles), state.RootDir)
	if file != "" {
		msg = fmt.Sprintf("解压完成：%s → %s", filepath.Base(file), state.RootDir)
	} else if state.ZipExtracted == 0 && state.TarUnwrapped == 0 {
		msg = fmt.Sprintf("当前目录已展开过，发现 %d 个 .tar：%s", len(state.TarFiles), state.RootDir)
	}
	s.okJob(job, msg)
}

func (s *Server) setJobResult(j *Job, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Result = result
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

// handleFSExists 批量判断路径是否仍在磁盘上（用于清掉已删除目录的残留填写）。
func (s *Server) handleFSExists(w http.ResponseWriter, r *http.Request) {
	var paths []string
	switch r.Method {
	case http.MethodGet:
		paths = r.URL.Query()["path"]
	case http.MethodPost:
		var body struct {
			Paths []string `json:"paths"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		paths = body.Paths
	default:
		http.Error(w, "method not allowed", 405)
		return
	}
	exists := map[string]bool{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		_, err := os.Stat(p)
		exists[p] = err == nil
	}
	s.writeJSON(w, 200, map[string]any{"exists": exists})
}
