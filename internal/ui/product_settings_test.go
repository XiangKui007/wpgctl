package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneMissingFieldPaths(t *testing.T) {
	dir := t.TempDir()
	live := filepath.Join(dir, "middle")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, "gone-pkg", "middleware")
	fp := map[string]string{
		"middlewareRoot":     gone,
		"platformRoot":       live,
		"gatewayIP":          "10.0.0.1",
		"form.dockerPackage": filepath.Join(dir, "docker_missing"),
	}
	got := pruneMissingFieldPaths(fp)
	if got == nil {
		t.Fatal("expected prune")
	}
	if got["middlewareRoot"] != "" {
		t.Fatalf("missing dir should drop, got %q", got["middlewareRoot"])
	}
	if got["platformRoot"] != live {
		t.Fatalf("existing dir kept, got %q", got["platformRoot"])
	}
	if got["gatewayIP"] != "10.0.0.1" {
		t.Fatalf("IP must keep, got %q", got["gatewayIP"])
	}
	if _, ok := got["form.dockerPackage"]; ok {
		t.Fatal("missing docker package path should drop")
	}
}

func TestPruneMissingFieldPathsUnchanged(t *testing.T) {
	dir := t.TempDir()
	fp := map[string]string{"middlewareRoot": dir, "gatewayIP": "1.1.1.1"}
	if got := pruneMissingFieldPaths(fp); got != nil {
		t.Fatalf("no change expected, got %#v", got)
	}
}
