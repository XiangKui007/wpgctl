package moduledeploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsBaseImageArchiveName(t *testing.T) {
	yes := []string{"java8.tar", "JAVA8.TAR", "java-8.tar", "java_8.tar", "jdk8.tar", "openjdk-8.tar", "java8.tar.zip"}
	for _, n := range yes {
		if !isBaseImageArchiveName(n) {
			t.Fatalf("want base image: %s", n)
		}
	}
	no := []string{"waterwork-center.tar", "nacos.tar", "readme.txt", "java11.tar"}
	for _, n := range no {
		if isBaseImageArchiveName(n) {
			t.Fatalf("did not want base image: %s", n)
		}
	}
}

func TestInferTagsFromBaseTar(t *testing.T) {
	got := inferTagsFromBaseTar("java8.tar")
	if len(got) != 1 || got[0] != "java:8" {
		t.Fatalf("got %v", got)
	}
}

func TestParseDockerfileFromImages(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Dockerfile")
	content := "# comment\nFROM java:8\nCOPY app.jar /app.jar\nFROM --platform=linux/amd64 java:8 AS runtime\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	imgs := parseDockerfileFromImages(p)
	if len(imgs) != 2 || imgs[0] != "java:8" {
		t.Fatalf("got %v", imgs)
	}
}

type fakeImages struct {
	have   map[string]bool
	loaded []string
	tags   []string
}

func (f *fakeImages) ImageExists(image string) bool { return f.have[image] }
func (f *fakeImages) LoadImage(tarPath string) ([]string, error) {
	f.loaded = append(f.loaded, filepath.Base(tarPath))
	f.have["loaded:from-tar"] = true
	return []string{"loaded:from-tar"}, nil
}
func (f *fakeImages) TagImage(src, dest string) error {
	f.tags = append(f.tags, src+"->"+dest)
	f.have[dest] = true
	return nil
}

func TestLoadBaseImagesForBuild_loadAndTag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "java8.tar"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM java:8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &fakeImages{have: map[string]bool{}}
	steps, err := loadBaseImagesForBuild([]string{dir}, dir, d)
	if err != nil {
		t.Fatalf("err %v steps %v", err, steps)
	}
	if len(d.loaded) != 1 || d.loaded[0] != "java8.tar" {
		t.Fatalf("loaded %v", d.loaded)
	}
	if !d.have["java:8"] {
		t.Fatalf("java:8 not tagged, tags=%v steps=%v", d.tags, steps)
	}
}

func TestLoadBaseImagesForBuild_alreadyLocal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM java:8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &fakeImages{have: map[string]bool{"java:8": true}}
	steps, err := loadBaseImagesForBuild([]string{dir}, dir, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.loaded) != 0 {
		t.Fatalf("should skip load, got %v", d.loaded)
	}
	if len(steps) == 0 || !strings.Contains(steps[0], "已在本地") {
		t.Fatalf("steps %v", steps)
	}
}

func TestLoadBaseImagesForBuild_missingTar(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM java:8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := &fakeImages{have: map[string]bool{}}
	_, err := loadBaseImagesForBuild([]string{dir}, dir, d)
	if err == nil || !strings.Contains(err.Error(), "java8.tar") {
		t.Fatalf("want java8.tar hint, got %v", err)
	}
}

func TestFindBaseImageTars_nestedParent(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "waterwork-4.1.1-pg")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	tar := filepath.Join(root, "java8.tar")
	if err := os.WriteFile(tar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, _, err := collectBaseImageTars([]string{inner})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("got %v", found)
	}
}
