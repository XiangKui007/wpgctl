package moduledeploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncComposeImagesFromDocker(t *testing.T) {
	dir := t.TempDir()
	compose := filepath.Join(dir, "docker-compose.yml")
	content := `version: '3'
services:
  platform-gateway-web:
    image: 10.10.102.75/wpg_release/platform-gateway-web:v4.7.6
  wpg-auth-biz-web:
    image: 10.10.102.75/wpg_release/wpg-auth-biz-web:v4.7.6
`
	if err := os.WriteFile(compose, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	local := []string{
		"10.10.102.75/wpg_release/platform-gateway-web:v4.8.0",
		"10.10.102.75/wpg_release/wpg-auth-biz-web:v4.7.6",
	}
	prefer := []string{"10.10.102.75/wpg_release/platform-gateway-web:v4.8.0"}
	notes, err := syncComposeWithLocalImages(compose, local, prefer)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 patch note, got %v", notes)
	}
	out, _ := os.ReadFile(compose)
	got := string(out)
	if !strings.Contains(got, "platform-gateway-web:v4.8.0") {
		t.Fatalf("compose not patched:\n%s", got)
	}
}

func TestPickLocalImageRefPrefersLoaded(t *testing.T) {
	local := []string{
		"10.10.102.75/wpg_release/platform-gateway-web:v4.7.6",
		"10.10.102.75/wpg_release/platform-gateway-web:v4.8.0",
	}
	prefer := []string{"10.10.102.75/wpg_release/platform-gateway-web:v4.8.0"}
	got := pickLocalImageRef("10.10.102.75/wpg_release/platform-gateway-web:v4.7.6", local, prefer)
	if got != "10.10.102.75/wpg_release/platform-gateway-web:v4.8.0" {
		t.Fatalf("unexpected pick: %q", got)
	}
}

func TestImageRepoBase(t *testing.T) {
	if got := imageRepoBase("10.10.102.75/wpg_release/platform-gateway-web:v4.7.6"); got != "platform-gateway-web" {
		t.Fatalf("unexpected base: %q", got)
	}
}

// syncComposeWithLocalImages 供单测注入 docker images 列表。
func syncComposeWithLocalImages(composePath string, local, prefer []string) ([]string, error) {
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
		newRef := pickLocalImageRef(cur, local, prefer)
		if newRef == "" || newRef == cur {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + "image: " + newRef
		notes = append(notes, cur+" → "+newRef)
		changed = true
	}
	if !changed {
		return notes, nil
	}
	return notes, os.WriteFile(composePath, []byte(strings.Join(lines, "\n")), 0o644)
}
