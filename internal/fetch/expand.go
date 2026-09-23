package fetch

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wpg/wpgctl/internal/util"
)

// ErrNoArchivesFound 目录内无任何可展开压缩包。
var ErrNoArchivesFound = errors.New("no archives found")

// ExpandResult 目录内压缩包展开结果。
type ExpandResult struct {
	RootDir      string   `json:"rootDir"`
	Steps        []string `json:"steps"`
	ZipExtracted int      `json:"zipExtracted"`
	TarUnwrapped int      `json:"tarUnwrapped"`
	TarFiles     []string `json:"tarFiles"`
	TarGzFiles   []string `json:"tarGzFiles"`
}

// ExpandOptions 解压范围与进度回调。
// Root 扫整棵目录；File 只解这一个 .zip / .tar.zip（优先于 Root）。
type ExpandOptions struct {
	Root     string
	File     string
	Log      func(string)
	Progress func(ExpandProgress)
	// DestName 把 zip 文件名（不含 .zip）映射成解压目录名；空则用原名。nginx/html 会自动去掉版本号。
	DestName func(zipBase string) string
}

// ExpandProgress 解压即时进度。Percent 含当前包内部字节/条目，避免整包解完才从 0 跳到 100。
type ExpandProgress struct {
	Done        int    // 已完成的压缩包个数
	Total       int    // 本次要处理的压缩包个数
	Percent     int    // 总体 0–100（按各包体积加权）
	PackPercent int    // 当前这一包 0–100
	Current     string // 当前压缩包文件名
	Entry       string // 当前正在写出的内部文件名
	FileDone    int    // 当前包已处理条目
	FileTotal   int    // 当前包条目总数
	BytesDone   int64  // 当前包已写出的未压缩字节
	BytesTotal  int64  // 当前包未压缩总字节
}

var skipExpandDirs = map[string]struct{}{
	"data": {}, "logs": {}, "log": {}, ".git": {}, "node_modules": {},
}

// ExpandArchives 解压目录下的 .zip，并递归将 .tar.zip 展开为 .tar。
// 幂等：已存在的目标目录/文件会跳过。
func ExpandArchives(root string) (*ExpandResult, error) {
	return Expand(ExpandOptions{Root: root})
}

// Expand 按选项解压，并通过 Log / Progress 汇报当前包及包内字节进度（供控制台进度条）。
func Expand(opts ExpandOptions) (*ExpandResult, error) {
	file := strings.TrimSpace(opts.File)
	root := strings.TrimSpace(opts.Root)
	if file != "" {
		return expandOne(file, opts)
	}
	if root == "" {
		return nil, fmt.Errorf("目录不能为空")
	}
	return expandTree(root, opts)
}

// ArchiveExpanded 目标已存在则不必再解（zip→目录，tar.zip→同名 .tar）。
// nginx/html 下的 zip 目标目录会去掉版本号与 3. / 4. 序号前缀。
func ArchiveExpanded(path string) bool {
	return archiveExpanded(path, nil)
}

func archiveExpanded(path string, namer func(string) string) bool {
	kind := archiveKindOf(path)
	switch kind {
	case "tar.zip":
		return util.FileExists(trimSuffixFold(path, ".zip"))
	case "zip":
		return dirHasEntries(zipExtractDir(path, namer))
	default:
		return false
	}
}

func expandOne(path string, opts ExpandOptions) (*ExpandResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("压缩包不可访问: %w", err)
	}
	if st.IsDir() {
		return nil, fmt.Errorf("不是压缩包: %s", abs)
	}
	kind := archiveKindOf(abs)
	if kind != "zip" && kind != "tar.zip" {
		return nil, fmt.Errorf("只支持 .zip / .tar.zip: %s", filepath.Base(abs))
	}
	res := &ExpandResult{RootDir: filepath.Dir(abs)}
	logf := makeExpandLog(res, opts.Log)
	logf("解压文件：%s", abs)
	rep := &expandReporter{opts: opts}
	rep.noteArchive(abs, st.Size())
	if archiveExpanded(abs, opts.DestName) {
		logf("跳过 %s：已经展开过", filepath.Base(abs))
		rep.markAllDone(filepath.Base(abs))
		collectTars(res, res.RootDir)
		return res, nil
	}
	rep.start(abs, filepath.Base(abs), st.Size())
	if err := extractArchive(abs, kind, st.Size(), res, logf, opts.DestName, rep); err != nil {
		return res, err
	}
	rep.finishCurrent(filepath.Base(abs))
	collectTars(res, res.RootDir)
	return res, nil
}

func expandTree(root string, opts ExpandOptions) (*ExpandResult, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("目录不可访问: %w", err)
	}
	if !st.IsDir() {
		abs = filepath.Dir(abs)
	}

	res := &ExpandResult{RootDir: abs}
	logf := makeExpandLog(res, opts.Log)
	if isHTMLParent(abs) {
		normalizeHTMLTree(abs, logf)
	}
	pending := listPendingArchives(abs, opts.DestName)
	logf("解压目录：%s（待处理 %d 个压缩包）", abs, len(pending))
	rep := &expandReporter{opts: opts}
	rep.noteArchives(pending)
	rep.emit(ExpandProgress{Done: 0, Total: rep.total}, true)

	const maxPass = 12
	for pass := 0; pass < maxPass; pass++ {
		changed := false
		err := filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				if info != nil && info.IsDir() {
					if _, skip := skipExpandDirs[strings.ToLower(info.Name())]; skip {
						return filepath.SkipDir
					}
				}
				return nil
			}
			kind := archiveKindOf(info.Name())
			if kind != "zip" && kind != "tar.zip" {
				return nil
			}
			if archiveExpanded(path, opts.DestName) {
				return nil
			}
			rep.start(path, info.Name(), info.Size())
			if err := extractArchive(path, kind, info.Size(), res, logf, opts.DestName, rep); err != nil {
				logf("跳过 %s: %v", info.Name(), err)
				rep.finishCurrent(info.Name())
				return nil
			}
			changed = true
			rep.finishCurrent(info.Name())
			return nil
		})
		if err != nil {
			return res, err
		}
		if !changed {
			break
		}
		pending = listPendingArchives(abs, opts.DestName)
		rep.noteArchives(pending)
	}

	if isHTMLParent(abs) {
		normalizeHTMLTree(abs, logf)
	}

	collectTars(res, abs)
	if res.ZipExtracted == 0 && res.TarUnwrapped == 0 && len(res.TarFiles) == 0 && len(res.TarGzFiles) == 0 {
		return res, fmt.Errorf("%w: 目录内未找到 .zip / .tar.zip / .tar / .tar.gz: %s", ErrNoArchivesFound, abs)
	}
	if res.ZipExtracted == 0 && res.TarUnwrapped == 0 {
		logf("没有新的压缩包需要解压（发现 .tar %d 个）", len(res.TarFiles))
	} else {
		logf("解压完成：zip=%d tar.zip=%d .tar=%d", res.ZipExtracted, res.TarUnwrapped, len(res.TarFiles))
	}
	rep.noteArchives(listPendingArchives(abs, opts.DestName))
	if rep.total > 0 {
		rep.emit(ExpandProgress{Done: rep.done, Total: rep.total}, true)
	}
	return res, nil
}

func extractArchive(path, kind string, size int64, res *ExpandResult, logf func(string, ...any), namer func(string) string, rep *expandReporter) error {
	name := filepath.Base(path)
	switch kind {
	case "tar.zip":
		dest := trimSuffixFold(path, ".zip")
		logf("正在展开 %s（%s）→ %s", name, formatSize(size), filepath.Base(dest))
		if err := unwrapTarZip(path, dest, func(copied, total int64) {
			rep.inside(0, 1, copied, total, filepath.Base(dest))
		}); err != nil {
			return err
		}
		res.TarUnwrapped++
		logf("完成 %s → %s", name, filepath.Base(dest))
	case "zip":
		dest := zipExtractDir(path, namer)
		logf("正在解压 %s（%s）→ %s/", name, formatSize(size), filepath.Base(dest))
		if err := unzipFile(path, dest, logf, func(fileDone, fileTotal int, bytesDone, bytesTotal int64, entry string) {
			rep.inside(fileDone, fileTotal, bytesDone, bytesTotal, entry)
		}); err != nil {
			return err
		}
		if namer != nil || isHTMLParent(filepath.Dir(path)) {
			if err := flattenHTMLWrapper(dest, logf); err != nil {
				return err
			}
		}
		res.ZipExtracted++
		logf("完成 %s → %s/", name, filepath.Base(dest))
	default:
		return fmt.Errorf("不支持的压缩包: %s", name)
	}
	return nil
}

func listPendingArchives(root string, namer func(string) string) []string {
	var out []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			if info != nil && info.IsDir() {
				if _, skip := skipExpandDirs[strings.ToLower(info.Name())]; skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		kind := archiveKindOf(info.Name())
		if kind != "zip" && kind != "tar.zip" {
			return nil
		}
		if !archiveExpanded(path, namer) {
			out = append(out, path)
		}
		return nil
	})
	return out
}

func collectTars(res *ExpandResult, abs string) {
	_ = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			if info != nil && info.IsDir() {
				if _, skip := skipExpandDirs[strings.ToLower(info.Name())]; skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		lower := strings.ToLower(info.Name())
		if strings.HasSuffix(lower, ".tar") && !strings.HasSuffix(lower, ".tar.zip") {
			res.TarFiles = append(res.TarFiles, path)
		}
		if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
			if !strings.Contains(lower, ".part") {
				res.TarGzFiles = append(res.TarGzFiles, path)
			}
		}
		return nil
	})
}

func makeExpandLog(res *ExpandResult, extra func(string)) func(string, ...any) {
	return func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		res.Steps = append(res.Steps, msg)
		if extra != nil {
			extra(msg)
		}
	}
}

const progressMinInterval = 200 * time.Millisecond

// expandReporter 把多包体积 + 包内字节折成总体百分比，并节流回调。
// 大包占的进度更长；解出嵌套 zip 后只拉长剩余段，总体百分比不往回跳。
type expandReporter struct {
	opts    ExpandOptions
	done    int
	total   int
	name    string
	weight  int64
	doneW   int64
	totalW  int64
	counted map[string]int64
	last    time.Time
	lastP   int
}

func (r *expandReporter) noteArchives(paths []string) {
	for _, p := range paths {
		var size int64
		if st, err := os.Stat(p); err == nil {
			size = st.Size()
		}
		r.noteArchive(p, size)
	}
}

func (r *expandReporter) noteArchive(path string, size int64) {
	if r == nil {
		return
	}
	if r.counted == nil {
		r.counted = map[string]int64{}
	}
	key := filepath.Clean(path)
	if _, ok := r.counted[key]; ok {
		return
	}
	if size <= 0 {
		if st, err := os.Stat(path); err == nil {
			size = st.Size()
		}
	}
	if size <= 0 {
		size = 1
	}
	r.counted[key] = size
	r.totalW += size
	r.total = len(r.counted)
}

func (r *expandReporter) start(path, name string, size int64) {
	if r == nil {
		return
	}
	r.noteArchive(path, size)
	r.name = name
	r.weight = r.counted[filepath.Clean(path)]
	r.emit(ExpandProgress{Done: r.done, Total: r.total, Current: name}, true)
}

func (r *expandReporter) inside(fileDone, fileTotal int, bytesDone, bytesTotal int64, entry string) {
	if r == nil {
		return
	}
	r.emit(ExpandProgress{
		Done:       r.done,
		Total:      r.total,
		Current:    r.name,
		Entry:      zipEntryBase(entry),
		FileDone:   fileDone,
		FileTotal:  fileTotal,
		BytesDone:  bytesDone,
		BytesTotal: bytesTotal,
	}, false)
}

func (r *expandReporter) finishCurrent(name string) {
	if r == nil {
		return
	}
	r.done++
	r.doneW += r.weight
	r.weight = 0
	if name != "" {
		r.name = name
	}
	r.emit(ExpandProgress{Done: r.done, Total: r.total, Current: r.name}, true)
}

func (r *expandReporter) markAllDone(name string) {
	if r == nil {
		return
	}
	r.done = r.total
	r.doneW = r.totalW
	r.weight = 0
	if name != "" {
		r.name = name
	}
	r.emit(ExpandProgress{Done: r.done, Total: r.total, Current: r.name}, true)
}

func (r *expandReporter) emit(p ExpandProgress, force bool) {
	if r == nil || r.opts.Progress == nil {
		return
	}
	frac := packFrac(p.BytesDone, p.BytesTotal, p.FileDone, p.FileTotal)
	p.PackPercent = int(frac * 100)
	if p.PackPercent > 100 {
		p.PackPercent = 100
	}
	if r.totalW > 0 {
		p.Percent = int((float64(r.doneW) + float64(r.weight)*frac) * 100 / float64(r.totalW))
	} else {
		p.Percent = overallPercent(p.Done, p.Total, p.BytesDone, p.BytesTotal, p.FileDone, p.FileTotal)
	}
	if p.Percent < r.lastP {
		p.Percent = r.lastP
	}
	if p.Percent > 100 {
		p.Percent = 100
	}
	now := time.Now()
	if !force && p.Percent == r.lastP && now.Sub(r.last) < progressMinInterval {
		return
	}
	r.lastP = p.Percent
	r.last = now
	r.opts.Progress(p)
}

func packFrac(bytesDone, bytesTotal int64, fileDone, fileTotal int) float64 {
	switch {
	case bytesTotal > 0:
		return clamp01(float64(bytesDone) / float64(bytesTotal))
	case fileTotal > 0:
		return clamp01(float64(fileDone) / float64(fileTotal))
	default:
		return 0
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func overallPercent(done, total int, bytesDone, bytesTotal int64, fileDone, fileTotal int) int {
	if total <= 0 {
		if done > 0 {
			return 100
		}
		return 0
	}
	frac := 0.0
	switch {
	case bytesTotal > 0:
		frac = float64(bytesDone) / float64(bytesTotal)
	case fileTotal > 0:
		frac = float64(fileDone) / float64(fileTotal)
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	pct := int((float64(done) + frac) * 100 / float64(total))
	if pct > 100 {
		return 100
	}
	if pct < 0 {
		return 0
	}
	return pct
}

func zipEntryBase(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

func archiveKindOf(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.zip"):
		return "tar.zip"
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".tar"):
		return "tar"
	default:
		return ""
	}
}

func trimSuffixFold(s, suffix string) string {
	if len(s) < len(suffix) {
		return s
	}
	if strings.EqualFold(s[len(s)-len(suffix):], suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func formatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// ExpandArchivesOptional 同 ExpandArchives，但目录内无压缩包时不报错（用于 nginx/html 等可选目录）。
func ExpandArchivesOptional(root string) (*ExpandResult, error) {
	res, err := ExpandArchives(root)
	if errors.Is(err, ErrNoArchivesFound) {
		return res, nil
	}
	return res, err
}

// UnwrapTarZipIfNeeded 将 .tar.zip 展开为同目录 .tar（已存在则跳过）。现场 java8.tar.zip 用这个。
func UnwrapTarZipIfNeeded(zipPath string) (string, error) {
	zipPath = strings.TrimSpace(zipPath)
	if zipPath == "" {
		return "", fmt.Errorf("zip 路径为空")
	}
	dest := trimSuffixFold(zipPath, ".zip")
	if archiveKindOf(zipPath) != "tar.zip" {
		return "", fmt.Errorf("不是 .tar.zip: %s", filepath.Base(zipPath))
	}
	if util.FileExists(dest) {
		return dest, nil
	}
	if err := unwrapTarZip(zipPath, dest, nil); err != nil {
		return "", err
	}
	return dest, nil
}

func dirHasEntries(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func unzipFile(zipPath, destDir string, logf func(string, ...any), onProg func(fileDone, fileTotal int, bytesDone, bytesTotal int64, entry string)) error {
	if err := util.EnsureDir(destDir); err != nil {
		return err
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	total := len(r.File)
	var uncompressed uint64
	for _, f := range r.File {
		uncompressed += f.UncompressedSize64
	}
	var copied uint64
	lastPct := -1
	for i, f := range r.File {
		target := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("非法 zip 路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := util.EnsureDir(target); err != nil {
				return err
			}
			if onProg != nil {
				onProg(i+1, total, int64(copied), int64(uncompressed), f.Name)
			}
			continue
		}
		if err := util.EnsureDir(filepath.Dir(target)); err != nil {
			return err
		}
		baseCopied := copied
		if err := extractZipEntry(f, target, func(n, _ int64) {
			if onProg != nil {
				onProg(i, total, int64(baseCopied)+n, int64(uncompressed), f.Name)
			}
		}); err != nil {
			return err
		}
		copied += f.UncompressedSize64
		if onProg != nil {
			onProg(i+1, total, int64(copied), int64(uncompressed), f.Name)
		}
		pct := 0
		if uncompressed > 0 {
			pct = int(copied * 100 / uncompressed)
		} else if total > 0 {
			pct = (i + 1) * 100 / total
		}
		if logf != nil && pct != lastPct && (pct == 0 || pct == 100 || pct-lastPct >= 10) {
			logf("  %s %d%%（%d/%d 个条目）", filepath.Base(zipPath), pct, i+1, total)
			lastPct = pct
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, dest string, onBytes func(copied, total int64)) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	tmp := dest + ".part"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	total := int64(f.UncompressedSize64)
	var copied int64
	buf := make([]byte, 256*1024)
	for {
		n, readErr := rc.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				_ = os.Remove(tmp)
				return werr
			}
			copied += int64(n)
			if onBytes != nil {
				onBytes(copied, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			_ = os.Remove(tmp)
			return readErr
		}
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func unwrapTarZip(zipPath, destTar string, onBytes func(copied, total int64)) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	var candidate *zip.File
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.ToLower(filepath.Base(f.Name))
		if strings.HasSuffix(name, ".tar") {
			candidate = f
			break
		}
		if candidate == nil || f.UncompressedSize64 > candidate.UncompressedSize64 {
			candidate = f
		}
	}
	if candidate == nil {
		return fmt.Errorf("zip 内无文件")
	}
	if err := util.EnsureDir(filepath.Dir(destTar)); err != nil {
		return err
	}
	return extractZipEntry(candidate, destTar, onBytes)
}
