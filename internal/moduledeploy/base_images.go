package moduledeploy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/fetch"
	"github.com/wpg/wpgctl/internal/util"
)

// imageLoader 离线构建前 load / tag 基础镜像（单测注入，生产走 docker CLI）。
type imageLoader interface {
	ImageExists(image string) bool
	LoadImage(tarPath string) ([]string, error)
	TagImage(src, dest string) error
}

// loadBaseImagesForBuild 在 compose --build 前把 java8.tar 等基础镜像 load 进本机。
// 现场离线无法访问 Docker Hub，Dockerfile 的 FROM java:8 必须先有本地镜像。
func loadBaseImagesForBuild(searchDirs []string, composeDir string, d imageLoader) ([]string, error) {
	var steps []string
	needed := dockerfileFromImagesInDir(composeDir)
	if len(needed) == 0 {
		needed = []string{"java:8"}
	}

	var still []string
	for _, img := range needed {
		if d.ImageExists(img) {
			steps = append(steps, "基础镜像已在本地: "+img)
			continue
		}
		still = append(still, img)
	}
	if len(still) == 0 {
		return steps, nil
	}

	tars, unwrapNotes, err := collectBaseImageTars(searchDirs)
	steps = append(steps, unwrapNotes...)
	if err != nil {
		return steps, err
	}

	var loaded []string
	for _, tar := range tars {
		refs, err := d.LoadImage(tar)
		if err != nil {
			return steps, err
		}
		loaded = append(loaded, refs...)
		step := "docker load 基础镜像: " + filepath.Base(tar)
		if len(refs) > 0 {
			step += "（" + strings.Join(refs, ", ") + "）"
		}
		steps = append(steps, step)
		for _, tag := range inferTagsFromBaseTar(filepath.Base(tar)) {
			if d.ImageExists(tag) {
				continue
			}
			src := tagSource(refs)
			if src == "" {
				continue
			}
			if err := d.TagImage(src, tag); err != nil {
				return steps, err
			}
			steps = append(steps, "docker tag "+src+" -> "+tag)
			loaded = append(loaded, tag)
		}
	}

	var missing []string
	for _, img := range still {
		if d.ImageExists(img) {
			continue
		}
		src := tagSource(loaded)
		if src != "" && src != img {
			if err := d.TagImage(src, img); err != nil {
				return steps, err
			}
			steps = append(steps, "docker tag "+src+" -> "+img)
		}
		if !d.ImageExists(img) {
			missing = append(missing, img)
		}
	}
	if len(missing) > 0 {
		return steps, fmt.Errorf("本地没有基础镜像 %s，离线环境无法从 Docker Hub 拉取。请把对应 tar（如 java8.tar）放到模块目录（与 Dockerfile 同级或上一级）后重试", strings.Join(missing, ", "))
	}
	return steps, nil
}

func tagSource(refs []string) string {
	for _, r := range refs {
		if strings.TrimSpace(r) != "" {
			return r
		}
	}
	return ""
}

func collectBaseImageTars(searchDirs []string) (tars []string, steps []string, err error) {
	seen := map[string]struct{}{}
	for _, root := range uniqueExistingDirs(searchDirs) {
		found, notes, walkErr := findBaseImageTars(root)
		steps = append(steps, notes...)
		if walkErr != nil {
			return tars, steps, walkErr
		}
		for _, p := range found {
			key := filepath.Clean(p)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			tars = append(tars, key)
		}
	}
	return tars, steps, nil
}

func uniqueExistingDirs(dirs []string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(d string) {
		d = filepath.Clean(strings.TrimSpace(d))
		if d == "" || d == "." || d == string(filepath.Separator) {
			return
		}
		if !util.DirExists(d) {
			return
		}
		if _, ok := seen[d]; ok {
			return
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	for _, d := range dirs {
		add(d)
		p := filepath.Clean(strings.TrimSpace(d))
		for i := 0; i < 2; i++ {
			next := filepath.Dir(p)
			if next == p {
				break
			}
			add(next)
			p = next
		}
	}
	return out
}

func findBaseImageTars(root string) ([]string, []string, error) {
	var tars []string
	var steps []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			lower := strings.ToLower(info.Name())
			if lower == "data" || lower == "logs" || lower == "log" || lower == "bak" {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil && rel != "." && len(strings.Split(rel, string(os.PathSeparator))) > 4 {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if !isBaseImageArchiveName(name) {
			return nil
		}
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".tar.zip") {
			dest, uerr := fetch.UnwrapTarZipIfNeeded(path)
			if uerr != nil {
				steps = append(steps, "WARN: 展开 "+name+" 失败: "+uerr.Error())
				return nil
			}
			steps = append(steps, "展开 "+name+" → "+filepath.Base(dest))
			tars = append(tars, dest)
			return nil
		}
		if strings.HasSuffix(lower, ".tar") {
			tars = append(tars, path)
		}
		return nil
	})
	return tars, steps, err
}

func isBaseImageArchiveName(name string) bool {
	n := strings.ToLower(filepath.Base(name))
	n = strings.TrimSuffix(n, ".tar.zip")
	n = strings.TrimSuffix(n, ".tar")
	compact := strings.NewReplacer("_", "", "-", "", ".", "").Replace(n)
	if compact == "java8" || compact == "jdk8" || compact == "openjdk8" ||
		strings.HasPrefix(compact, "java8") || strings.HasPrefix(compact, "jdk8") || strings.HasPrefix(compact, "openjdk8") {
		return true
	}
	return false
}

func inferTagsFromBaseTar(name string) []string {
	n := strings.ToLower(filepath.Base(name))
	n = strings.TrimSuffix(n, ".tar.zip")
	n = strings.TrimSuffix(n, ".tar")
	compact := strings.NewReplacer("_", "", "-", "", ".", "").Replace(n)
	switch {
	case strings.HasPrefix(compact, "openjdk8"):
		return []string{"openjdk:8", "java:8"}
	case strings.HasPrefix(compact, "java8"), strings.HasPrefix(compact, "jdk8"):
		return []string{"java:8"}
	default:
		return nil
	}
}

func dockerfileFromImagesInDir(dir string) []string {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" || !util.DirExists(dir) {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			lower := strings.ToLower(info.Name())
			if lower == "data" || lower == "logs" || lower == "log" {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(dir, path)
			if relErr == nil && rel != "." && len(strings.Split(rel, string(os.PathSeparator))) > 3 {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(info.Name(), "Dockerfile") {
			return nil
		}
		for _, img := range parseDockerfileFromImages(path) {
			if _, ok := seen[img]; ok {
				continue
			}
			seen[img] = struct{}{}
			out = append(out, img)
		}
		return nil
	})
	return out
}

func parseDockerfileFromImages(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "FROM") {
			continue
		}
		i := 1
		if strings.HasPrefix(fields[i], "--") {
			i++
			if i >= len(fields) {
				continue
			}
		}
		img := fields[i]
		if img == "" || strings.EqualFold(img, "scratch") || strings.Contains(img, "$") {
			continue
		}
		out = append(out, img)
	}
	return out
}
