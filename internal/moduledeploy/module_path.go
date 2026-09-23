package moduledeploy

import (
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/util"
)

// nestedPackNames 现场 zip 常解出同名套层：middleware/middleware、middle/middle、platform/platform。
var nestedPackNames = []string{"middleware", "middle", "platform"}

// ModulePath 拼接包根 + 模块名。
// 兼容多层套包，例如：
//
//	root/nginx
//	root/middle/nginx
//	/workspace/middle/middle/nginx
//	root/middleware/middleware/nginx
func ModulePath(root, name string) string {
	name = strings.TrimSpace(name)
	if strings.TrimSpace(root) == "" || name == "" {
		return ""
	}
	fallback := filepath.Join(root, moduleNameAliases(name)[0])
	if strings.EqualFold(name, "nginx") {
		if found := FindNginxModuleDir(root); found != "" {
			return found
		}
	}
	for _, alias := range moduleNameAliases(name) {
		for _, p := range modulePathCandidates(root, alias) {
			if util.DirExists(p) {
				return p
			}
		}
	}
	return fallback
}

// moduleNameAliases 目录别名：waterjob 交付包常见 water-job / water-job-biz。
func moduleNameAliases(name string) []string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "waterjob", "water-job", "water-job-biz":
		return []string{"waterjob", "water-job", "water-job-biz"}
	default:
		if name == "" {
			return []string{""}
		}
		return []string{name}
	}
}

func modulePathCandidates(root, name string) []string {
	root = filepath.Clean(strings.TrimSpace(root))
	name = strings.TrimSpace(name)
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(filepath.Join(root, name))
	cur := root
	for i := 0; i < 4; i++ {
		base := filepath.Base(cur)
		inner := filepath.Join(cur, base)
		if !util.DirExists(inner) || inner == cur {
			break
		}
		add(filepath.Join(inner, name))
		cur = inner
	}
	for _, pack := range nestedPackNames {
		p1 := filepath.Join(root, pack)
		add(filepath.Join(p1, name))
		add(filepath.Join(p1, pack, name))
		add(filepath.Join(p1, pack, pack, name))
	}
	return out
}
