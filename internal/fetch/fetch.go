// Package fetch 实现拉包、分卷合并、sha256 校验与解压（方案 §6.4）。
package fetch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wpg/wpgctl/internal/util"
)

// Options 拉包选项。
type Options struct {
	Name     string // 例如 release-4.0.2
	FromURL  string // 覆盖拉包基址
	LocalDir string // 离线模式：本地分卷目录
	DestDir  string // 解压目标，默认 ~/.wpgctl/packages/<name>
}

// Result 拉包结果。
type Result struct {
	PackageDir string
	Bytes      int64
}

// Run 下载（或本地导入）→ 合并分卷 → 校验 → 解压。
func Run(opts Options) (*Result, error) {
	if opts.Name == "" && opts.LocalDir == "" {
		return nil, fmt.Errorf("必须指定包名或 --local 目录")
	}
	name := opts.Name
	if name == "" {
		name = filepath.Base(strings.TrimRight(opts.LocalDir, `/\`))
	}
	dest := opts.DestDir
	if dest == "" {
		dest = filepath.Join(util.PackagesDir(), name)
	}
	work := filepath.Join(util.PackagesDir(), ".download", name)
	_ = util.EnsureDir(work)
	_ = util.EnsureDir(dest)

	var parts []string
	var err error
	if opts.LocalDir != "" {
		parts, err = collectLocalParts(opts.LocalDir)
		if err != nil {
			return nil, err
		}
	} else {
		base := opts.FromURL
		if base == "" {
			base = os.Getenv("WPGCTL_FETCH_URL")
		}
		if base == "" {
			return nil, fmt.Errorf("未配置拉包地址，请传 --from 或设置 WPGCTL_FETCH_URL，或使用 --local")
		}
		parts, err = downloadParts(base, name, work)
		if err != nil {
			return nil, err
		}
	}

	merged := filepath.Join(work, name+".tar.gz")
	n, err := mergeParts(parts, merged)
	if err != nil {
		return nil, err
	}
	util.Infof("分卷合并完成: %s (%d bytes)", merged, n)

	sumFile := findChecksumFile(opts.LocalDir, work)
	if sumFile != "" {
		if err := verifySHA256(sumFile, work, dest); err != nil {
			return nil, err
		}
	} else {
		util.Warnf("未找到 sha256sums.txt，跳过完整性校验")
	}

	if err := extractTarGz(merged, dest); err != nil {
		return nil, err
	}
	util.Successf("拉包完成 → %s", dest)
	return &Result{PackageDir: dest, Bytes: n}, nil
}

func collectLocalParts(dir string) ([]string, error) {
	parts := findArchiveParts(dir)
	if len(parts) == 0 {
		// 若已是完整包目录（含 manifest），直接返回空让上层复制
		if ready := findReadyPackageDir(dir); ready != "" {
			return nil, fmt.Errorf("LOCAL_READY:" + ready)
		}
		if findLegacyMiddlewareDir(dir) == "" {
			if res, expandErr := ExpandArchives(dir); expandErr == nil &&
				(res.ZipExtracted > 0 || res.TarUnwrapped > 0) {
				util.Infof("已自动解压 %d 个 zip、展开 %d 个 tar.zip", res.ZipExtracted, res.TarUnwrapped)
				parts = findArchiveParts(dir)
			}
		}
		if len(parts) > 0 {
			return parts, nil
		}
		if legacy := findLegacyMiddlewareDir(dir); legacy != "" {
			return nil, fmt.Errorf(
				"检测到旧版中间件目录（%s），不是 wpgctl 交付包。\n"+
					"包中心只接受：① 含 manifest.yaml 的 release/base 包目录；② .tar.gz / 分卷文件。\n"+
					"此目录是各服务独立 compose + 镜像 tar，请在各服务子目录手动 docker compose up，"+
					"或把带 manifest.yaml 的业务包填到交付向导的「Release 包目录」",
				legacy,
			)
		}
		return nil, fmt.Errorf(
			"本地目录未找到可用包：需要 *.tar.gz / *.tar.gz.aa 分卷，或含 manifest.yaml 的完整包目录（当前: %s）",
			dir,
		)
	}
	sort.Strings(parts)
	return parts, nil
}

func findArchiveParts(dir string) []string {
	var parts []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		lower := strings.ToLower(name)
		if strings.Contains(lower, ".tar.gz") || strings.HasSuffix(lower, ".part") ||
			strings.Contains(lower, ".tar.gz.") {
			parts = append(parts, path)
		}
		return nil
	})
	sort.Strings(parts)
	return parts
}

// findReadyPackageDir 返回含 manifest.yaml 的包根（含一层同名嵌套）。
func findReadyPackageDir(dir string) string {
	if util.FileExists(filepath.Join(dir, "manifest.yaml")) {
		return dir
	}
	nested := filepath.Join(dir, filepath.Base(dir))
	if util.FileExists(filepath.Join(nested, "manifest.yaml")) {
		return nested
	}
	return ""
}

// findLegacyMiddlewareDir 识别「每服务一个 compose + 镜像 tar」的旧中间件包。
func findLegacyMiddlewareDir(dir string) string {
	for _, candidate := range []string{dir, filepath.Join(dir, filepath.Base(dir))} {
		if looksLikeLegacyMiddleware(candidate) {
			return candidate
		}
	}
	return ""
}

func looksLikeLegacyMiddleware(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return false
	}
	composeCount := 0
	tarCount := 0
	for _, e := range entries {
		if !e.IsDir() {
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, ".tar") || strings.HasSuffix(name, ".tar.zip") {
				tarCount++
			}
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if util.FileExists(filepath.Join(sub, "docker-compose.yml")) ||
			util.FileExists(filepath.Join(sub, "docker-compose.yaml")) {
			composeCount++
		}
	}
	return composeCount >= 3 || (composeCount >= 1 && tarCount >= 1)
}

func downloadParts(base, name, work string) ([]string, error) {
	// 约定：index 或直接尝试 .tar.gz / .tar.gz.aa 分卷
	client := &http.Client{Timeout: 0}
	candidates := []string{
		fmt.Sprintf("%s/%s.tar.gz", strings.TrimRight(base, "/"), name),
	}
	for _, u := range candidates {
		dest := filepath.Join(work, filepath.Base(u))
		n, err := downloadResume(client, u, dest)
		if err != nil {
			return nil, err
		}
		util.Infof("下载完成 %s (%d bytes)", u, n)
		return []string{dest}, nil
	}
	return nil, fmt.Errorf("无可用下载地址")
}

// downloadResume 支持 HTTP Range 断点续传。
func downloadResume(client *http.Client, url, dest string) (int64, error) {
	var existing int64
	if fi, err := os.Stat(dest); err == nil {
		existing = fi.Size()
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	if existing > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
	}

	var resp *http.Response
	for attempt := 0; attempt < 5; attempt++ {
		resp, err = client.Do(req)
		if err == nil && (resp.StatusCode == 200 || resp.StatusCode == 206) {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		wait := time.Duration(1<<attempt) * time.Second
		util.Warnf("下载失败，%s 后重试 (%d/5): %v", wait, attempt+1, err)
		time.Sleep(wait)
	}
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	flag := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == 206 {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
		existing = 0
	}
	f, err := os.OpenFile(dest, flag, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	return existing + n, err
}

func mergeParts(parts []string, dest string) (int64, error) {
	if len(parts) == 1 && strings.HasSuffix(parts[0], ".tar.gz") {
		// 已是完整文件，复制或硬链
		data, err := os.ReadFile(parts[0])
		if err != nil {
			return 0, err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return 0, err
		}
		return int64(len(data)), nil
	}
	out, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	var total int64
	for _, p := range parts {
		f, err := os.Open(p)
		if err != nil {
			return total, err
		}
		n, err := io.Copy(out, f)
		f.Close()
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func findChecksumFile(localDir, work string) string {
	for _, d := range []string{localDir, work} {
		if d == "" {
			continue
		}
		p := filepath.Join(d, "sha256sums.txt")
		if util.FileExists(p) {
			return p
		}
	}
	return ""
}

func verifySHA256(sumFile, work, _ string) error {
	data, err := os.ReadFile(sumFile)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		expect, file := fields[0], fields[len(fields)-1]
		path := filepath.Join(work, file)
		if !util.FileExists(path) {
			path = filepath.Join(filepath.Dir(sumFile), file)
		}
		if !util.FileExists(path) {
			util.Warnf("校验清单中的文件不存在，跳过: %s", file)
			continue
		}
		got, err := util.SHA256File(path)
		if err != nil {
			return err
		}
		if !strings.EqualFold(got, expect) {
			return fmt.Errorf("校验失败: %s 期望 %s 实际 %s（请重传该分卷）", file, expect, got)
		}
		util.Infof("校验通过: %s", file)
	}
	return nil
}

func extractTarGz(archive, dest string) error {
	// 使用系统 tar，现场 Linux 必有；Windows 演练可用 bsdtar。
	return extractWithTar(archive, dest)
}
