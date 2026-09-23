package firewall

import "testing"

func TestUniquePorts(t *testing.T) {
	got := uniquePorts([]int{80, 443, 80, 0, -1, 443})
	if len(got) != 2 || got[0] != 80 || got[1] != 443 {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestSSHPort(t *testing.T) {
	if SSHPort != 22 {
		t.Fatalf("SSHPort=%d", SSHPort)
	}
	if UIPort != 9527 {
		t.Fatalf("UIPort=%d", UIPort)
	}
	got := uniquePorts(append(ProtectPorts(), 22, 3306, 9527))
	if len(got) != 3 || got[0] != 22 || got[1] != 9527 || got[2] != 3306 {
		t.Fatalf("protect+extra: %v", got)
	}
}

func TestParseListenPort(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"0.0.0.0:9527", 9527},
		{"127.0.0.1:9527", 9527},
		{":8080", 8080},
		{"9527", 9527},
		{"", 0},
		{"bad", 0},
		{"0.0.0.0:0", 0},
	}
	for _, c := range cases {
		if got := ParseListenPort(c.in); got != c.want {
			t.Fatalf("ParseListenPort(%q)=%d want %d", c.in, got, c.want)
		}
	}
}
