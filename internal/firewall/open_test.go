package firewall

import "testing"

func TestUniquePorts(t *testing.T) {
	got := uniquePorts([]int{80, 443, 80, 0, -1, 443})
	if len(got) != 2 || got[0] != 80 || got[1] != 443 {
		t.Fatalf("unexpected: %v", got)
	}
}
