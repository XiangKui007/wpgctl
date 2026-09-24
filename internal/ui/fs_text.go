package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wpg/wpgctl/internal/moduledeploy"
	"github.com/wpg/wpgctl/internal/util"
)

func (s *Server) handleFSText(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		s.writeJSON(w, 400, map[string]string{"error": "path 不能为空"})
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	find := r.URL.Query().Get("find") == "1" || isDotEnvPath(abs) || isComposeFileName(abs) || isNginxWebConfName(abs)

	switch r.Method {
	case http.MethodGet:
		s.getFSText(w, abs, find)
	case http.MethodPut:
		s.putFSText(w, r, abs)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) getFSText(w http.ResponseWriter, abs string, find bool) {
	if util.FileExists(abs) {
		s.writeFSTextOK(w, abs, abs, nil)
		return
	}
	if !find {
		s.writeJSON(w, 404, map[string]string{"error": "文件不存在: " + abs})
		return
	}
	searchRoot := existingAncestor(abs)
	if nr := nginxModuleDirOf(abs); nr != "" {
		if util.DirExists(nr) {
			searchRoot = nr
		} else if found := moduledeploy.FindNginxModuleDir(filepath.Dir(nr)); found != "" {
			searchRoot = found
		} else if found := moduledeploy.FindNginxModuleDir(searchRoot); found != "" {
			searchRoot = found
		}
	}
	if searchRoot == "" {
		s.writeMissingDraft(w, abs)
		return
	}
	var found []string
	if isDotEnvPath(abs) {
		found = findDotEnvFiles(searchRoot)
		found = preferFilesMatchingHint(found, abs)
		if len(found) == 0 {
			if ex := findEnvExample(searchRoot); ex != "" {
				data, err := os.ReadFile(ex)
				if err != nil {
					s.writeJSON(w, 500, map[string]string{"error": err.Error()})
					return
				}
				s.writeJSON(w, 200, map[string]any{
					"path":       filepath.Join(filepath.Dir(ex), ".env"),
					"text":       string(data),
					"size":       len(data),
					"exists":     false,
					"from":       ex,
					"hint":       "目录没有 .env，已用 .env.example 作为初稿；保存后会新建 .env",
					"candidates": []string{ex},
				})
				return
			}
		}
	} else if isComposeFileName(abs) {
		found = findComposeFiles(searchRoot)
		found = preferFilesMatchingHint(found, abs)
	} else if isNginxWebConfName(abs) {
		if p := moduledeploy.FindNginxWebConf(searchRoot); util.FileExists(p) {
			found = []string{p}
		}
	} else {
		found = findNamedFiles(searchRoot, filepath.Base(abs), 5)
	}
	if len(found) > 0 {
		s.writeFSTextOK(w, abs, found[0], found)
		return
	}
	s.writeMissingDraft(w, abs)
}

func (s *Server) writeFSTextOK(w http.ResponseWriter, requested, target string, candidates []string) {
	data, err := os.ReadFile(target)
	if err != nil {
		s.writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	out := map[string]any{
		"path":   target,
		"text":   string(data),
		"size":   len(data),
		"exists": true,
	}
	if len(candidates) > 1 {
		out["candidates"] = candidates
		out["hint"] = fmt.Sprintf("找到 %d 个同名文件，当前打开：%s", len(candidates), target)
	} else if filepath.Clean(target) != filepath.Clean(requested) && !sameComposeFile(requested, target) {
		// 按钮固定请求 docker-compose.yaml，现场文件常是 .yml，同一目录打开成功不算找不到。
		out["hint"] = "请求路径没有该文件，已打开实际位置: " + target
	}
	s.writeJSON(w, 200, out)
}

func (s *Server) writeMissingDraft(w http.ResponseWriter, abs string) {
	hint := "文件尚不存在，保存后会新建"
	if isDotEnvPath(abs) {
		hint = "目录里还没有 .env（可能还在 tar.zip 里）。保存将新建该文件；也可先「批量解压」后再编辑。"
	} else if isComposeFileName(abs) {
		hint = "目录里还没有 docker-compose.yml / docker-compose.yaml（可能还在 tar.zip 里）。保存将新建该文件；也可先解压后再编辑。"
	} else if isNginxWebConfName(abs) {
		hint = "未找到 http-web-8877.conf。常见位置：nginx/conf/conf.d/；多层 middleware 时在 …/middleware/middleware/nginx/conf/conf.d/。"
	}
	s.writeJSON(w, 200, map[string]any{
		"path":   abs,
		"text":   "",
		"size":   0,
		"exists": false,
		"hint":   hint,
	})
}

func existingAncestor(path string) string {
	p := filepath.Dir(filepath.Clean(path))
	for i := 0; i < 10; i++ {
		if util.DirExists(p) {
			return p
		}
		next := filepath.Dir(p)
		if next == p {
			break
		}
		p = next
	}
	return ""
}

func nginxModuleDirOf(path string) string {
	p := filepath.Clean(path)
	for i := 0; i < 12; i++ {
		if strings.EqualFold(filepath.Base(p), "nginx") {
			return p
		}
		next := filepath.Dir(p)
		if next == p {
			break
		}
		p = next
	}
	return ""
}

func isNginxWebConfName(path string) bool {
	return strings.EqualFold(filepath.Base(path), "http-web-8877.conf")
}

func (s *Server) putFSText(w http.ResponseWriter, r *http.Request, abs string) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	if err := util.EnsureDir(filepath.Dir(abs)); err != nil {
		s.writeJSON(w, 500, map[string]string{"error": "创建目录失败: " + err.Error()})
		return
	}
	tmp := abs + ".wpgctl.tmp"
	if err := os.WriteFile(tmp, []byte(body.Text), 0o644); err != nil {
		s.writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if err := os.Rename(tmp, abs); err != nil {
		if err2 := os.WriteFile(abs, []byte(body.Text), 0o644); err2 != nil {
			s.writeJSON(w, 500, map[string]string{"error": fmt.Sprintf("写入失败: %v / %v", err, err2)})
			return
		}
		_ = os.Remove(tmp)
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "path": abs, "size": len(body.Text)})
}

func isDotEnvPath(path string) bool {
	return strings.EqualFold(filepath.Base(path), ".env")
}

// sameComposeFile 同一目录下的 yml / yaml 视为同一个 compose，避免打开成功还提示「没有该文件」。
func sameComposeFile(requested, target string) bool {
	if !isComposeFileName(requested) || !isComposeFileName(target) {
		return false
	}
	return filepath.Clean(filepath.Dir(requested)) == filepath.Clean(filepath.Dir(target))
}

// isComposeFileName 请求的是 compose 文件（yml / yaml 都算）。
func isComposeFileName(path string) bool {
	switch strings.ToLower(filepath.Base(path)) {
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		return true
	default:
		return false
	}
}

// findComposeFiles 在模块目录下找 compose。较浅的优先；同一层优先 docker-compose.yml。
func findComposeFiles(root string) []string {
	var found []string
	seen := map[string]bool{}
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		for _, p := range findNamedFiles(root, name, 4) {
			if seen[p] {
				continue
			}
			seen[p] = true
			found = append(found, p)
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		di, dj := composeDepth(root, found[i]), composeDepth(root, found[j])
		if di != dj {
			return di < dj
		}
		return composeNameRank(found[i]) < composeNameRank(found[j])
	})
	return found
}

func composeDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(rel, string(os.PathSeparator)))
}

func composeNameRank(path string) int {
	switch strings.ToLower(filepath.Base(path)) {
	case "docker-compose.yml":
		return 0
	case "docker-compose.yaml":
		return 1
	case "compose.yml":
		return 2
	default:
		return 3
	}
}

var skipEnvWalkDirs = map[string]bool{
	"data": true, "logs": true, "log": true, "bak": true,
	".git": true, "node_modules": true, "rendered": true,
}

func findDotEnvFiles(root string) []string {
	return findNamedFiles(root, ".env", 4)
}

// preferFilesMatchingHint 编辑 waterwork-device/.env 时，优先打开路径里带 device 的那份，而不是字典序第一份 center。
func preferFilesMatchingHint(found []string, hintAbs string) []string {
	if len(found) <= 1 {
		return found
	}
	hintDir := strings.ToLower(filepath.Base(filepath.Dir(filepath.Clean(hintAbs))))
	if hintDir == "" || hintDir == "." || hintDir == ".env" {
		return found
	}
	var prefer, rest []string
	for _, p := range found {
		if strings.Contains(strings.ToLower(filepath.ToSlash(p)), strings.ToLower(hintDir)) {
			prefer = append(prefer, p)
		} else {
			rest = append(rest, p)
		}
	}
	if len(prefer) == 0 {
		return found
	}
	return append(prefer, rest...)
}

func findEnvExample(root string) string {
	for _, name := range []string{".env.example", ".env.template", "env.example"} {
		found := findNamedFiles(root, name, 3)
		if len(found) > 0 {
			return found[0]
		}
	}
	return ""
}

func findNamedFiles(root, name string, maxDepth int) []string {
	root = filepath.Clean(root)
	want := strings.ToLower(name)
	type hit struct {
		path  string
		score int
	}
	var hits []hit
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = len(strings.Split(rel, string(os.PathSeparator)))
		}
		if info.IsDir() {
			if rel != "." && skipEnvWalkDirs[strings.ToLower(info.Name())] {
				return filepath.SkipDir
			}
			if depth > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(info.Name()) != want {
			return nil
		}
		dir := filepath.Dir(path)
		score := depth * 10
		if util.FileExists(filepath.Join(dir, "docker-compose.yml")) ||
			util.FileExists(filepath.Join(dir, "docker-compose.yaml")) {
			score -= 5
		}
		hits = append(hits, hit{path: path, score: score})
		return nil
	})
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score < hits[j].score
		}
		return hits[i].path < hits[j].path
	})
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.path)
	}
	return out
}
