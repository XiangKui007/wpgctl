package moduledeploy

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/fetch"
	"github.com/wpg/wpgctl/internal/util"
)

// NginxLayout middleware/nginx 目录约定。
type NginxLayout struct {
	ModuleDir string // .../middleware/nginx
	HTMLDir   string // .../nginx/html
	ConfDir   string // .../nginx/conf/conf.d 或 .../nginx/conf.d
	WebConf   string // .../http-web-8877.conf
}

const nginxWebConfName = "http-web-8877.conf"

// ResolveNginxLayout 解析 nginx 模块目录（兼容 conf/conf.d、conf.d、同名套层、多层 middleware）。
func ResolveNginxLayout(nginxModuleDir string) NginxLayout {
	root := nginxModuleRoot(nginxModuleDir)
	web := FindNginxWebConf(root)
	html := filepath.Join(root, "html")
	if !util.DirExists(html) {
		if nested := filepath.Join(root, filepath.Base(root), "html"); util.DirExists(nested) {
			html = nested
		}
	}
	return NginxLayout{
		ModuleDir: root,
		HTMLDir:   html,
		ConfDir:   filepath.Dir(web),
		WebConf:   web,
	}
}

func nginxModuleRoot(dir string) string {
	root := filepath.Clean(strings.TrimSpace(dir))
	base := strings.ToLower(filepath.Base(root))
	switch base {
	case "conf.d":
		parent := filepath.Dir(root)
		if strings.EqualFold(filepath.Base(parent), "conf") {
			root = filepath.Dir(parent)
		} else {
			root = parent
		}
	case "conf":
		root = filepath.Dir(root)
	}
	if found := FindNginxModuleDir(root); found != "" {
		return found
	}
	if strings.EqualFold(filepath.Base(root), "nginx") {
		parent := filepath.Dir(root)
		if found := FindNginxModuleDir(parent); found != "" {
			return found
		}
		if found := ModulePath(parent, "nginx"); found != "" && util.DirExists(found) && found != root {
			return found
		}
	}
	nested := filepath.Join(root, filepath.Base(root))
	if util.DirExists(nested) {
		if p := findNginxWebConfExact(nested); p != "" {
			return nested
		}
	}
	return root
}

// FindNginxModuleDir 从包根或错误拼接的 nginx 路径定位真正的 nginx 模块目录。
// 覆盖现场 …/middleware/middleware/nginx/conf/conf.d。
func FindNginxModuleDir(start string) string {
	start = filepath.Clean(strings.TrimSpace(start))
	if start == "" {
		return ""
	}
	var roots []string
	seen := map[string]struct{}{}
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		if !util.DirExists(p) {
			return
		}
		seen[p] = struct{}{}
		roots = append(roots, p)
	}
	add(start)
	if strings.EqualFold(filepath.Base(start), "nginx") {
		add(filepath.Dir(start))
	}
	for _, root := range roots {
		for _, c := range nginxDirCandidates(root) {
			if !util.DirExists(c) {
				continue
			}
			if composeInDir(c) != "" {
				return c
			}
			if p := findNginxWebConfExact(c); p != "" {
				return nginxRootFromWebConf(p)
			}
		}
	}
	return ""
}

func nginxDirCandidates(root string) []string {
	rel := []string{
		"nginx",
		filepath.Join("nginx", "nginx"),
		filepath.Join("middleware", "nginx"),
		filepath.Join("middleware", "middleware", "nginx"),
		filepath.Join("middleware", "middleware", "middleware", "nginx"),
		filepath.Join("middle", "nginx"),
		filepath.Join("middle", "middle", "nginx"),
	}
	out := make([]string, 0, len(rel)+1)
	for _, r := range rel {
		out = append(out, filepath.Join(root, r))
	}
	out = append(out, root)
	return out
}

func nginxRootFromWebConf(conf string) string {
	p := filepath.Dir(filepath.Clean(conf))
	if strings.EqualFold(filepath.Base(p), "conf.d") {
		p = filepath.Dir(p)
		if strings.EqualFold(filepath.Base(p), "conf") {
			p = filepath.Dir(p)
		}
	}
	return p
}

// FindNginxWebConf 在 nginx 模块目录下查找 http-web-8877.conf。
func FindNginxWebConf(root string) string {
	if p := findNginxWebConfExact(root); p != "" {
		return p
	}
	return filepath.Join(root, "conf", "conf.d", nginxWebConfName)
}

func findNginxWebConfExact(root string) string {
	root = filepath.Clean(root)
	candidates := []string{
		filepath.Join(root, "conf", "conf.d", nginxWebConfName),
		filepath.Join(root, "conf.d", nginxWebConfName),
		filepath.Join(root, "conf", nginxWebConfName),
		filepath.Join(root, nginxWebConfName),
		filepath.Join(root, filepath.Base(root), "conf", "conf.d", nginxWebConfName),
		filepath.Join(root, filepath.Base(root), "conf.d", nginxWebConfName),
	}
	for _, p := range candidates {
		if util.FileExists(p) {
			return p
		}
	}
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			lower := strings.ToLower(info.Name())
			if lower == "data" || lower == "logs" || lower == "log" || lower == "html" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(info.Name(), nginxWebConfName) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// ExpandNginxHTML 解压 html 目录下全部 .zip（可选；无 zip 时不报错）。
func ExpandNginxHTML(htmlDir string) (*fetch.ExpandResult, error) {
	if !util.DirExists(htmlDir) {
		return &fetch.ExpandResult{RootDir: htmlDir}, nil
	}
	return fetch.ExpandArchivesOptional(htmlDir)
}
