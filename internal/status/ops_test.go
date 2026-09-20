package status

import "testing"

func TestRunActionRejectsBadName(t *testing.T) {
	if err := RunAction("stop", "foo; rm -rf /", ""); err == nil {
		t.Fatal("expected illegal name error")
	}
	if err := RunAction("nope", "nginx", ""); err == nil {
		t.Fatal("expected unknown action")
	}
}

func TestParseDockerPsJSON_inDockerPackage(t *testing.T) {
	// name validation only here; docker JSON parse lives in dockerx tests
	if !reSafeName.MatchString("nacos-nacos-1") {
		t.Fatal("expected compose-style name ok")
	}
	if reSafeName.MatchString("../etc") {
		t.Fatal("path traversal should fail")
	}
}
