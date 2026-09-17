package moduledeploy

import (
	"fmt"
	"os"
	"strings"

	dockerx "github.com/wpg/wpgctl/internal/docker"
)

// SyncComposeImagesFromDocker 以 docker images 中的实际 tag 为准，修正 compose 中不一致的 image 行。
// prefer 为本次 load 新增的 tag，同仓库名时优先采用。
func SyncComposeImagesFromDocker(composePath string, d *dockerx.Runner, prefer []string) ([]string, error) {
	if d == nil {
		return nil, fmt.Errorf("docker runner 不能为空")
	}
	local, err := d.ListLocalImages()
	if err != nil {
		return nil, fmt.Errorf("读取 docker images 失败: %w", err)
	}
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var notes []string
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "image:") {
			continue
		}
		cur := strings.TrimSpace(strings.TrimPrefix(trimmed, "image:"))
		cur = strings.Trim(cur, `"'`)
		if cur == "" || strings.Contains(cur, "${") {
			continue
		}
		newRef := pickLocalImageRef(cur, local, prefer)
		if newRef == "" || newRef == cur {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + "image: " + newRef
		notes = append(notes, fmt.Sprintf("%s → %s (docker images)", cur, newRef))
		changed = true
	}
	if !changed {
		return notes, nil
	}
	return notes, os.WriteFile(composePath, []byte(strings.Join(lines, "\n")), 0o644)
}

func pickLocalImageRef(composeImage string, local, prefer []string) string {
	base := imageRepoBase(composeImage)
	if base == "" {
		return ""
	}
	for _, ref := range prefer {
		if imageRepoBase(ref) == base {
			return ref
		}
	}
	for _, ref := range local {
		if imageRepoBase(ref) == base {
			return ref
		}
	}
	return ""
}

func imageRepoBase(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndex(ref, ":"); i > 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		ref = ref[i+1:]
	}
	return strings.ToLower(ref)
}
