package dockerx

import "testing"

func TestParseLoadedImages(t *testing.T) {
	out := `Loaded image: 10.10.102.75/wpg_release/platform-gateway-web:v4.8.0
Loaded image ID: sha256:abc123
`
	refs := parseLoadedImages(out)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %v", refs)
	}
	if refs[0] != "10.10.102.75/wpg_release/platform-gateway-web:v4.8.0" {
		t.Fatalf("unexpected ref: %q", refs[0])
	}
}

func TestDiffImageRefs(t *testing.T) {
	before := []string{"a:1", "b:2"}
	after := []string{"a:1", "b:2", "c:3"}
	added := diffImageRefs(before, after)
	if len(added) != 1 || added[0] != "c:3" {
		t.Fatalf("unexpected diff: %v", added)
	}
}

func TestParseComposePsStandalone(t *testing.T) {
	out := `      Name                     Command               State    Ports
-----------------------------------------------------------------------
mysql   docker-entrypoint.sh mysqld   Up      0.0.0.0:3306->3306/tcp
`
	list, err := parseComposePsStandalone(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 service, got %d", len(list))
	}
	if list[0].Name != "mysql" {
		t.Fatalf("unexpected name: %q", list[0].Name)
	}
}
