package fetch

import (
	"archive/zip"
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
