package initenv

import (
	"path/filepath"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestDockerDataRoot(t *testing.T) {
	site := &config.SiteConfig{
		Paths: config.PathsConfig{Workspace: "/workspace/waterwork"},
	}
	got := DockerDataRoot(site)
	want := filepath.Join("/workspace", "docker_data", "docker", "lib")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
