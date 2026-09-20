package dockerx

import (
	"strings"
	"testing"
)

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
	ids := parseLoadedImageIDs(out)
	if len(ids) != 1 || ids[0] != "sha256:abc123" {
		t.Fatalf("unexpected ids: %v", ids)
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

func TestParseDockerPsLine(t *testing.T) {
	line := `{"ID":"abc123","Names":"nacos-nacos-1","Image":"nacos/nacos-server:v2.3","State":"running","Status":"Up 3 hours (healthy)","Ports":"0.0.0.0:8848->8848/tcp","CreatedAt":"2026-09-17 10:00:00 +0800 CST","Networks":"nacos_default","Labels":"com.docker.compose.project=nacos,com.docker.compose.service=nacos,com.docker.compose.project.working_dir=/workspace/middleware/nacos"}`
	s, err := parseDockerPsLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "nacos-nacos-1" || s.Service != "nacos" || s.Project != "nacos" {
		t.Fatalf("got %+v", s)
	}
	if s.Health != "healthy" {
		t.Fatalf("health %q", s.Health)
	}
	if s.Ports == "" || s.ComposeDir == "" {
		t.Fatalf("missing ports/dir: %+v", s)
	}
	if s.Created != "2026-09-17 10:00:00" {
		t.Fatalf("created %q", s.Created)
	}
	if s.Networks != "nacos_default" {
		t.Fatalf("networks %q", s.Networks)
	}
}

func TestParseComposePs_NumericCreatedAndPublishers(t *testing.T) {
	line := `{"ID":"def4567890ab","Name":"kafka-1","Service":"kafka","State":"running","Status":"Up 16 hours","Health":"","Image":"10.10.102.75/ops_dev/kafka:2.12-2.4.1","Created":1726639200,"Project":"middleware","Labels":{"com.docker.compose.project.working_dir":"/workspace/middleware/kafka"},"Networks":["middleware_default"],"Publishers":[{"URL":"0.0.0.0","TargetPort":9092,"PublishedPort":9092,"Protocol":"tcp"}]}`
	list, err := parseComposePs(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d", len(list))
	}
	s := list[0]
	if s.Service != "kafka" || s.ComposeDir != "/workspace/middleware/kafka" {
		t.Fatalf("got %+v", s)
	}
	if s.Created == "" {
		t.Fatalf("created empty: %+v", s)
	}
	if s.Networks != "middleware_default" {
		t.Fatalf("networks %q", s.Networks)
	}
	if !strings.Contains(s.Ports, "9092") {
		t.Fatalf("ports %q", s.Ports)
	}
	if s.ExitCode != nil {
		t.Fatalf("running should omit exit code: %+v", s.ExitCode)
	}
}

func TestParseComposePs_ExitedCode(t *testing.T) {
	line := `{"ID":"aa","Name":"nginx-1","Service":"nginx","State":"exited","Status":"Exited (137) 2 minutes ago","ExitCode":137,"Created":"2026-09-18 08:00:00 +0800 CST"}`
	list, err := parseComposePs(line)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ExitCode == nil || *list[0].ExitCode != 137 {
		t.Fatalf("exit %+v", list[0].ExitCode)
	}
	if list[0].Created != "2026-09-18 08:00:00" {
		t.Fatalf("created %q", list[0].Created)
	}
}

