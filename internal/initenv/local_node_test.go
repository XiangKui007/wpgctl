package initenv

import (
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestRemoteNodesSkipsLocal(t *testing.T) {
	info := config.Node{Name: "local", IP: "127.0.0.1", SSH: config.SSHAuth{User: "root", Port: 22}, Roles: []string{"middleware"}}
	remote := config.Node{Name: "remote", IP: "192.0.2.50", SSH: config.SSHAuth{User: "root", Port: 22}, Roles: []string{"database"}}
	out := remoteNodes([]config.Node{info, remote})
	if len(out) != 1 || out[0].Name != "remote" {
		t.Fatalf("expected only remote node, got %+v", out)
	}
}
