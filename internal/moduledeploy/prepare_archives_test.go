package moduledeploy

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareModuleArchives_tarZip(t *testing.T) {
	dir := t.TempDir()
	tarZip := filepath.Join(dir, "public.tar.zip")
	payload := []byte("fake-tar-payload")
	zf, err := os.Create(tarZip)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)
	f, err := w.Create("public.tar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	steps, err := PrepareModuleArchives(dir)
	if err != nil {
		t.Fatalf("prepare: %v\nsteps: %v", err, steps)
	}
	tars, err := findImageTars(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tars) != 1 {
		t.Fatalf("expected 1 tar, got %v", tars)
	}
	pending, _ := findPendingTarZips(dir)
	if len(pending) != 0 {
		t.Fatalf("tar.zip should be unwrapped, still pending %v", pending)
	}
}
