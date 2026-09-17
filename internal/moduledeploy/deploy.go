package moduledeploy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/fetch"
	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 单模块目录部署（解压 → load → 改 env → compose up）。
type Options struct {
	ModuleDir string
	Site      *config.SiteConfig
	Expand    bool
	Load      bool
	PatchEnv  bool
	ComposeUp bool
	ComposeBuild bool // 市政/模型等：compose up -d --build，从 jar 目录构建，跳过 load tar
	ComposeFlags []string // 如 mongodb --compatibility
	LoadedRefs   []string // 已 load 的镜像（nginx 等分步部署时传入 compose sync）
}

// Result 执行摘要。
type Result struct {
	ModuleDir   string   `json:"moduleDir"`
	Steps       []string `json:"steps"`
	TarsLoaded  []string `json:"tarsLoaded"`
	LoadedRefs  []string `json:"loadedRefs,omitempty"`
	EnvPatched  []string `json:"envPatched"`
	ComposeFile string   `json:"composeFile,omitempty"`
}

// Run 在单个模块目录执行现场标准动作。
func Run(opts Options) (*Result, error) {
	dir, err := filepath.Abs(strings.TrimSpace(opts.ModuleDir))
	if err != nil {
		return nil, err
	}
	if !util.DirExists(dir) {
		return nil, fmt.Errorf("模块目录不存在: %s", dir)
	}
	res := &Result{ModuleDir: dir}

	if opts.Expand {
		steps, err := PrepareModuleArchives(dir)
		res.Steps = append(res.Steps, steps...)
		if err != nil {
			return res, fmt.Errorf("解压失败: %w", err)
		}
	} else if opts.Load {
		tars, _ := findImageTars(dir)
		pending, _ := findPendingTarZips(dir)
		if len(tars) > 0 && len(pending) == 0 {
			res.Steps = append(res.Steps, "已有 .tar，跳过解压")
		}
	}

	loadedRefs := append([]string(nil), opts.LoadedRefs...)
	if opts.Load {
		tars, err := findImageTars(dir)
		if err != nil {
			return res, err
		}
		if len(tars) == 0 {
			return res, fmt.Errorf("未发现可 load 的 .tar（请先解压 tar.zip）；compose 将尝试从私服 pull 并可能失败")
		}
		d := dockerx.New()
		for _, tar := range tars {
			refs, err := d.LoadImage(tar)
			if err != nil {
				return res, err
			}
			loadedRefs = append(loadedRefs, refs...)
			res.TarsLoaded = append(res.TarsLoaded, filepath.Base(tar))
			step := "docker load: " + filepath.Base(tar)
			if len(refs) > 0 {
				step += "（docker images 新增: " + strings.Join(refs, ", ") + "）"
			}
			res.Steps = append(res.Steps, step)
		}
	}
	res.LoadedRefs = loadedRefs

	if opts.PatchEnv && opts.Site != nil {
		envPath := filepath.Join(dir, ".env")
		if util.FileExists(envPath) {
			changed, err := EnvPatchHosts(envPath, opts.Site)
			if err != nil {
				return res, err
			}
			res.EnvPatched = changed
			if len(changed) > 0 {
				res.Steps = append(res.Steps, fmt.Sprintf("已更新 .env: %v", changed))
			}
		}
		// gis 等含子服务目录
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || info.Name() != ".env" || path == envPath {
				return nil
			}
			changed, e := EnvPatchHosts(path, opts.Site)
			if e == nil && len(changed) > 0 {
				res.EnvPatched = append(res.EnvPatched, changed...)
				res.Steps = append(res.Steps, "已更新 "+path)
			}
			return nil
		})
		composeChanged, err := PatchComposeEnvForSite(dir, opts.Site)
		if err != nil {
			return res, err
		}
		if len(composeChanged) > 0 {
			res.EnvPatched = append(res.EnvPatched, composeChanged...)
			res.Steps = append(res.Steps, fmt.Sprintf("已更新 compose environment: %v", composeChanged))
		}
	}

	if opts.ComposeUp {
		compose, err := findComposeFile(dir)
		if err != nil {
			return res, err
		}
		res.ComposeFile = compose
		d := dockerx.New()
		composeDir := dir
		composeFile := compose
		if filepath.Dir(compose) != dir {
			composeDir = filepath.Dir(compose)
			composeFile = filepath.Base(compose)
		}
		if !opts.ComposeBuild {
			if notes, err := SyncComposeImagesFromDocker(compose, d, loadedRefs); err != nil {
				return res, fmt.Errorf("同步 compose 镜像版本失败: %w", err)
			} else if len(notes) > 0 {
				for _, n := range notes {
					res.Steps = append(res.Steps, "compose 镜像已同步: "+n)
				}
			}
			images, _ := imagesFromCompose(compose)
			if missing := d.MissingImages(images); len(missing) > 0 {
				return res, fmt.Errorf("本地缺少镜像（需先 docker load）：%s", strings.Join(missing, ", "))
			}
		}
		if err := d.ComposeUp(composeDir, composeFile, nil, opts.ComposeBuild); err != nil {
			return res, err
		}
		if opts.ComposeBuild {
			res.Steps = append(res.Steps, "docker compose up -d --build 完成")
		} else {
			res.Steps = append(res.Steps, "docker compose up -d 完成")
		}
	}

	return res, nil
}

func imagesFromCompose(composePath string) ([]string, error) {
	f, err := os.Open(composePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var images []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "image:") {
			continue
		}
		img := strings.TrimSpace(strings.TrimPrefix(line, "image:"))
		img = strings.Trim(img, `"'`)
		if img != "" && !strings.Contains(img, "${") {
			images = append(images, img)
		}
	}
	return images, sc.Err()
}

func findComposeFile(dir string) (string, error) {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml"} {
		p := filepath.Join(dir, name)
		if util.FileExists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 docker-compose.yml: %s", dir)
}

// PrepareModuleArchives 展开目录内 .zip / .tar.zip，直至可 docker load 的 .tar 就绪。
// 若目录内已有 .tar 且无待展开 tar.zip，则跳过解压。
func PrepareModuleArchives(dir string) ([]string, error) {
	tars, _ := findImageTars(dir)
	pending, _ := findPendingTarZips(dir)
	if len(tars) > 0 && len(pending) == 0 {
		return []string{"已有 .tar，跳过解压"}, nil
	}
	var steps []string
	var lastErr error
	for pass := 0; pass < 3; pass++ {
		er, err := fetch.ExpandArchives(dir)
		if er != nil {
			steps = append(steps, er.Steps...)
			if er.ZipExtracted > 0 || er.TarUnwrapped > 0 {
				steps = append(steps, fmt.Sprintf("解压: zip=%d tar.zip=%d", er.ZipExtracted, er.TarUnwrapped))
			}
		}
		lastErr = err
		tars, _ = findImageTars(dir)
		pending, _ = findPendingTarZips(dir)
		if len(pending) == 0 {
			if len(tars) > 0 {
				return steps, nil
			}
			if err == nil {
				return steps, nil
			}
			return steps, lastErr
		}
		if er != nil && (er.ZipExtracted > 0 || er.TarUnwrapped > 0) {
			continue
		}
	}
	pending, _ = findPendingTarZips(dir)
	if len(pending) > 0 {
		names := make([]string, 0, len(pending))
		for _, p := range pending {
			names = append(names, filepath.Base(p))
		}
		return steps, fmt.Errorf("仍有 tar.zip 未展开: %s", strings.Join(names, ", "))
	}
	tars, _ = findImageTars(dir)
	if len(tars) == 0 && lastErr != nil {
		return steps, lastErr
	}
	return steps, nil
}

func findTarZipFiles(dir string) ([]string, error) {
	var zips []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			if info != nil && info.IsDir() {
				lower := strings.ToLower(info.Name())
				if lower == "data" || lower == "logs" || lower == "log" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".tar.zip") {
			zips = append(zips, path)
		}
		return nil
	})
	return zips, err
}

func findPendingTarZips(dir string) ([]string, error) {
	all, err := findTarZipFiles(dir)
	if err != nil {
		return nil, err
	}
	var pending []string
	for _, z := range all {
		dest := strings.TrimSuffix(z, ".zip")
		if !util.FileExists(dest) {
			pending = append(pending, z)
		}
	}
	return pending, nil
}

func findImageTars(dir string) ([]string, error) {
	var tars []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				lower := strings.ToLower(info.Name())
				if lower == "data" || lower == "logs" || lower == "log" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		lower := strings.ToLower(info.Name())
		if strings.HasSuffix(lower, ".tar") && !strings.HasSuffix(lower, ".tar.zip") {
			tars = append(tars, path)
		}
		return nil
	})
	return tars, err
}

// ModulePath 拼接包根 + 模块名，兼容 platform/platform 嵌套。
func ModulePath(root, name string) string {
	if root == "" {
		return ""
	}
	p := filepath.Join(root, name)
	if util.DirExists(p) {
		return p
	}
	nested := filepath.Join(root, filepath.Base(root), name)
	if util.DirExists(nested) {
		return nested
	}
	return p
}

// ListInitSQL 列出模块内 init SQL（供步骤 2 提示）。
func ListInitSQL(dir string) []string {
	var out []string
	for _, sub := range []string{"init", "sql"} {
		d := filepath.Join(dir, sub)
		if !util.DirExists(d) {
			continue
		}
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			lower := strings.ToLower(e.Name())
			if strings.HasSuffix(lower, ".sql") {
				out = append(out, filepath.Join(d, e.Name()))
			}
		}
	}
	return out
}
