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
	res, err := listFS(dir, "sql")
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
