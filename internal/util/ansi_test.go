package util

import "testing"

func TestStripANSI(t *testing.T) {
	want := "2026-09-20 14:29:25.740[2.70_8848-intergrate] INFO get"
	in := "\x1b[36m2026-09-20 14:29:25.740\x1b[33m[2.70_8848-intergrate]\x1b[39m \x1b[34mINFO\x1b[39m get"
	if got := StripANSI(in); got != want {
		t.Fatalf("csi got %q", got)
	}
	orphan := "[36m2026-09-20 14:29:25.740[33m[2.70_8848-intergrate][39m [34mINFO[39m get"
	if got := StripANSI(orphan); got != want {
		t.Fatalf("orphan got %q", got)
	}
	plain := "hello [not-color]"
	if StripANSI(plain) != plain {
		t.Fatal("plain text changed")
	}
}
