package fetch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wpg/wpgctl/internal/util"
)

// 交付清单常给 zip 起 3.outwork、alarm.v4.8.1 这类名字；nginx location 只用短目录名。
var reHTMLVersion = regexp.MustCompile(`(?i)[._-]v?\d+(?:\.\d+){1,3}(?:[._-]patch[._-]?\d+)*$`)

// keepHTMLInner 静态资源子目录；若这一层没有 index.html、内层有，仍要掀开，否则 nginx 会 403。
var keepHTMLInner = map[string]struct{}{
	"static": {}, "assets": {}, "css": {}, "js": {}, "img": {},
	"images": {}, "fonts": {}, "lib": {}, "libs": {},
}

var htmlIndexNames = []string{"index.html", "index.htm"}

// htmlIndexWrappers 前端包常见的「套一层」目录名。解压时一律掀开，location 目录下直接是 index.html。
// front / frontend 是研发打包时把 dist 再包一层的习惯名；不掀开会留下 html/xxx/front/dist/index.html，nginx 403。
var htmlIndexWrappers = []string{"dist", "build", "www", "public", "front", "frontend"}

// NormalizeHTMLDirName 把前端 zip / 目录名收成 nginx 能对上的短名。
// 去掉 3. / 4. 这类序号前缀，以及 .v4.8.1、_v4.2.1、_patch.1 等版本后缀。
func NormalizeHTMLDirName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = trimSuffixFold(name, ".zip")
	orig := name
	name = stripHTMLSortPrefix(name)
	name = reHTMLVersion.ReplaceAllString(name, "")
	name = strings.Trim(name, "._-")
	if name == "" {
		return orig
	}
	return name
}

// stripHTMLSortPrefix 去掉 3.outwork、4.outworkApp 这类清单序号（后面必须是字母，避免碰到 2.0）。
func stripHTMLSortPrefix(name string) string {
	dot := strings.IndexByte(name, '.')
	if dot <= 0 || dot+1 >= len(name) {
		return name
	}
	for i := 0; i < dot; i++ {
		c := name[i]
		if c < '0' || c > '9' {
			return name
		}
	}
	rest := name[dot+1:]
	c := rest[0]
	if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' {
		return rest
	}
	return name
}

func isHTMLParent(dir string) bool {
	return strings.EqualFold(filepath.Base(dir), "html")
}

func htmlHasIndex(dir string) bool {
	for _, n := range htmlIndexNames {
		if util.FileExists(filepath.Join(dir, n)) {
			return true
		}
	}
	return false
}

func zipExtractDir(zipPath string, namer func(string) string) string {
	dir := filepath.Dir(zipPath)
	base := trimSuffixFold(filepath.Base(zipPath), ".zip")
	if namer == nil && isHTMLParent(dir) {
		namer = NormalizeHTMLDirName
	}
	if namer != nil {
		if n := strings.TrimSpace(namer(base)); n != "" {
			base = n
		}
	}
	return filepath.Join(dir, base)
}

// normalizeHTMLTree 整理 nginx/html：短目录名、掀开套层、dist→index、目录可被容器读取。
func normalizeHTMLTree(htmlDir string, logf func(string, ...any)) {
	if !isHTMLParent(htmlDir) {
		return
	}
	renameHTMLExtractDirs(htmlDir, logf)
	entries, err := os.ReadDir(htmlDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dest := filepath.Join(htmlDir, e.Name())
		_ = flattenHTMLWrapper(dest, logf)
		if !htmlHasIndex(dest) && logf != nil {
			logf("html/%s 没有 index.html，访问该路径会 403", e.Name())
		}
	}
	chmodHTMLReadable(htmlDir)
}

// renameHTMLExtractDirs 把 html 下已经解出的带版本/序号目录改成短名，便于对上 nginx。
func renameHTMLExtractDirs(htmlDir string, logf func(string, ...any)) {
	if !isHTMLParent(htmlDir) {
		return
	}
	entries, err := os.ReadDir(htmlDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		clean := NormalizeHTMLDirName(e.Name())
		if clean == "" || clean == e.Name() {
			continue
		}
		old := filepath.Join(htmlDir, e.Name())
		neu := filepath.Join(htmlDir, clean)
		if dirHasEntries(neu) {
			if logf != nil {
				logf("html 已有 %s/，保留 %s/", clean, e.Name())
			}
			continue
		}
		if err := os.Rename(old, neu); err != nil {
			if logf != nil {
				logf("重命名 %s → %s 失败: %v", e.Name(), clean, err)
			}
			continue
		}
		if logf != nil {
			logf("html 目录 %s → %s", e.Name(), clean)
		}
	}
}

// flattenHTMLWrapper 掀开版本目录或 dist/build，让 location 目录下直接有 index.html。
func flattenHTMLWrapper(dest string, logf func(string, ...any)) error {
	for i := 0; i < 4; i++ {
		if htmlHasIndex(dest) {
			return nil
		}
		changed, err := flattenOneHTMLWrapper(dest, logf)
		if err != nil {
			return err
		}
		if !changed {
			return promoteHTMLIndex(dest, logf)
		}
	}
	return promoteHTMLIndex(dest, logf)
}

func flattenOneHTMLWrapper(dest string, logf func(string, ...any)) (bool, error) {
	entries, err := os.ReadDir(dest)
	if err != nil {
		return false, err
	}
	var dirs []os.DirEntry
	var nFiles int
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			nFiles++
		}
	}
	if nFiles > 0 || len(dirs) != 1 {
		return false, nil
	}
	innerName := dirs[0].Name()
	inner := filepath.Join(dest, innerName)
	if _, keep := keepHTMLInner[strings.ToLower(innerName)]; keep && !htmlHasIndex(inner) {
		return false, nil
	}
	destBase := filepath.Base(dest)
	cleanedInner := NormalizeHTMLDirName(innerName)
	isWrapper := strings.EqualFold(innerName, destBase) || cleanedInner == destBase || cleanedInner != innerName
	for _, w := range htmlIndexWrappers {
		if strings.EqualFold(innerName, w) {
			isWrapper = true
			break
		}
	}
	if !isWrapper && !htmlHasIndex(inner) {
		return false, nil
	}
	if err := liftDirContents(inner, dest, innerName, logf); err != nil {
		return false, err
	}
	return true, nil
}

func promoteHTMLIndex(dest string, logf func(string, ...any)) error {
	if htmlHasIndex(dest) {
		return nil
	}
	for _, wrap := range htmlIndexWrappers {
		inner := filepath.Join(dest, wrap)
		if htmlHasIndex(inner) {
			return liftDirContents(inner, dest, wrap, logf)
		}
	}
	return nil
}

func liftDirContents(inner, dest, innerName string, logf func(string, ...any)) error {
	innerEntries, err := os.ReadDir(inner)
	if err != nil {
		return err
	}
	for _, e := range innerEntries {
		from := filepath.Join(inner, e.Name())
		to := filepath.Join(dest, e.Name())
		if _, err := os.Stat(to); err == nil {
			continue
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	_ = os.Remove(inner)
	if logf != nil {
		logf("html 掀开内层目录 %s/ → %s/", innerName, filepath.Base(dest))
	}
	return nil
}

func chmodHTMLReadable(root string) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			_ = os.Chmod(path, 0o755)
		} else {
			_ = os.Chmod(path, 0o644)
		}
		return nil
	})
}
