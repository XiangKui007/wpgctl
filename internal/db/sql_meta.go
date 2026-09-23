package db

import (
	"path/filepath"
	"regexp"
	"strings"
)

// 本文件识别现场 .sql 里的切库指令与文件名推断。
// 市政水厂脚本常写 CREATE DATABASE / \c，不跟着切库就会把表建进 postgres，业务库仍是空壳。

var (
	reCreateDB = regexp.MustCompile(`(?is)^\s*CREATE\s+DATABASE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + sqlIdentPat)
	reUseDB    = regexp.MustCompile(`(?is)^\s*USE\s+` + sqlIdentPat)
	rePsqlC    = regexp.MustCompile(`(?is)^\s*\\c(?:onnect)?\s+` + sqlIdentPat)
	reCreateTb = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:UNLOGGED\s+|TEMPORARY\s+|TEMP\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:[A-Za-z_][A-Za-z0-9_]*\.)?` + sqlIdentPat)
	reIdent    = regexp.MustCompile(`(?i)^[A-Za-z_][A-Za-z0-9_]*$`)
	reMetaLine = regexp.MustCompile(`(?m)^[ \t]*\\[A-Za-z]+[^\n]*`)
)

const sqlIdentPat = `(?:"([^"]+)"|` + "`" + `([^` + "`" + `]+)` + "`" + `|([A-Za-z_][A-Za-z0-9_]*))`

var filenameSkip = map[string]bool{
	"init": true, "schema": true, "ddl": true, "data": true,
	"quartz": true, "sql": true, "dump": true,
}

// markPsqlMetaLines 给单独一行的 \c / \connect 补分号，避免和下一句粘在一起。
func markPsqlMetaLines(src string) string {
	return reMetaLine.ReplaceAllStringFunc(src, func(s string) string {
		return strings.TrimRight(s, "; \t\r") + ";"
	})
}

func firstIdent(m []string) string {
	if len(m) < 2 {
		return ""
	}
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

// stripLeadingSQLComments 去掉语句开头的 -- 与 /* */ 注释。
// 现场脚本常在 \c / CREATE TABLE 上面写一行说明，不剥掉就会认不出首关键字。
func stripLeadingSQLComments(s string) string {
	t := strings.TrimSpace(s)
	for t != "" {
		if strings.HasPrefix(t, "--") {
			i := strings.IndexByte(t, '\n')
			if i < 0 {
				return ""
			}
			t = strings.TrimSpace(t[i+1:])
			continue
		}
		if strings.HasPrefix(t, "/*") {
			i := strings.Index(t, "*/")
			if i < 0 {
				return ""
			}
			t = strings.TrimSpace(t[i+2:])
			continue
		}
		return t
	}
	return ""
}

// parseSwitchDB 从单条语句识别目标库：\c、USE、CREATE DATABASE。
func parseSwitchDB(stmt string) (kind, name string) {
	s := stripLeadingSQLComments(stmt)
	if s == "" {
		return "", ""
	}
	if m := rePsqlC.FindStringSubmatch(s); len(m) > 0 {
		if n := firstIdent(m); n != "" {
			return "connect", n
		}
	}
	if m := reUseDB.FindStringSubmatch(s); len(m) > 0 {
		if n := firstIdent(m); n != "" {
			return "use", n
		}
	}
	if m := reCreateDB.FindStringSubmatch(s); len(m) > 0 {
		if n := firstIdent(m); n != "" {
			return "createdb", n
		}
	}
	return "", ""
}

func parseCreateTable(stmt string) string {
	m := reCreateTb.FindStringSubmatch(stripLeadingSQLComments(stmt))
	if len(m) == 0 {
		return ""
	}
	return firstIdent(m)
}

func isPsqlMeta(stmt string) bool {
	return strings.HasPrefix(stripLeadingSQLComments(stmt), `\`)
}

func validIdent(name string) bool {
	return reIdent.MatchString(strings.TrimSpace(name))
}

// inferDBFromSQL 从脚本正文取第一个切库目标。
func inferDBFromSQL(text string) string {
	prepared := markPsqlMetaLines(text)
	for _, stmt := range splitSQL(prepared) {
		if _, name := parseSwitchDB(stmt); name != "" && validIdent(name) {
			return name
		}
	}
	return ""
}

// inferDBFromFilename 从 wpg_waterwork_quartz_pg.sql 这类文件名收成库名。
func inferDBFromFilename(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	n := strings.TrimSpace(base)
	for {
		lower := strings.ToLower(n)
		cut := ""
		for _, suf := range []string{"_pg", "_pgsql", "_postgres", "_postgresql", "_mysql", "_mariadb", "_quartz", "_ddl", "_schema", "_init"} {
			if strings.HasSuffix(lower, suf) {
				cut = suf
				break
			}
		}
		if cut == "" {
			break
		}
		n = n[:len(n)-len(cut)]
	}
	n = strings.Trim(n, "_- ")
	if n == "" || filenameSkip[strings.ToLower(n)] || !validIdent(n) {
		return ""
	}
	return n
}

// stmtPreview 压成单行摘要写进日志；剥掉前置注释，避免整行只看到「-- 建表」。
func stmtPreview(stmt string, max int) string {
	body := stripLeadingSQLComments(stmt)
	if body == "" {
		body = stmt
	}
	s := strings.Join(strings.Fields(body), " ")
	if max <= 0 {
		max = 96
	}
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

func stmtKind(stmt string) string {
	body := stripLeadingSQLComments(stmt)
	u := strings.ToUpper(body)
	switch {
	case strings.HasPrefix(body, `\`):
		return "meta"
	case strings.HasPrefix(u, "CREATE DATABASE"):
		return "createdb"
	case strings.HasPrefix(u, "CREATE TABLE"):
		return "createtable"
	case strings.HasPrefix(u, "CREATE "):
		return "create"
	case strings.HasPrefix(u, "ALTER "):
		return "alter"
	case strings.HasPrefix(u, "DROP "):
		return "drop"
	case strings.HasPrefix(u, "INSERT "):
		return "insert"
	case strings.HasPrefix(u, "COPY "):
		return "copy"
	case strings.HasPrefix(u, "USE "), strings.HasPrefix(u, `\C`):
		return "switch"
	default:
		return "other"
	}
}

func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "already exists") ||
		strings.Contains(s, "duplicate database") ||
		strings.Contains(s, "database exists")
}
