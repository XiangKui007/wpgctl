package sshx

import (
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestRemoteDirFor(t *testing.T) {
	cases := map[string]string{
		"/workspace/middleware/redis":  "/workspace/middleware/redis",
		"/workspace/middleware/redis/": "/workspace/middleware/redis",
		`D:\pkg\middleware\redis`:      RemoteStageRoot + "/redis",
		"relative/dir":                 RemoteStageRoot + "/dir",
	}
	for in, want := range cases {
		if got := RemoteDirFor(in); got != want {
			t.Errorf("RemoteDirFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrivileged(t *testing.T) {
	root := &Session{Node: config.Node{SSH: config.SSHAuth{User: "root"}}}
	if cmd, stdin := root.Privileged("ls"); cmd != "ls" || stdin != "" {
		t.Fatalf("root should not sudo: %q %q", cmd, stdin)
	}
	user := &Session{Node: config.Node{SSH: config.SSHAuth{User: "ops"}}, password: "pw"}
	cmd, stdin := user.Privileged("ls")
	if cmd != "sudo -S -p '' ls" || stdin != "pw\n" {
		t.Fatalf("non-root with password: %q %q", cmd, stdin)
	}
	nopw := &Session{Node: config.Node{SSH: config.SSHAuth{User: "ops"}}}
	if cmd, _ := nopw.Privileged("ls"); cmd != "sudo -n ls" {
		t.Fatalf("non-root without password: %q", cmd)
	}
}

func TestHumanSize(t *testing.T) {
	if got := humanSize(512); got != "512 B" {
		t.Fatalf("got %q", got)
	}
	if got := humanSize(3 * 1024 * 1024); got != "3.0 MB" {
		t.Fatalf("got %q", got)
	}
}

func TestSkipSyncRel(t *testing.T) {
	if !SkipSyncRel("data", true) || !SkipSyncRel("logs", true) {
		t.Fatal("data/logs must always skip")
	}
	if SkipSyncRel("html/index.html", false) {
		t.Fatal("files must not skip by name")
	}
	if !SkipSyncRel("html", true) || !SkipSyncRel("frontend", true) || !SkipSyncRel("dist", true) {
		t.Fatal("frontend dirs must never SSH-sync")
	}
	if SkipSyncRel("conf", true) || SkipSyncRel("compose.yaml", false) {
		t.Fatal("nginx conf / compose must still sync")
	}
}
