package ui

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"archive/zip"
	"net/http"
	"net/http/httptest"
)

func TestHandleFSExpandReturnsJob(t *testing.T) {
	root := t.TempDir()
	content := filepath.Join(root, "src")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(content, "a.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(root, "pkg.zip")
	if err := writeTestZip(zipPath, filepath.Join(content, "a.txt"), "a.txt"); err != nil {
		t.Fatal(err)
	}

	s := &Server{jobs: map[string]*Job{}}
	body, _ := json.Marshal(map[string]string{"file": zipPath})
	req := httptest.NewRequest(http.MethodPost, "/api/fs/expand", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleFSExpand(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var job Job
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Fatal("missing job id")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		cur := s.jobs[job.ID]
		st := ""
		var msg string
		if cur != nil {
			st = cur.Status
			msg = cur.Message
		}
		s.mu.Unlock()
		if st == "ok" {
			if !fileExistsUI(filepath.Join(root, "pkg", "a.txt")) {
				t.Fatal("zip not extracted")
			}
			return
		}
		if st == "fail" {
			t.Fatalf("job failed: %s", msg)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not finish")
}

func writeTestZip(dest, srcFile, name string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	zf, err := w.Create(name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(srcFile)
	if err != nil {
		return err
	}
	if _, err := zf.Write(data); err != nil {
		return err
	}
	return w.Close()
}

func fileExistsUI(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
