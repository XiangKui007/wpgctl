package initenv

import (
	"path/filepath"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestDockerDataRoot(t *testing.T) {
	want := filepath.Join("/workspace", "docker_data", "docker", "lib")
	cases := []struct {
		name string
		ws   string
	}{
		{name: "工作簿根", ws: "/workspace"},
		{name: "工作簿根带斜杠", ws: "/workspace/"},
		{name: "旧子目录 waterwork", ws: "/workspace/waterwork"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DockerDataRoot(&config.SiteConfig{Paths: config.PathsConfig{Workspace: tc.ws}})
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

func TestDockerDataRoot_空配置走现场默认(t *testing.T) {
	if got := DockerDataRoot(nil); got != defaultDockerDataRoot {
		t.Fatalf("nil site got %q", got)
	}
	empty := &config.SiteConfig{}
	if got := DockerDataRoot(empty); got != defaultDockerDataRoot {
		t.Fatalf("empty workspace got %q", got)
	}
}
