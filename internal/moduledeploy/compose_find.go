package moduledeploy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wpg/wpgctl/internal/util"
)

var composeFileNames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

var skipComposeWalkDirs = map[string]bool{
	"data": true, "logs": true, "log": true, "bak": true,
	".git": true, "node_modules": true, "rendered": true,
}

func composeInDir(dir string) string {
	for _, name := range composeFileNames {
		p := filepath.Join(dir, name)
		if util.FileExists(p) {
			return p
		}
	}
	return ""
}

func isComposeFileName(name string) bool {
	lower := strings.ToLower(name)
	for _, n := range composeFileNames {
		if lower == n {
			return true
		}
	}
	return false
}

// findComposeFile 在模块目录查找 compose。兼容 zip 解出的同名套层、多层 middleware，以及选到不存在的 …/middleware/nginx。
func findComposeFile(dir string) (string, error) {
	projects, err := findComposeProjects(dir)
	if err != nil {
		return "", err
	}
	return projects[0], nil
}

// findComposeProjects 返回需要 compose up 的文件列表。
// 市政水厂夹层下通常有 waterwork-center、waterwork-device；GIS 夹层下有 giscenter、gisdefault；都要部署。
func findComposeProjects(dir string) ([]string, error) {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" || dir == "." {
		return nil, fmt.Errorf("模块目录为空")
	}
	if named := findNamedWaterworkComposes(dir); len(named) > 0 {
		return named, nil
	}
	if named := findNamedGISComposes(dir); len(named) > 0 {
		return named, nil
	}
	one, err := findComposeFileRaw(dir)
	if err != nil {
		return nil, err
	}
	return []string{one}, nil
}

// projectsForModule 按 SubService 筛选 compose。空则返回该包全部套件。
func projectsForModule(dir, subService string) ([]string, error) {
	projects, err := findComposeProjects(dir)
	if err != nil {
		return nil, err
	}
	return filterComposeBySubService(projects, subService)
}

// normalizeWaterworkSub 把 UI / CLI 传入的 center、waterwork-device 等归一成 center / device。
func normalizeWaterworkSub(s string) string {
	n := strings.ToLower(strings.TrimSpace(s))
	n = strings.ReplaceAll(n, "_", "-")
	switch {
	case n == "" || n == "all":
		return ""
	case n == "center" || n == "waterwork-center" || strings.HasPrefix(n, "waterwork-center-"):
		return "center"
	case n == "device" || n == "waterwork-device" || strings.HasPrefix(n, "waterwork-device-"):
		return "device"
	default:
		return n
	}
}

func filterComposeBySubService(projects []string, subService string) ([]string, error) {
	want := normalizeWaterworkSub(subService)
	if want == "" {
		return projects, nil
	}
	var out []string
	for _, p := range projects {
		base := filepath.Base(filepath.Dir(p))
		kind := waterworkServiceKind(base)
		if kind == "" {
			kind = gisServiceKind(base)
		}
		if kind == want || strings.EqualFold(base, subService) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("未找到子服务 %s 的 compose（已找到：%s）", subService, composeProjectLabels(projects))
	}
	return out, nil
}

func findComposeFileRaw(dir string) (string, error) {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" || dir == "." {
		return "", fmt.Errorf("模块目录为空")
	}
	for _, root := range composeSearchDirs(dir) {
		if p := composeInDir(root); p != "" {
			return p, nil
		}
		nested := filepath.Join(root, filepath.Base(root))
		if p := composeInDir(nested); p != "" {
			return p, nil
		}
		depth := 4
		if !util.DirExists(dir) && root != dir {
			depth = 5
		}
		if hits := walkComposeFiles(root, depth); len(hits) > 0 {
			return pickComposeFile(hits, root), nil
		}
	}
	return "", fmt.Errorf("未找到 docker-compose.yml（已查当前目录、同名嵌套层及多层 middleware）: %s", dir)
}

func composeSearchDirs(dir string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		p = filepath.Clean(strings.TrimSpace(p))
		if p == "" || p == "." || p == string(filepath.Separator) {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(dir)
	parent := filepath.Dir(dir)
	if parent == dir {
		return out
	}
	name := filepath.Base(dir)
	if strings.EqualFold(filepath.Base(parent), name) {
		add(parent)
	}
	if !looksLikeNestedModulePath(dir) {
		return out
	}
	for _, c := range modulePathCandidates(parent, name) {
		add(c)
	}
	gp := filepath.Dir(parent)
	if gp != parent {
		for _, c := range modulePathCandidates(gp, name) {
			add(c)
		}
	}
	return out
}

func looksLikeNestedModulePath(dir string) bool {
	name := strings.ToLower(filepath.Base(dir))
	parent := strings.ToLower(filepath.Base(filepath.Dir(dir)))
	switch name {
	case "nginx", "mysql", "pgsql", "postgres", "postgis", "mongodb", "redis",
		"kafka", "nacos", "minio", "influxdb", "emqx", "waterjob", "water-job", "water-job-biz":
		return true
	}
	return parent == "middleware" || parent == "middle" || parent == "platform"
}

func walkComposeFiles(root string, maxDepth int) []string {
	root = filepath.Clean(root)
	var out []string
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
			if rel != "." && skipComposeWalkDirs[strings.ToLower(info.Name())] {
				return filepath.SkipDir
			}
			if depth > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if isComposeFileName(info.Name()) {
			out = append(out, path)
		}
		return nil
	})
	return out
}

func pickComposeFile(hits []string, preferRoot string) string {
	if len(hits) == 1 {
		return hits[0]
	}
	preferRoot = filepath.Clean(preferRoot)
	type scored struct {
		path  string
		score int
	}
	var items []scored
	for _, p := range hits {
		dir := filepath.Dir(p)
		rel, _ := filepath.Rel(preferRoot, p)
		depth := 0
		if rel != "" && rel != "." && !strings.HasPrefix(rel, "..") {
			depth = len(strings.Split(rel, string(os.PathSeparator)))
		} else if strings.HasPrefix(rel, "..") {
			depth = 3
		}
		score := depth * 10
		if util.FileExists(filepath.Join(dir, "Dockerfile")) ||
			util.FileExists(filepath.Join(dir, "dockerfile")) {
			score -= 6
		}
		if strings.EqualFold(filepath.Base(p), "docker-compose.yml") {
			score--
		}
		items = append(items, scored{path: p, score: score})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score < items[j].score
		}
		return items[i].path < items[j].path
	})
	return items[0].path
}

func composeInDirOrNested(dir string) string {
	if p := composeInDir(dir); p != "" {
		return p
	}
	return composeInDir(filepath.Join(dir, filepath.Base(dir)))
}

func waterworkServiceKind(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == "waterwork-center" || strings.HasPrefix(n, "waterwork-center-"):
		return "center"
	case n == "waterwork-device" || strings.HasPrefix(n, "waterwork-device-"):
		return "device"
	default:
		return ""
	}
}

// findNamedWaterworkComposes 按 center → device 收集市政水厂两套 compose（取各套最浅的一份）。
func findNamedWaterworkComposes(dir string) []string {
	return findNamedServiceComposes(dir, []string{"center", "device"}, waterworkServiceKind)
}

// findNamedGISComposes 按 giscenter → gisdefault 收集 GIS 两套 compose。
func findNamedGISComposes(dir string) []string {
	return findNamedServiceComposes(dir, []string{"center", "default"}, gisServiceKind)
}

// findNamedServiceComposes 按 kindOf(目录名) 归类，依 order 返回各套最浅的一份 compose。
func findNamedServiceComposes(dir string, order []string, kindOf func(string) string) []string {
	if kindOf == nil || len(order) == 0 {
		return nil
	}
	type hit struct {
		path  string
		depth int
	}
	best := map[string]hit{}
	seenRoot := map[string]struct{}{}
	for _, root := range composeSearchDirs(dir) {
		root = filepath.Clean(root)
		if _, ok := seenRoot[root]; ok {
			continue
		}
		seenRoot[root] = struct{}{}
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || !info.IsDir() {
				return nil
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return nil
			}
			depth := 0
			if rel != "." {
				depth = len(strings.Split(rel, string(os.PathSeparator)))
			}
			if skipComposeWalkDirs[strings.ToLower(info.Name())] && rel != "." {
				return filepath.SkipDir
			}
			if depth > 5 {
				return filepath.SkipDir
			}
			kind := kindOf(info.Name())
			if kind == "" {
				return nil
			}
			p := composeInDirOrNested(path)
			if p == "" {
				return nil
			}
			cur, ok := best[kind]
			if !ok || depth < cur.depth || (depth == cur.depth && p < cur.path) {
				best[kind] = hit{path: p, depth: depth}
			}
			return filepath.SkipDir
		})
	}
	var out []string
	for _, kind := range order {
		if h, ok := best[kind]; ok {
			out = append(out, h.path)
		}
	}
	return out
}
