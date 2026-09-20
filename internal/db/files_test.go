package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wpg/wpgctl/internal/config"
)

func TestSplitSQL_IgnoresSemicolonInQuotesAndComments(t *testing.T) {
	src := `
CREATE TABLE t (id int); -- note; still comment
INSERT INTO t VALUES ('a;b');
/* block ; comment */
SELECT 1
`
	got := splitSQL(src)
	if len(got) != 3 {
		t.Fatalf("got %d stmts: %#v", len(got), got)
	}
	if !strings.Contains(got[1], "'a;b'") {
		t.Fatalf("quote split broken: %q", got[1])
	}
}

func TestSplitSQL_DollarQuote(t *testing.T) {
	src := `CREATE FUNCTION f() RETURNS text AS $$ SELECT 'x;y'; $$ LANGUAGE sql; SELECT 1;`
	got := splitSQL(src)
	if len(got) != 2 {
		t.Fatalf("got %d stmts: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "$$") {
		t.Fatalf("dollar quote lost: %q", got[0])
	}
}

func TestNormalizeSQLFiles_RejectsNonSQL(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := normalizeSQLFiles([]string{p})
	if err == nil || !strings.Contains(err.Error(), ".sql") {
		t.Fatalf("want .sql error, got %v", err)
	}
}

func TestNormalizeSQLFiles_AcceptsSQL(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "init.sql")
	if err := os.WriteFile(p, []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := normalizeSQLFiles([]string{p, p, "  "})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("dedupe: %#v", got)
	}
}

func TestResolveDriver_SkipMySQL(t *testing.T) {
	site := &config.SiteConfig{}
	site.Middleware.MySQL.Disabled = true
	site.Middleware.PgSQL.Host = "127.0.0.1"
	_, _, err := resolveDriver(site, "mysql")
	if err == nil {
		t.Fatal("expected mysql skipped")
	}
	d, conn, err := resolveDriver(site, "")
	if err != nil {
		t.Fatal(err)
	}
	if d != DriverPgSQL {
		t.Fatalf("driver %s", d)
	}
	if conn.Port != 5433 {
		t.Fatalf("default pgsql port %d", conn.Port)
	}
}

func TestResolveDriver_EmptyMySQLHost(t *testing.T) {
	site := &config.SiteConfig{}
	_, _, err := resolveDriver(site, "mysql")
	if err == nil || !strings.Contains(err.Error(), "mysql.host") {
		t.Fatalf("want host error, got %v", err)
	}
}

func TestApplyFiles_RequiresFiles(t *testing.T) {
	_, err := ApplyFiles(FileOptions{Site: &config.SiteConfig{}})
	if err == nil || !strings.Contains(err.Error(), "sql") {
		t.Fatalf("got %v", err)
	}
}
