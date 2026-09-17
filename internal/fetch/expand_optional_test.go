package fetch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandArchivesOptional_empty(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "html"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := ExpandArchivesOptional(filepath.Join(dir, "html"))
	if err != nil {
		t.Fatalf("optional expand should not error: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
}
