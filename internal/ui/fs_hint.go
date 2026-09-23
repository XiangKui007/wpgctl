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
	Marks    []string `json:"marks,omitempty"` // 作为到层依据的文件/文件夹
}

func judgePickLevel(target, current string, entries []FSEntry) FSPickHint {
	title := pickTitle(target)
	if strings.TrimSpace(current) == "" {
		return FSPickHint{
			Level:    "idle",
			Title:    title,
			Headline: "从盘符开始往下进",
			Message:  "一层一层点文件夹。到了标志齐全的那一层，再点「选择当前目录」，不要凭目录名猜。",
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
			"看到 mysql、redis、nacos 等并列的文件夹就停，点「就选这一层」。不要点进 mysql 里面。",
			middlewareModuleNames(), []string{"middleware", "middle"}, 2,
			base, fileSet, dirSet)
	case "platformRoot":
		return hintModuleRoot(title,
			"看到 public、device、gis 等并列的文件夹就停。不要点进 public 里面。",
			platformModuleNames(), []string{"platform"}, 2,
			base, fileSet, dirSet)
	case "waterworkDir":
		return hintWaterwork(title, base, fileSet, dirSet, dirs)
	case "intelligentModelDir":
		return hintComposeRoot(title,
			"看到 docker-compose 和 .env 就停，这是模型包根。",
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
			Headline: "按下面标志找",
			Message:  "对照列表里的文件判断是否到层：compose、.env、manifest.yaml 通常出现在该选的那一层。",
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
			Message:  "当前目录有 manifest.yaml，这就是包根。点「就选这一层」。",
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
			Message:  "点带 [manifest] 的文件夹，那一层才是包根。",
			Enter:    enter,
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到包根",
		Message:  "继续往下找，直到当前层直接出现 manifest.yaml。",
	}
}

func hintModuleRoot(title, readyMsg string, modules, nests []string, min int, base string, fileSet, dirSet map[string]bool) FSPickHint {
	if containsFold(modules, base) && hasComposeOrEnv(fileSet) {
		return FSPickHint{
			Level:    "up",
			Title:    title,
			Headline: "选深了，点「上级」",
			Message:  "现在在单个模块里面。要选能同时看到多个模块文件夹的那一层。",
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
			Message:  "现场 zip 常解出同名套层。点进「" + nestHits[0] + "」，里面才是模块并列的那一层。",
			Enter:    nestHits,
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到根",
		Message:  "继续往下找。到层标志：多个模块文件夹排在一起（例如 " + strings.Join(sampleNames(modules, 3), "、") + "）。",
	}
}

func hintWaterwork(title, base string, fileSet, dirSet map[string]bool, dirs []FSEntry) FSPickHint {
	if waterworkLeaf(base) && hasComposeOrEnv(fileSet) {
		return FSPickHint{
			Level:    "up",
			Title:    title,
			Headline: "选深了，点「上级」",
			Message:  "现在在 center 或 device 里面。包根是能同时看到 waterwork-center 和 waterwork-device 的那一层。",
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
		msg := "看到 waterwork-center / waterwork-device 就停。两套都在更好，缺一套也能先选。"
		if len(hits) == 1 {
			msg = "已经看到 " + hits[0] + "。通常同一层还有另一套；没有的话也可以先选这一层。"
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
			Message:  "点进水厂包文件夹，里面应能看到 waterwork-center 和 waterwork-device。",
			Enter:    enter,
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到水厂包根",
		Message:  "继续往下找，直到当前层同时出现 waterwork-center、waterwork-device。不要选 platform 下面的目录。",
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
			Message:  "点进「" + nests[0] + "」，找到带 docker-compose 的那一层。",
			Enter:    nests,
		}
	}
	_ = base
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到包根",
		Message:  "继续往下找，直到当前层出现 docker-compose.yml / .env。",
	}
}

func hintNginx(title, base string, fileSet, dirSet map[string]bool) FSPickHint {
	if base == "html" && !hasCompose(fileSet) {
		return FSPickHint{
			Level:    "up",
			Title:    title,
			Headline: "选深了，点「上级」",
			Message:  "html 是前端静态目录。Nginx 模块是上一层（有 docker-compose 的那层）。",
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
			Message:  "有 docker-compose，这就是 Nginx 模块目录。点「就选这一层」。",
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
		Message:  "继续往下找名为 nginx、且带 docker-compose 的文件夹。",
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
			Message:  "看到离线安装脚本或 docker.service 就停。",
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
		Message:  "继续往下找 docker_package，直到出现 offline_install_docker.sh。",
	}
}

func hintBasePkg(title string, fileSet, dirSet map[string]bool, dirs []FSEntry) FSPickHint {
	if dirSet["docker-install"] || fileSet["offline_install_docker.sh"] {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "标准 base 含 docker-install；也可以选中间件根目录。",
			Marks:    []string{"docker-install"},
		}
	}
	hits := intersectFold(dirSet, middlewareModuleNames())
	if len(hits) >= 2 {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "当前像中间件根，可以当 base 用。",
			Marks:    hits,
		}
	}
	if dirSet["docker_package"] {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点进 docker_package，或回到含 docker-install 的 base 包。",
			Enter:    []string{"docker_package"},
		}
	}
	_ = dirs
	_ = fileSet
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "还没到 base",
		Message:  "标准 base 含 docker-install/；也可以选中间件根（mysql、redis 并列的那层）。",
	}
}

func hintWorkspace(title, base string, dirSet map[string]bool) FSPickHint {
	if base == "workspace" {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "当前就是工作簿根。下面通常会有 platform、middleware。",
			Marks:    []string{"workspace"},
		}
	}
	if dirSet["platform"] && (dirSet["middleware"] || dirSet["middle"] || dirSet["docker_data"]) {
		return FSPickHint{
			Level:    "ready",
			Title:    title,
			Headline: "可以选这一层",
			Message:  "已经能看到 platform / 中间件目录，可以当工作簿根。",
			Marks:    intersectFold(dirSet, []string{"platform", "middleware", "middle", "docker_data"}),
		}
	}
	if dirSet["workspace"] {
		return FSPickHint{
			Level:    "deeper",
			Title:    title,
			Headline: "再往下进一层",
			Message:  "点进 workspace。也可以停在这一层，工具会自动接上 workspace。",
			Enter:    []string{"workspace"},
		}
	}
	return FSPickHint{
		Level:    "unknown",
		Title:    title,
		Headline: "找名为 workspace 的目录",
		Message:  "选工作簿根（常见名 workspace）。选它的上一层也可以，工具会补上 /workspace。",
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
