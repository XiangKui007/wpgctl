package fetch

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

var skipExpandDirs = map[string]struct{}{
	"data": {}, "logs": {}, "log": {}, ".git": {}, "node_modules": {},
}

// ExpandArchives 解压目录下的 .zip，并递归将 .tar.zip 展开为 .tar。
// 幂等：已存在的目标目录/文件会跳过。
func ExpandArchives(root string) (*ExpandResult, error) {
	abs, err := filepath.Abs(strings.TrimSpace(root))
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
			lower := strings.ToLower(info.Name())
			dir := filepath.Dir(path)

			if strings.HasSuffix(lower, ".tar.zip") {
				dest := strings.TrimSuffix(path, ".zip")
				if util.FileExists(dest) {
					return nil
				}
				if err := unwrapTarZip(path, dest); err != nil {
					res.Steps = append(res.Steps, fmt.Sprintf("跳过 %s: %v", filepath.Base(path), err))
					return nil
				}
				res.TarUnwrapped++
				res.Steps = append(res.Steps, fmt.Sprintf("展开 %s → %s", filepath.Base(path), filepath.Base(dest)))
				changed = true
				return nil
			}

			if strings.HasSuffix(lower, ".zip") {
				base := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
				dest := filepath.Join(dir, base)
				if dirHasEntries(dest) {
					return nil
				}
				if err := unzipFile(path, dest); err != nil {
					res.Steps = append(res.Steps, fmt.Sprintf("跳过 %s: %v", filepath.Base(path), err))
					return nil
				}
				res.ZipExtracted++
				res.Steps = append(res.Steps, fmt.Sprintf("解压 %s → %s/", filepath.Base(path), base))
				changed = true
			}
			return nil
		})
		if err != nil {
			return res, err
		}
		if !changed {
			break
		}
	}

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

	if res.ZipExtracted == 0 && res.TarUnwrapped == 0 && len(res.TarFiles) == 0 && len(res.TarGzFiles) == 0 {
		return res, fmt.Errorf("%w: 目录内未找到 .zip / .tar.zip / .tar / .tar.gz: %s", ErrNoArchivesFound, abs)
	}
	return res, nil
}

// ExpandArchivesOptional 同 ExpandArchives，但目录内无压缩包时不报错（用于 nginx/html 等可选目录）。
func ExpandArchivesOptional(root string) (*ExpandResult, error) {
	res, err := ExpandArchives(root)
	if errors.Is(err, ErrNoArchivesFound) {
		return res, nil
	}
	return res, err
}

func dirHasEntries(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func unzipFile(zipPath, destDir string) error {
	if err := util.EnsureDir(destDir); err != nil {
		return err
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("非法 zip 路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := util.EnsureDir(target); err != nil {
				return err
			}
			continue
		}
		if err := util.EnsureDir(filepath.Dir(target)); err != nil {
			return err
		}
		if err := extractZipEntry(f, target); err != nil {
		 return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, dest string) error {
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
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func unwrapTarZip(zipPath, destTar string) error {
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
	return extractZipEntry(candidate, destTar)
}
