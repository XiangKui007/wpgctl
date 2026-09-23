package fetch

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestExpandArchives_zipAndTarZip(t *testing.T) {
	root := t.TempDir()

	// middleware.zip -> middleware/middleware/readme.txt
	content := filepath.Join(root, "zipcontent")
	_ = os.MkdirAll(filepath.Join(content, "middleware"), 0o755)
	_ = os.WriteFile(filepath.Join(content, "middleware", "readme.txt"), []byte("ok"), 0o644)
	mwZip := filepath.Join(root, "middleware.zip")
	if err := zipDir(mwZip, content, ""); err != nil {
		t.Fatal(err)
	}

	// mysql.tar.zip -> fake tar payload
	tarZip := filepath.Join(root, "mysql.tar.zip")
	if err := writeTarZip(tarZip, []byte("fake-tar-content")); err != nil {
		t.Fatal(err)
	}

	res, err := ExpandArchives(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.ZipExtracted != 1 {
		t.Fatalf("zip extracted=%d want 1", res.ZipExtracted)
	}
	if res.TarUnwrapped != 1 {
		t.Fatalf("tar unwrapped=%d want 1", res.TarUnwrapped)
	}
	wantReadme := filepath.Join(root, "middleware", "middleware", "readme.txt")
	if !fileExists(wantReadme) {
		t.Fatalf("expected %s, steps=%v", wantReadme, res.Steps)
	}
	if !fileExists(filepath.Join(root, "mysql.tar")) {
		t.Fatal("mysql.tar missing")
	}
	if len(res.TarFiles) == 0 {
		t.Fatal("expected tar in result list")
	}
}

func TestExpand_oneFileLeavesSibling(t *testing.T) {
	root := t.TempDir()
	content := filepath.Join(root, "a")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(content, "x.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	azip := filepath.Join(root, "alpha.zip")
	bzip := filepath.Join(root, "beta.zip")
	if err := zipDir(azip, content, ""); err != nil {
		t.Fatal(err)
	}
	if err := zipDir(bzip, content, ""); err != nil {
		t.Fatal(err)
	}
	res, err := Expand(ExpandOptions{File: azip})
	if err != nil {
		t.Fatal(err)
	}
	if res.ZipExtracted != 1 {
		t.Fatalf("extracted=%d", res.ZipExtracted)
	}
	if !fileExists(filepath.Join(root, "alpha", "x.txt")) {
		t.Fatal("alpha not extracted")
	}
	if fileExists(filepath.Join(root, "beta", "x.txt")) {
		t.Fatal("beta should stay zipped")
	}
}

func TestArchiveExpanded(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "pkg.zip")
	if err := os.WriteFile(zipPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ArchiveExpanded(zipPath) {
		t.Fatal("empty dest should not count as expanded")
	}
	if err := os.Mkdir(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "f"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ArchiveExpanded(zipPath) {
		t.Fatal("pkg/ with files should be expanded")
	}
}

func zipDir(destZip, srcDir, prefix string) error {
	f, err := os.Create(destZip)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(srcDir, path)
		name := filepath.ToSlash(filepath.Join(prefix, rel))
		zf, err := w.Create(name)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = zf.Write(data)
		return err
	})
	if err != nil {
		w.Close()
		f.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return f.Close()
}

func writeTarZip(path string, tarBody []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	zf, err := w.Create("mysql5.7.44.tar")
	if err != nil {
		return err
	}
	if _, err := zf.Write(tarBody); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return f.Close()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestOverallPercent(t *testing.T) {
	if got := overallPercent(0, 1, 0, 0, 0, 0); got != 0 {
		t.Fatalf("start=%d", got)
	}
	if got := overallPercent(0, 1, 50, 100, 0, 0); got != 50 {
		t.Fatalf("half file=%d", got)
	}
	if got := overallPercent(1, 2, 0, 0, 0, 0); got != 50 {
		t.Fatalf("one of two=%d", got)
	}
	if got := overallPercent(1, 1, 0, 0, 0, 0); got != 100 {
		t.Fatalf("done=%d", got)
	}
}

func TestExpand_progressReportsInsideZip(t *testing.T) {
	root := t.TempDir()
	content := filepath.Join(root, "src")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := os.WriteFile(filepath.Join(content, fmt.Sprintf("f%d.bin", i)), bytes.Repeat([]byte{'x'}, 32*1024), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	zipPath := filepath.Join(root, "bulk.zip")
	if err := zipDir(zipPath, content, ""); err != nil {
		t.Fatal(err)
	}
	var seen []ExpandProgress
	res, err := Expand(ExpandOptions{
		File: zipPath,
		Progress: func(p ExpandProgress) {
			seen = append(seen, p)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ZipExtracted != 1 {
		t.Fatalf("extracted=%d", res.ZipExtracted)
	}
	if len(seen) < 3 {
		t.Fatalf("want intra-zip ticks, got %d %+v", len(seen), seen)
	}
	var maxPct int
	var sawBytes bool
	for _, p := range seen {
		if p.Percent < 0 || p.Percent > 100 {
			t.Fatalf("bad percent %+v", p)
		}
		if p.Percent > maxPct {
			maxPct = p.Percent
		}
		if p.BytesTotal > 0 && p.BytesDone > 0 {
			sawBytes = true
		}
	}
	if maxPct < 50 {
		t.Fatalf("percent never moved, last=%+v", seen[len(seen)-1])
	}
	if !sawBytes {
		t.Fatalf("expected byte progress, last=%+v", seen[len(seen)-1])
	}
	if seen[len(seen)-1].Percent != 100 {
		t.Fatalf("last percent=%d", seen[len(seen)-1].Percent)
	}
}

func TestExpand_progressReportsMultipleZips(t *testing.T) {
	root := t.TempDir()
	smallDir := filepath.Join(root, "s")
	largeDir := filepath.Join(root, "l")
	if err := os.MkdirAll(smallDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(largeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(smallDir, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := os.WriteFile(filepath.Join(largeDir, fmt.Sprintf("b%d.bin", i)), bytes.Repeat([]byte{'y'}, 32*1024), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := zipDir(filepath.Join(root, "small.zip"), smallDir, ""); err != nil {
		t.Fatal(err)
	}
	if err := zipDir(filepath.Join(root, "large.zip"), largeDir, ""); err != nil {
		t.Fatal(err)
	}

	var seen []ExpandProgress
	res, err := Expand(ExpandOptions{
		Root: root,
		Progress: func(p ExpandProgress) {
			seen = append(seen, p)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ZipExtracted != 2 {
		t.Fatalf("extracted=%d want 2", res.ZipExtracted)
	}
	if len(seen) < 4 {
		t.Fatalf("want multi-zip ticks, got %d %+v", len(seen), seen)
	}
	var maxTotal, lastPct int
	names := map[string]bool{}
	for _, p := range seen {
		if p.Total > maxTotal {
			maxTotal = p.Total
		}
		if p.Current != "" {
			names[p.Current] = true
		}
		if p.Percent < lastPct {
			t.Fatalf("percent went backwards %d → %d", lastPct, p.Percent)
		}
		lastPct = p.Percent
	}
	if maxTotal < 2 {
		t.Fatalf("total never reached 2, last=%+v", seen[len(seen)-1])
	}
	if !names["small.zip"] || !names["large.zip"] {
		t.Fatalf("want both zip names in progress, got %v", names)
	}
	if seen[len(seen)-1].Percent != 100 {
		t.Fatalf("last percent=%d", seen[len(seen)-1].Percent)
	}
}

func TestUnwrapTarZipIfNeeded(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "java8.tar.zip")
	if err := writeTarZip(zipPath, []byte("java8-tar")); err != nil {
		t.Fatal(err)
	}
	dest, err := UnwrapTarZipIfNeeded(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "java8.tar")
	if dest != want {
		t.Fatalf("got %q want %q", dest, want)
	}
	if !fileExists(want) {
		t.Fatal("java8.tar not created")
	}
	again, err := UnwrapTarZipIfNeeded(zipPath)
	if err != nil || again != want {
		t.Fatalf("idempotent: %v %q", err, again)
	}
}
