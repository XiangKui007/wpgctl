package firewall

import "testing"

func TestParseFirewalldListPorts(t *testing.T) {
	got := ParseFirewalldListPorts("22/tcp 8848/tcp 9092/udp")
	if len(got) != 3 || got[0].Port != 22 || got[1].Spec != "8848/tcp" || got[2].Proto != "udp" {
		t.Fatalf("%+v", got)
	}
}

func TestParseFirewalldServices(t *testing.T) {
	got := ParseFirewalldServices("ssh dhcpv6-client")
	if len(got) != 2 || got[0].Kind != "service" || got[0].Spec != "ssh" {
		t.Fatalf("%+v", got)
	}
}

func TestParseUfwStatus(t *testing.T) {
	raw := `Status: active

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW       Anywhere
8848/tcp                   ALLOW       Anywhere
22/tcp (v6)                ALLOW       Anywhere (v6)
`
	got := ParseUfwStatus(raw)
	if len(got) < 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Port != 22 || got[1].Port != 8848 {
		t.Fatalf("%+v", got)
	}
}

func TestMergeOpenPorts(t *testing.T) {
	a := ParseFirewalldListPorts("22/tcp 8848/tcp")
	b := ParseFirewalldServices("ssh ssh")
	got := MergeOpenPorts(a, b)
	if len(got) != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestParsePortRangeIgnoredAsSingleStart(t *testing.T) {
	got := ParseFirewalldListPorts("10000-10010/tcp")
	if len(got) != 1 || got[0].Port != 10000 {
		t.Fatalf("%+v", got)
	}
}
