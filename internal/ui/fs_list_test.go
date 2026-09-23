package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListFS_SQLMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("select 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := listFS(dir, "sql", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range res.Entries {
		names = append(names, e.Name)
		if !e.IsDir && e.Name != "a.sql" {
			t.Fatalf("unexpected file %s", e.Name)
		}
	}
	if len(names) != 2 {
		t.Fatalf("want dir+sql, got %v", names)
	}
}

func TestListFS_DirModeArchivesAreNotSelectedNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pkg.zip"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "svc"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := listFS(dir, "dir", "")
	if err != nil {
		t.Fatal(err)
	}
	var zipEntry *FSEntry
	for i := range res.Entries {
		e := &res.Entries[i]
		if e.IsArchive {
			zipEntry = e
		}
	}
	if zipEntry == nil || zipEntry.Name != "pkg.zip" || zipEntry.ArchiveKind != "zip" {
		t.Fatalf("archive: %+v", zipEntry)
	}
	if zipEntry.AlreadyExpanded {
		t.Fatal("empty dest is not expanded")
	}
}

func TestListFS_DirModeShowsRegularFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := listFS(dir, "dir", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]FSEntry{}
	for _, e := range res.Entries {
		got[e.Name] = e
	}
	if _, ok := got["data"]; !ok || !got["data"].IsDir {
		t.Fatalf("want dir data, got %+v", res.Entries)
	}
	yml, ok := got["docker-compose.yml"]
	if !ok || yml.IsDir || yml.IsArchive {
		t.Fatalf("want regular file docker-compose.yml, got %+v", res.Entries)
	}
	env, ok := got[".env"]
	if !ok || env.IsDir || env.IsArchive {
		t.Fatalf("want .env listed in dir mode, got %+v", res.Entries)
	}
	if _, ok := got[".git"]; ok {
		t.Fatalf(".git should stay hidden, got %+v", res.Entries)
	}
}
