package ui

import (
	"path/filepath"
	"strings"
)

// FSPickHint 告诉不熟现场的人：当前这一层能不能选。
//
// Level：
//   - idle：还在盘符根，先往下走
//   - ready：标志齐了，点「就选这一层」
//   - deeper：差一层套包，点 Enter 里的文件夹
//   - up：进到模块里面了，点「上级」
//   - unknown：还对不上，按 Message 继续找
type FSPickHint struct {
	Level    string   `json:"level"`
	Title    string   `json:"title"`
	Headline string   `json:"headline"`
	Message  string   `json:"message"`
	Enter    []string `json:"enter,omitempty"` // 建议再点进去的文件夹名
	Marks    []string `json:"marks,omitempty"` // 已经到层的文件/文件夹名
}

func judgePickLevel(target, current string, entries []FSEntry) FSPickHint {
	title := pickTitle(target)
	if strings.TrimSpace(current) == "" {
		return FSPickHint{
			Level:    "idle",
			Title:    title,
			Headline: "从盘符往下点",
			Message:  "",
		}
	}
	files, dirs := splitPickEntries(entries)
	fileSet := pickNameSet(files)
	dirSet := pickNameSet(dirs)
	base := strings.ToLower(filepath.Base(current))

	switch target {
	case "package", "scanDir", "patchDir", "fetchLocal":
		return hintManifestRoot(title, fileSet, dirs)
	case "middlewareRoot":
		return hintModuleRoot(title,
			"mysql、redis、nacos 已并列",
			middlewareModuleNames(), []string{"middleware", "middle"}, 2,
			current, fileSet, dirSet)
	case "platformRoot":
		return hintModuleRoot(title,
			"public、device 等已并列",
			platformModuleNames(), []string{"platform"}, 2,
			current, fileSet, dirSet)
	case "waterworkDir":
		return hintWaterwork(title, base, fileSet, dirSet, dirs)
	case "intelligentModelDir":
		return hintComposeRoot(title,
			"有 docker-compose 和 .env",
			base, []string{"intelligent-model", "intelligent_model"}, fileSet, dirSet)
	case "nginxDir":
		return hintNginx(title, base, fileSet, dirSet)
	case "dockerPackage":
		return hintDockerPkg(title, fileSet, dirs)
	case "base":
		return hintBasePkg(title, fileSet, dirSet, dirs)
	case "pathsWorkspace":
		return hintWorkspace(title, base, dirSet)
	case "pathsNginxHtml":
		return hintNginxHTML(title, base, fileSet, dirSet)
	default:
		if title == "目录" {
			return FSPickHint{}
		}
		return FSPickHint{
			Level:    "unknown",
			Title:    title,
			Headline: "对照列表里的文件",
			Message:  "compose、.env 或 manifest.yaml 出现时再选",
		}
	}
}

func pickTitle(target string) string {
	switch target {
	case "package":
		return "Release 包目录"
	case "scanDir":
		return "扫描目录"
	case "patchDir":
		return "补丁目录"
	case "fetchLocal":
		return "本地包目录"
	case "middlewareRoot":
		return "中间件根目录"
	case "platformRoot":
		return "平台根目录"
	case "waterworkDir":
		return "市政水厂包目录"
	case "intelligentModelDir":
		return "模型服务包目录"
	case "nginxDir":
		return "Nginx 目录"
	case "dockerPackage":
		return "Docker 离线安装目录"
	case "base":
		return "Base 包目录"
	case "pathsWorkspace":
		return "工作簿根目录"
	case "pathsNginxHtml":
		return "Nginx html 目录"
	default:
		return "目录"
	}
}

func hintManifestRoot(title string, fileSet map[string]bool, dirs []FSEntry) FSPickHint {
	if fileSet["manifest.yaml"] {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "有 manifest.yaml",
			Marks:    []string{"manifest.yaml"},
		}
	}
	var enter []string
	for _, d := range dirs {
		if d.HasManifest {
			enter = append(enter, pickEntryName(d))
		}
	}
	if len(enter) > 0 {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点带 manifest 的文件夹",
			Enter:    enter,
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到包根",
		Message:  "找到直接含 manifest.yaml 的那一层",
	}
}

func hintModuleRoot(title, readyMsg string, modules, nests []string, min int, current string, fileSet, dirSet map[string]bool) FSPickHint {
	// 路径里已经出现模块名，说明点进了 mysql 或它的子目录，不能再说「还没到根」。
	if pathHasSegment(current, modules) {
		return FSPickHint{
			Level:    "up",
			Title:    title,
			Headline: "选深了，点「上级」",
			Message:  "回到能看到多个模块的那一层",
		}
	}
	hits := intersectFold(dirSet, modules)
	if len(hits) >= min {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  readyMsg,
			Marks:    hits,
		}
	}
	nestHits := intersectFold(dirSet, nests)
	if len(nestHits) > 0 {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点进「" + nestHits[0] + "」",
			Enter:    nestHits,
		}
	}
	// 已经在 middleware / platform 套层下面，再往下也不是根。
	if pathBelowSegment(current, nests) {
		return FSPickHint{
			Level:    "up",
			Title:    title,
			Headline: "选深了，点「上级」",
			Message:  "回到能看到多个模块的那一层",
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到根",
		Message:  "找到 " + strings.Join(sampleNames(modules, 3), "、") + " 并列处",
	}
}

func hintWaterwork(title, base string, fileSet, dirSet map[string]bool, dirs []FSEntry) FSPickHint {
	if waterworkLeaf(base) && hasComposeOrEnv(fileSet) {
		return FSPickHint{
			Level:    "up",
			Title:    title,
			Headline: "选深了，点「上级」",
			Message:  "回到能同时看到 center 和 device 的那一层",
		}
	}
	hits := intersectFold(dirSet, []string{"waterwork-center", "waterwork-device"})
	if len(hits) == 0 {
		for name := range dirSet {
			if waterworkLeaf(name) {
				hits = append(hits, name)
			}
		}
	}
	if len(hits) >= 1 {
		msg := "center / device 已出现"
		if len(hits) == 1 {
			msg = "已看到 " + hits[0]
		}
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  msg,
			Marks:    hits,
		}
	}
	var enter []string
	for _, d := range dirs {
		n := strings.ToLower(pickEntryName(d))
		if strings.Contains(n, "waterwork") && !waterworkLeaf(n) {
			enter = append(enter, pickEntryName(d))
		}
	}
	if len(enter) > 0 {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点进水厂包文件夹",
			Enter:    enter,
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到水厂包根",
		Message:  "找到 waterwork-center、waterwork-device",
	}
}

func hintComposeRoot(title, readyMsg, base string, nestNames []string, fileSet, dirSet map[string]bool) FSPickHint {
	if hasCompose(fileSet) {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  readyMsg,
			Marks:    composeMarks(fileSet),
		}
	}
	nests := intersectFold(dirSet, nestNames)
	if len(nests) > 0 {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点进「" + nests[0] + "」",
			Enter:    nests,
		}
	}
	_ = base
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到包根",
		Message:  "找到带 docker-compose 的那一层",
	}
}

func hintNginx(title, base string, fileSet, dirSet map[string]bool) FSPickHint {
	if base == "html" && !hasCompose(fileSet) {
		return FSPickHint{
			Level:    "up",
			Title:    title,
			Headline: "选深了，点「上级」",
			Message:  "Nginx 在上一层",
		}
	}
	if hasCompose(fileSet) && (base == "nginx" || dirSet["html"] || dirSet["conf"] || fileSet["nginx.conf"]) {
		marks := composeMarks(fileSet)
		if dirSet["html"] {
			marks = append(marks, "html")
		}
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "有 docker-compose",
			Marks:    marks,
		}
	}
	if dirSet["nginx"] {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点进 nginx 文件夹。",
			Enter:    []string{"nginx"},
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到 Nginx 目录",
		Message:  "找到带 docker-compose 的 nginx",
	}
}

func hintDockerPkg(title string, fileSet map[string]bool, dirs []FSEntry) FSPickHint {
	if fileSet["offline_install_docker.sh"] || fileSet["docker.service"] {
		marks := []string{}
		if fileSet["offline_install_docker.sh"] {
			marks = append(marks, "offline_install_docker.sh")
		}
		if fileSet["docker.service"] {
			marks = append(marks, "docker.service")
		}
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "有安装脚本或 docker.service",
			Marks:    marks,
		}
	}
	var enter []string
	for _, d := range dirs {
		if d.HasDockerInstall {
			enter = append(enter, pickEntryName(d))
		}
	}
	if len(enter) > 0 {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点带 [docker] 的文件夹。",
			Enter:    enter,
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到安装包",
		Message:  "找到 offline_install_docker.sh",
	}
}

func hintBasePkg(title string, fileSet, dirSet map[string]bool, dirs []FSEntry) FSPickHint {
	if dirSet["docker-install"] || fileSet["offline_install_docker.sh"] {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "含 docker-install",
			Marks:    []string{"docker-install"},
		}
	}
	hits := intersectFold(dirSet, middlewareModuleNames())
	if len(hits) >= 2 {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "可当 base 用",
			Marks:    hits,
		}
	}
	if dirSet["docker_package"] {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点进 docker_package",
			Enter:    []string{"docker_package"},
		}
	}
	_ = dirs
	_ = fileSet
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到 base",
		Message:  "找 docker-install，或 mysql、redis 并列的那一层",
	}
}

func hintWorkspace(title, base string, dirSet map[string]bool) FSPickHint {
	if base == "workspace" {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "当前就是工作簿根",
			Marks:    []string{"workspace"},
		}
	}
	if dirSet["platform"] && (dirSet["middleware"] || dirSet["middle"] || dirSet["docker_data"]) {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "已看到 platform / 中间件",
			Marks:    intersectFold(dirSet, []string{"platform", "middleware", "middle", "docker_data"}),
		}
	}
	if dirSet["workspace"] {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点进 workspace，或停在这一层",
			Enter:    []string{"workspace"},
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "找 workspace",
		Message:  "选它或它的上一层都可以",
	}
}

func hintNginxHTML(title, base string, fileSet, dirSet map[string]bool) FSPickHint {
	if base == "html" {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "当前是 html 静态目录。",
		}
	}
	if dirSet["html"] {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点进 html。",
			Enter:    []string{"html"},
		}
	}
	_ = fileSet
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "找 html 文件夹",
		Message:  "一般在 nginx 模块下面，名为 html。",
	}
}

func middlewareModuleNames() []string {
	return []string{
		"mysql", "pgsql", "postgis", "mongodb",
		"redis", "kafka", "nacos", "minio", "influxdb", "emqx",
		"waterjob", "water-job", "water-job-biz",
	}
}

func platformModuleNames() []string {
	return []string{"public", "device", "alarm", "graph", "gis", "monitor", "report-center", "out-work"}
}

func waterworkLeaf(name string) bool {
	n := strings.ToLower(name)
	return n == "waterwork-center" || strings.HasPrefix(n, "waterwork-center-") ||
		n == "waterwork-device" || strings.HasPrefix(n, "waterwork-device-")
}

func hasCompose(fileSet map[string]bool) bool {
	return fileSet["docker-compose.yml"] || fileSet["docker-compose.yaml"] || fileSet["compose.yml"]
}

func hasComposeOrEnv(fileSet map[string]bool) bool {
	return hasCompose(fileSet) || fileSet[".env"]
}

func composeMarks(fileSet map[string]bool) []string {
	var out []string
	for _, n := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", ".env"} {
		if fileSet[n] {
			out = append(out, n)
		}
	}
	return out
}

func splitPickEntries(entries []FSEntry) (files, dirs []FSEntry) {
	for _, e := range entries {
		if e.IsDir {
			dirs = append(dirs, e)
		} else if !e.IsArchive {
			files = append(files, e)
		}
	}
	return files, dirs
}

func pickEntryName(e FSEntry) string {
	if e.Path != "" {
		return filepath.Base(e.Path)
	}
	return strings.TrimSpace(strings.Split(e.Name, "  [")[0])
}

func pickNameSet(entries []FSEntry) map[string]bool {
	out := map[string]bool{}
	for _, e := range entries {
		out[strings.ToLower(pickEntryName(e))] = true
	}
	return out
}

func intersectFold(have map[string]bool, names []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, n := range names {
		k := strings.ToLower(n)
		if have[k] && !seen[k] {
			seen[k] = true
			out = append(out, n)
		}
	}
	return out
}

// pathHasSegment 当前路径的任一段（含当前目录名）是否命中 names。
func pathHasSegment(current string, names []string) bool {
	for _, seg := range pathSegments(current) {
		if containsFold(names, seg) {
			return true
		}
	}
	return false
}

// pathBelowSegment 当前目录已经位于 names 中某个目录的里面（当前名本身不算）。
func pathBelowSegment(current string, names []string) bool {
	segs := pathSegments(current)
	if len(segs) < 2 {
		return false
	}
	for _, seg := range segs[:len(segs)-1] {
		if containsFold(names, seg) {
			return true
		}
	}
	return false
}

func pathSegments(current string) []string {
	var out []string
	for _, seg := range strings.Split(filepath.ToSlash(current), "/") {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

func containsFold(names []string, want string) bool {
	want = strings.ToLower(want)
	for _, n := range names {
		if strings.ToLower(n) == want {
			return true
		}
	}
	return false
}

func sampleNames(names []string, n int) []string {
	if len(names) <= n {
		return names
	}
	return names[:n]
}
