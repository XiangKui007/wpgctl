package status

import (
	"strings"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
	dockerx "github.com/wpg/wpgctl/internal/docker"
)

func TestAnnotateNode_CopiesIdentity(t *testing.T) {
	list := annotateNode([]dockerx.ComposeService{{Name: "nacos"}}, "node2", "192.0.2.8", false)
	if len(list) != 1 {
		t.Fatalf("len %d", len(list))
	}
	if list[0].Node != "node2" || list[0].NodeIP != "192.0.2.8" || list[0].Local {
		t.Fatalf("got %+v", list[0])
	}
}

func TestRemoteNodes_SkipsLoopbackAndDedupes(t *testing.T) {
	site := &config.SiteConfig{}
	site.Nodes = []config.Node{
		{Name: "n1", IP: "127.0.0.1"},
		{Name: "n2", IP: "192.0.2.8"},
		{Name: "n2b", IP: "192.0.2.8"},
		{Name: "empty", IP: ""},
	}
	got := remoteNodes(site)
	if len(got) != 1 || got[0].IP != "192.0.2.8" {
		t.Fatalf("got %+v", got)
	}
}

func TestShellSingleQuote_RejectsInjection(t *testing.T) {
	if _, err := shellSingleQuote("/ok/path"); err != nil {
		t.Fatal(err)
	}
	if _, err := shellSingleQuote("relative"); err == nil {
		t.Fatal("want relative rejected")
	}
	if _, err := shellSingleQuote("/tmp/foo; rm -rf /"); err == nil {
		t.Fatal("want metachar rejected")
	}
}

func TestFindNodeByIP(t *testing.T) {
	site := &config.SiteConfig{}
	site.Nodes = []config.Node{{Name: "n2", IP: "192.0.2.8"}}
	n, ok := findNodeByIP(site, "192.0.2.8")
	if !ok || n.Name != "n2" {
		t.Fatalf("got %+v %v", n, ok)
	}
	if _, ok := findNodeByIP(site, "10.0.0.1"); ok {
		t.Fatal("missing ip should fail")
	}
}

func TestSnapshotFromList_ErrorKeepsCounts(t *testing.T) {
	snap := snapshotFromList("n2", "192.0.2.8", false, nil, "SSH 失败")
	if snap.Total != 0 || snap.Error != "SSH 失败" || snap.Local {
		t.Fatalf("%+v", snap)
	}
}

func TestParseDockerPsJSON_SkipsSSHNoise(t *testing.T) {
	out := "sudo: a password is required\n{\"ID\":\"abc\",\"Names\":\"web\",\"State\":\"running\",\"Status\":\"Up 1 second\",\"Image\":\"nginx\"}\n"
	list, err := dockerx.ParseDockerPsJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "web" {
		t.Fatalf("got %+v", list)
	}
}

func TestRunClusterAction_UnknownAction(t *testing.T) {
	if err := RunClusterAction(ActionOptions{Action: "explode", Name: "nginx"}); err == nil {
		t.Fatal("expected unknown action")
	}
}

func TestRunClusterAction_RemoteMissingNode(t *testing.T) {
	err := RunClusterAction(ActionOptions{
		Action: "stop",
		Name:   "nginx",
		NodeIP: "192.0.2.8",
		Site:   &config.SiteConfig{},
	})
	if err == nil || !strings.Contains(err.Error(), "没有节点") {
		t.Fatalf("got %v", err)
	}
}
