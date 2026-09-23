package db

import "testing"

func TestParseSwitchDB(t *testing.T) {
	kind, name := parseSwitchDB(`\c wpg_waterwork`)
	if kind != "connect" || name != "wpg_waterwork" {
		t.Fatalf("\\c: %s %s", kind, name)
	}
	kind, name = parseSwitchDB(`\connect "wpg_waterwork"`)
	if kind != "connect" || name != "wpg_waterwork" {
		t.Fatalf("\\connect: %s %s", kind, name)
	}
	kind, name = parseSwitchDB(`CREATE DATABASE wpg_waterwork WITH OWNER = wpg;`)
	if kind != "createdb" || name != "wpg_waterwork" {
		t.Fatalf("createdb: %s %s", kind, name)
	}
	kind, name = parseSwitchDB(`USE wpg_biz`)
	if kind != "use" || name != "wpg_biz" {
		t.Fatalf("use: %s %s", kind, name)
	}
}

func TestInferDBFromSQL(t *testing.T) {
	src := `
-- quartz
\c wpg_waterwork
CREATE TABLE qrtz_job_details (id int);
`
	if got := inferDBFromSQL(src); got != "wpg_waterwork" {
		t.Fatalf("got %q", got)
	}
	src = `CREATE DATABASE IF NOT EXISTS wpg_waterwork;
CREATE TABLE t(id int);`
	if got := inferDBFromSQL(src); got != "wpg_waterwork" {
		t.Fatalf("createdb infer %q", got)
	}
}

func TestInferDBFromFilename(t *testing.T) {
	got := inferDBFromFilename("/workspace/waterwork-4.1.1-pg/wpg_waterwork_quartz_pg.sql")
	if got != "wpg_waterwork" {
		t.Fatalf("got %q", got)
	}
	if inferDBFromFilename("init.sql") != "" {
		t.Fatal("init.sql should not infer")
	}
}

func TestParseCreateTable(t *testing.T) {
	if got := parseCreateTable(`CREATE TABLE IF NOT EXISTS qrtz_job_details (`); got != "qrtz_job_details" {
		t.Fatalf("got %q", got)
	}
}

func TestMarkPsqlMetaLines(t *testing.T) {
	src := markPsqlMetaLines("\\c wpg_waterwork\nCREATE TABLE t(id int);")
	stmts := splitSQL(src)
	if len(stmts) < 2 {
		t.Fatalf("stmts %#v", stmts)
	}
	if kind, name := parseSwitchDB(stmts[0]); kind != "connect" || name != "wpg_waterwork" {
		t.Fatalf("first %q", stmts[0])
	}
}
