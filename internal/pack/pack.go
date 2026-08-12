// Package pack 实现公司侧打包（方案 §7.1）：base / release / patch。
package pack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 打包选项。
type Options struct {
	Kind         string // base|release|patch
	Version      string
	ManifestPath string
	OutputDir    string
	Services     []string // patch 涉及服务
	BaseRelease  string   // patch 基线
	WorkDir      string   // 组装工作目录来源（镜像/jar/模板）
	VolumeSizeGB int      // 分卷大小，默认 2G
}

// Result 打包结果。
type Result struct {
	PackageDir string
	Archive    string
	Parts      []string
}

// 强制排除的运行时目录（方案 §4.2 红线）。
var excludeDirNames = map[string]struct{}{
	"data": {}, "logs": {}, "log": {}, ".git": {},
}

// Run 执行打包：组装目录 → 生成 sha256 → tar → 分卷。
func Run(opts Options) (*Result, error) {
	if opts.Kind != "base" && opts.Kind != "release" && opts.Kind != "patch" {
		return nil, fmt.Errorf("kind 必须是 base|release|patch")
	}
	if opts.Version == "" {
		return nil, fmt.Errorf("version 不能为空")
	}
	if opts.OutputDir == "" {
		opts.OutputDir = "./dist"
	}
	if opts.VolumeSizeGB <= 0 {
		opts.VolumeSizeGB = 2
	}
	_ = util.EnsureDir(opts.OutputDir)

	pkgName := fmt.Sprintf("wpg-%s-%s", opts.Kind, opts.Version)
	pkgDir := filepath.Join(opts.OutputDir, pkgName)
	_ = os.RemoveAll(pkgDir)
	if err := util.EnsureDir(pkgDir); err != nil {
		return nil, err
	}

	// 复制工作区内容（排除 data/logs）
	if opts.WorkDir != "" {
		if err := copyTreeFiltered(opts.WorkDir, pkgDir); err != nil {
			return nil, err
		}
	}

	// 写入/覆盖 manifest
	if opts.ManifestPath != "" {
		data, err := os.ReadFile(opts.ManifestPath)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "manifest.yaml"), data, 0o644); err != nil {
			return nil, err
		}
		// 校验
		if _, err := config.LoadManifest(filepath.Join(pkgDir, "manifest.yaml")); err != nil {
			return nil, fmt.Errorf("manifest 校验失败: %w", err)
		}
	} else if opts.Kind == "patch" {
		content := fmt.Sprintf("kind: patch\nversion: %s\nbaseRelease: %s\narch: [amd64]\nservicesList: [%s]\n",
			opts.Version, opts.BaseRelease, strings.Join(quoteAll(opts.Services), ", "))
		if err := os.WriteFile(filepath.Join(pkgDir, "manifest.yaml"), []byte(content), 0o644); err != nil {
			return nil, err
		}
	}

	sumPath := filepath.Join(pkgDir, "sha256sums.txt")
	if err := writeChecksums(pkgDir, sumPath); err != nil {
		return nil, err
	}

	archive := filepath.Join(opts.OutputDir, pkgName+".tar.gz")
	if err := tarGz(pkgDir, archive); err != nil {
		return nil, err
	}

	parts, err := splitFile(archive, opts.VolumeSizeGB*1024*1024*1024)
	if err != nil {
		return nil, err
	}
	util.Successf("打包完成: %s（%d 个分卷）", archive, len(parts))
	return &Result{PackageDir: pkgDir, Archive: archive, Parts: parts}, nil
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

func copyTreeFiltered(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		// 排除运行时目录
		parts := strings.Split(filepath.ToSlash(rel), "/")
		for _, p := range parts {
			if _, bad := excludeDirNames[p]; bad {
				util.Warnf("打包排除运行时目录: %s", rel)
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		dest := filepath.Join(dst, rel)
		if info.IsDir() {
			return util.EnsureDir(dest)
		}
		return copyFile(path, dest)
	})
}

func writeChecksums(root, sumPath string) error {
	f, err := os.Create(sumPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if filepath.Base(path) == "sha256sums.txt" {
			return nil
		}
		sum, err := hashFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		_, err = fmt.Fprintf(f, "%s  %s\n", sum, filepath.ToSlash(rel))
		return err
	})
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func tarGz(srcDir, dest string) error {
	parent := filepath.Dir(srcDir)
	base := filepath.Base(srcDir)
	cmd := exec.Command("tar", "-czf", dest, "-C", parent, base)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func splitFile(path string, size int) ([]string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() <= int64(size) {
		return []string{path}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var parts []string
	idx := 0
	buf := make([]byte, 32*1024)
	for {
		part := fmt.Sprintf("%s.part%03d", path, idx)
		out, err := os.Create(part)
		if err != nil {
			return parts, err
		}
		var written int64
		for written < int64(size) {
			toRead := len(buf)
			if int64(toRead) > int64(size)-written {
				toRead = int(int64(size) - written)
			}
			n, rerr := f.Read(buf[:toRead])
			if n > 0 {
				if _, werr := out.Write(buf[:n]); werr != nil {
					out.Close()
					return parts, werr
				}
				written += int64(n)
			}
			if rerr == io.EOF {
				out.Close()
				parts = append(parts, part)
				return parts, nil
			}
			if rerr != nil {
				out.Close()
				return parts, rerr
			}
		}
		out.Close()
		parts = append(parts, part)
		idx++
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = util.EnsureDir(filepath.Dir(dst))
	return os.WriteFile(dst, data, 0o644)
}
