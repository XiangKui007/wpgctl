package moduledeploy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/fetch"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 单模块目录部署（解压 → load → 改 env → compose up）。
type Options struct {
	ModuleDir    string
	Site         *config.SiteConfig
	Expand       bool
	Load         bool
	PatchEnv     bool
	ComposeUp    bool
	ComposeBuild bool     // 市政/模型等：compose up -d --build；构建前仍 load java8.tar 等基础镜像
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

	// Kafka 等：无论是否 patch .env，都按 site.yaml 改 compose environment（库/中间件常无 .env）
	if opts.Site != nil {
		composeChanged, err := PatchComposeEnvForSite(dir, opts.Site)
		if err != nil {
			return res, err
		}
		if len(composeChanged) > 0 {
			res.EnvPatched = append(res.EnvPatched, composeChanged...)
			res.Steps = append(res.Steps, fmt.Sprintf("已更新 compose environment: %v", composeChanged))
		}
	}

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
		// gis / 市政水厂等含子服务目录
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
		if projects, perr := findComposeProjects(dir); perr == nil {
			repl := hostReplacements(opts.Site)
			for _, compose := range projects {
				changed, e := PatchComposeEnv(compose, repl)
				if e == nil && len(changed) > 0 {
					res.EnvPatched = append(res.EnvPatched, changed...)
					res.Steps = append(res.Steps, fmt.Sprintf("已更新 compose environment（%s）: %v", filepath.Base(filepath.Dir(compose)), changed))
				}
			}
		}
	}

	if opts.ComposeUp {
		projects, err := findComposeProjects(dir)
		if err != nil && opts.ComposeBuild {
			er, exErr := fetch.ExpandArchivesOptional(dir)
			if exErr == nil && er != nil {
				res.Steps = append(res.Steps, er.Steps...)
			}
			projects, err = findComposeProjects(dir)
		}
		if err != nil {
			return res, err
		}
		if named := findNamedWaterworkComposes(dir); len(named) > 0 {
			res.Steps = append(res.Steps, fmt.Sprintf("市政水厂将部署 %d 套 compose：%s", len(projects), composeProjectLabels(projects)))
			if len(named) == 1 {
				res.Steps = append(res.Steps, "WARN: 只找到 waterwork-center / waterwork-device 其中一套，夹层下通常两套都要部署")
			}
		}
		d := dockerx.New()
		for _, compose := range projects {
			if err := composeUpOne(dir, opts, compose, d, loadedRefs, res); err != nil {
				return res, err
			}
		}
		// Nacos：compose 起来后立刻放行 8848/9848，否则控制台访问与配置导入会连不上
		if IsNacosModule(dir) {
			fwSteps, fwErr := OpenNacosFirewall(opts.Site)
			res.Steps = append(res.Steps, fwSteps...)
			if fwErr != nil {
				res.Steps = append(res.Steps, "WARN: "+fwErr.Error()+"（请手工放行后再导入配置）")
			}
		}
	}

	return res, nil
}

func composeProjectLabels(projects []string) string {
	var names []string
	for _, p := range projects {
		names = append(names, filepath.Base(filepath.Dir(p)))
	}
	return strings.Join(names, "、")
}

func composeUpOne(moduleDir string, opts Options, compose string, d *dockerx.Runner, loadedRefs []string, res *Result) error {
	res.ComposeFile = compose
	composeDir := filepath.Dir(compose)
	composeFile := filepath.Base(compose)
	if filepath.Clean(composeDir) != filepath.Clean(moduleDir) {
		res.Steps = append(res.Steps, "使用 compose: "+compose)
	}
	if opts.ComposeBuild {
		baseSteps, baseErr := loadBaseImagesForBuild([]string{moduleDir, composeDir}, composeDir, d)
		res.Steps = append(res.Steps, baseSteps...)
		if baseErr != nil {
			return baseErr
		}
	} else {
		if notes, err := SyncComposeImagesFromDocker(compose, d, loadedRefs); err != nil {
			return fmt.Errorf("同步 compose 镜像版本失败: %w", err)
		} else if len(notes) > 0 {
			for _, n := range notes {
				res.Steps = append(res.Steps, "compose 镜像已同步: "+n)
			}
		}
		images, _ := imagesFromCompose(compose)
		if missing := d.MissingImages(images); len(missing) > 0 {
			return fmt.Errorf("本地缺少镜像（需先 docker load）：%s", strings.Join(missing, ", "))
		}
	}
	if err := d.ComposeUp(composeDir, composeFile, nil, opts.ComposeBuild); err != nil {
		return err
	}
	label := filepath.Base(composeDir)
	if opts.ComposeBuild {
		res.Steps = append(res.Steps, "docker compose up -d --build 完成（"+label+"）")
	} else {
		res.Steps = append(res.Steps, "docker compose up -d 完成（"+label+"）")
	}
	return nil
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
