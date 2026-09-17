package initenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeLegacyDockerService(t *testing.T) {
	dir := t.TempDir()
	svc := filepath.Join(dir, "docker.service")
	content := `[Service]
ExecStart=/usr/bin/dockerd --graph /workspace/docker_data/docker/lib/ --log-driver json-file --log-opt max-size=500m --log-opt max-file=2
`
	if err := os.WriteFile(svc, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := normalizeLegacyDockerService(svc, "/workspace/docker_data/docker/lib"); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(svc)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "--log-driver") || strings.Contains(got, "--log-opt") {
		t.Fatalf("expected log flags stripped, got:\n%s", got)
	}
	want := "ExecStart=/usr/bin/dockerd --graph /workspace/docker_data/docker/lib/"
	if !strings.Contains(got, want) {
		t.Fatalf("want %q in:\n%s", want, got)
	}
}

func TestIsLegacyDockerPackage(t *testing.T) {
	dir := t.TempDir()
	if IsLegacyDockerPackage(dir) {
		t.Fatal("empty dir should not match")
	}
	if err := os.WriteFile(filepath.Join(dir, "offline_install_docker.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsLegacyDockerPackage(dir) {
		t.Fatal("expected script dir to match")
	}
}
