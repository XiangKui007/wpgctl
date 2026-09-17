// Package pack：从交付目录扫描镜像 tar，自动生成 manifest.yaml。
package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"gopkg.in/yaml.v3"
)

// ScanOptions 扫描选项。
type ScanOptions struct {
	Dir     string
	Kind    string // base|release，默认 base（中间件包）或 release
	Version string // manifest 包版本；空则尝试从目录名推断
	Arch    string // 默认 amd64
}

// ScannedImage 扫描到的镜像条目。
type ScannedImage struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Version  string `json:"version"`
	Layer    int    `json:"layer"`
	Port     int    `json:"port,omitempty"`
	Profile  string `json:"profile,omitempty"`
	Health   string `json:"health"` // tcp|http|none
	TarPath  string `json:"tarPath"`
	ParentDir string `json:"parentDir,omitempty"`
}

// ScanResult 扫描结果。
type ScanResult struct {
	Images   []ScannedImage `json:"images"`
	Manifest *config.Manifest `json:"-"`
	YAML     string         `json:"yaml"`
	Warnings []string       `json:"warnings,omitempty"`
}

type serviceHint struct {
	Layer   int
	Port    int
	Profile string
	Health  string // tcp|http|none
	Image   string // 覆盖镜像仓库名，空则用 Name
}

// 已知组件默认分层 / 端口（与附录 A 中间件对齐）。
var knownHints = map[string]serviceHint{
	"mysql":          {Layer: 1, Port: 3306, Health: "tcp"},
	"pgsql":          {Layer: 1, Port: 5433, Health: "tcp"},
	"postgresql":     {Layer: 1, Port: 5433, Health: "tcp"},
	"postgis":        {Layer: 1, Port: 5432, Health: "tcp", Profile: "gis"},
	"mongodb":        {Layer: 1, Port: 27017, Health: "tcp", Profile: "gis"},
	"redis":          {Layer: 1, Port: 6377, Health: "tcp"},
	"kafka":          {Layer: 1, Port: 9092, Health: "tcp"},
	"zookeeper":      {Layer: 1, Port: 2181, Health: "tcp"},
	"nacos":          {Layer: 1, Port: 8848, Health: "http"},
	"nacos-server":   {Layer: 1, Port: 8848, Health: "http", Image: "nacos/nacos-server"},
	"minio":          {Layer: 1, Port: 9000, Health: "tcp"},
	"influxdb":       {Layer: 1, Port: 8086, Health: "tcp"},
	"emqx":           {Layer: 1, Port: 1883, Health: "tcp"},
	"nginx":          {Layer: 4, Port: 8877, Health: "http"},
	"kafdrop":        {Layer: 2, Port: 9001, Health: "http"},
	"waterjob":       {Layer: 2, Port: 8088, Health: "http"},
	"water-job-biz":  {Layer: 2, Port: 8088, Health: "http", Image: "water-job-biz"},
	"prometheus":     {Layer: 4, Port: 29090, Health: "http", Profile: "monitor"},
	"grafana":        {Layer: 4, Port: 3000, Health: "http", Profile: "monitor"},
	"node-exporter":  {Layer: 4, Port: 9100, Health: "tcp", Profile: "monitor"},
}

var (
	reExt = regexp.MustCompile(`(?i)\.(tar\.zip|tar\.gz|tgz|tar)$`)
	// name-version / name_version，version 以数字或 v+数字开头
	reDashVer = regexp.MustCompile(`(?i)^([a-z][a-z0-9._-]*)[-_](v?\d[\w.-]*)$`)
	// name.version
	reDotVer = regexp.MustCompile(`(?i)^([a-z][a-z0-9_-]*)\.(v?\d[\w.-]*)$`)
	// mysql5.7.44 / redis6.0
	reGlued = regexp.MustCompile(`(?i)^([a-z]+)(\d[\w.-]*)$`)
	reVerLike = regexp.MustCompile(`(?i)^v?\d`)
)

// ScanDir 扫描目录内镜像 tar，生成 manifest YAML。
func ScanDir(opts ScanOptions) (*ScanResult, error) {
	dir := strings.TrimSpace(opts.Dir)
	if dir == "" {
		return nil, fmt.Errorf("目录不能为空")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("目录不存在: %s", abs)
	}

	kind := opts.Kind
	if kind == "" {
		kind = "base"
	}
	if kind != "base" && kind != "release" && kind != "patch" {
		return nil, fmt.Errorf("kind 必须是 base|release|patch")
	}
	arch := opts.Arch
	if arch == "" {
		arch = "amd64"
	}
	version := opts.Version
	if version == "" {
		version = inferPackageVersion(abs)
	}
	if version == "" {
		version = "1.0.0"
	}

	var images []ScannedImage
	var warnings []string
	seen := map[string]string{} // name -> tar path

	err = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			base := strings.ToLower(info.Name())
			if base == "data" || base == "logs" || base == "log" || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		lower := strings.ToLower(name)
		if !reExt.MatchString(lower) {
			return nil
		}
		// 跳过整包分卷 / 前端静态包常见名
		if strings.Contains(lower, ".tar.gz.a") || strings.HasSuffix(lower, ".part") {
			return nil
		}
		if strings.Contains(lower, "dist-") || strings.Contains(filepath.ToSlash(path), "/frontend/") {
			if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
				return nil
			}
		}

		stem := reExt.ReplaceAllString(name, "")
		parent := filepath.Base(filepath.Dir(path))
		svc, ver := parseTarStem(stem, parent)
		if svc == "" {
			warnings = append(warnings, "无法解析: "+path)
			return nil
		}
		hint := lookupHint(svc)
		imgName := svc
		if hint.Image != "" {
			imgName = hint.Image
		}
		tag := ver
		if tag == "" {
			tag = "latest"
			warnings = append(warnings, fmt.Sprintf("%s 未解析到版本，使用 :latest（来自 %s）", svc, name))
		}
		layer := hint.Layer
		if layer == 0 {
			layer = 3
		}
		health := hint.Health
		if health == "" {
			health = "tcp"
		}
		item := ScannedImage{
			Name:      normalizeServiceName(svc),
			Image:     fmt.Sprintf("%s:%s", imgName, tag),
			Version:   tag,
			Layer:     layer,
			Port:      hint.Port,
			Profile:   hint.Profile,
			Health:    health,
			TarPath:   path,
			ParentDir: parent,
		}
		key := item.Name
		if prev, ok := seen[key]; ok {
			warnings = append(warnings, fmt.Sprintf("重复服务 %s：保留 %s，忽略 %s", key, prev, path))
			return nil
		}
		seen[key] = path
		images = append(images, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("未在目录中找到可解析的镜像 tar: %s", abs)
	}

	sort.Slice(images, func(i, j int) bool {
		if images[i].Layer != images[j].Layer {
			return images[i].Layer < images[j].Layer
		}
		return images[i].Name < images[j].Name
	})

	mf := &config.Manifest{
		Kind:    kind,
		Version: version,
		Arch:    []string{arch},
	}
	if kind == "release" {
		mf.RequiresBase = ">=1.0.0"
	}
	for _, img := range images {
		hs := config.HealthSpec{Type: img.Health, TimeoutSec: 120, IntervalSec: 5}
		if img.Health == "http" {
			switch img.Name {
			case "nacos", "nacos-server":
				hs.Path = "/nacos/"
				hs.TimeoutSec = 180
			case "nginx":
				hs.Path = "/"
			case "prometheus":
				hs.Path = "/-/healthy"
			default:
				hs.Path = "/actuator/health"
			}
		}
		mf.Services = append(mf.Services, config.ServiceSpec{
			Name:    img.Name,
			Image:   img.Image,
			Profile: img.Profile,
			Layer:   img.Layer,
			Port:    img.Port,
			Health:  hs,
			RequiredFiles: []string{relTar(abs, img.TarPath)},
		})
	}

	if err := mf.Validate(); err != nil {
		warnings = append(warnings, "manifest 校验提示: "+err.Error())
	}

	raw, err := yaml.Marshal(mf)
	if err != nil {
		return nil, err
	}
	header := fmt.Sprintf("# generated by wpgctl pack scan\n# source: %s\n# 请人工核对 image tag / port / profile / layer\n", abs)
	return &ScanResult{
		Images:   images,
		Manifest: mf,
		YAML:     header + string(raw),
		Warnings: warnings,
	}, nil
}

// WriteManifest 将扫描结果写入路径。
func WriteManifest(path string, yamlText string) error {
	if err := utilEnsureParent(path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(yamlText), 0o644)
}

func utilEnsureParent(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func relTar(root, tarPath string) string {
	rel, err := filepath.Rel(root, tarPath)
	if err != nil {
		return filepath.Base(tarPath)
	}
	return filepath.ToSlash(rel)
}

func lookupHint(name string) serviceHint {
	n := strings.ToLower(name)
	if h, ok := knownHints[n]; ok {
		return h
	}
	// nacos-server → try nacos
	if strings.HasPrefix(n, "nacos") {
		return knownHints["nacos-server"]
	}
	if strings.Contains(n, "water") && strings.Contains(n, "job") {
		return knownHints["water-job-biz"]
	}
	return serviceHint{}
}

func normalizeServiceName(name string) string {
	n := strings.ToLower(name)
	switch n {
	case "nacos-server":
		return "nacos"
	case "postgresql":
		return "pgsql"
	case "water-job-biz":
		return "waterjob"
	default:
		return n
	}
}

func inferPackageVersion(dir string) string {
	base := filepath.Base(dir)
	// wpg-release-4.0.2 / middleware-1.0.0
	re := regexp.MustCompile(`(?i)(\d+\.\d+\.\d+(?:[._-][\w.-]*)?)$`)
	if m := re.FindStringSubmatch(base); len(m) > 1 {
		return m[1]
	}
	return ""
}

// parseTarStem 从去扩展名的文件名 + 父目录解析服务名与版本。
func parseTarStem(stem, parent string) (name, version string) {
	stem = strings.TrimSpace(stem)
	if stem == "" {
		return "", ""
	}

	// 1) 已知前缀优先（mysql5.7.44 / nacos-server.v2...）
	if n, v := matchKnownPrefix(stem); n != "" {
		return n, v
	}

	// 2) name-version / name_version
	if m := reDashVer.FindStringSubmatch(stem); len(m) == 3 {
		return m[1], m[2]
	}

	// 3) name.version（避免把 mysql5.7 拆成 mysql5 + 7）
	if m := reDotVer.FindStringSubmatch(stem); len(m) == 3 {
		left := m[1]
		if left[len(left)-1] < '0' || left[len(left)-1] > '9' {
			return left, m[2]
		}
	}

	// 4) 通用粘连 name+version
	if m := reGlued.FindStringSubmatch(stem); len(m) == 3 {
		return m[1], m[2]
	}

	// 5) 父目录是服务名
	parentLower := strings.ToLower(parent)
	if parentLower != "" && parentLower != "." && parentLower != "images" && parentLower != "middleware" {
		if _, ok := knownHints[parentLower]; ok || looksServiceName(parentLower) {
			if reVerLike.MatchString(stem) {
				return parentLower, stem
			}
			if strings.HasPrefix(strings.ToLower(stem), parentLower) {
				rest := stem[len(parentLower):]
				rest = strings.TrimLeft(rest, "._-")
				if rest == "" {
					return parentLower, ""
				}
				if reVerLike.MatchString(rest) {
					return parentLower, rest
				}
			}
			return parentLower, extractTrailingVersion(stem)
		}
	}

	// 6) 无版本纯名
	if looksServiceName(stem) {
		return strings.ToLower(stem), ""
	}
	return strings.ToLower(stem), extractTrailingVersion(stem)
}

func matchKnownPrefix(stem string) (name, version string) {
	lower := strings.ToLower(stem)
	keys := make([]string, 0, len(knownHints))
	for k := range knownHints {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		if lower == k {
			return k, ""
		}
		if strings.HasPrefix(lower, k) {
			rest := lower[len(k):]
			rest = strings.TrimLeft(rest, "._-")
			if rest == "" {
				return k, ""
			}
			if reVerLike.MatchString(rest) {
				return k, rest
			}
		}
	}
	return "", ""
}

func extractTrailingVersion(stem string) string {
	re := regexp.MustCompile(`(?i)(?:^|[-_.])(v?\d[\w.-]*)$`)
	if m := re.FindStringSubmatch(stem); len(m) == 2 {
		return m[1]
	}
	return ""
}

func looksServiceName(s string) bool {
	if s == "" || reVerLike.MatchString(s) {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
