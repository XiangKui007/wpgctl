package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdUnitContainsExecStart(t *testing.T) {
	got := systemdUnit("/opt/wpgctl/wpgctl", "0.0.0.0:9527", "/workspace/site.yaml", "/workspace")
	for _, want := range []string{
		"Description=wpgctl Web 控制台",
		"ExecStart=/opt/wpgctl/wpgctl ui --listen 0.0.0.0:9527 --site /workspace/site.yaml",
		"WorkingDirectory=/workspace",
		"EnvironmentFile=-/etc/wpgctl/ui.env",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("unit missing %q\n%s", want, got)
		}
	}
}

func TestSystemdQuoteSpaces(t *testing.T) {
	if got := systemdQuote(`/opt/wpg ctl/wpgctl`); got != `"/opt/wpg ctl/wpgctl"` {
		t.Fatalf("got %s", got)
	}
	if got := systemdQuote(`/opt/wpgctl/wpgctl`); got != `/opt/wpgctl/wpgctl` {
		t.Fatalf("plain path quoted: %s", got)
	}
}

func TestPidFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WPGCTL_HOME", dir)
	if err := writePID(4242); err != nil {
		t.Fatal(err)
	}
	got, err := readPID()
	if err != nil {
		t.Fatal(err)
	}
	if got != 4242 {
		t.Fatalf("pid=%d", got)
	}
	if pidPath() != filepath.Join(dir, "ui.pid") {
		t.Fatalf("pidPath=%s", pidPath())
	}
	if err := writeListenMeta("127.0.0.1:9527"); err != nil {
		t.Fatal(err)
	}
	if got := readListenMeta(); got != "127.0.0.1:9527" {
		t.Fatalf("listen=%s", got)
	}
}

func TestProcessAliveSelf(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("current process should be alive")
	}
	if processAlive(-1) || processAlive(0) {
		t.Fatal("invalid pid must be dead")
	}
}

func TestTailFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ui.log")
	if got := tailFile(p, 3); got != "(无日志)" {
		t.Fatalf("missing file: %s", got)
	}
	body := "a\nb\nc\nd\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := tailFile(p, 2)
	if got != "c\nd" {
		t.Fatalf("tail=%q", got)
	}
}
