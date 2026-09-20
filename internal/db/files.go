package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	_ "github.com/lib/pq"
	"github.com/wpg/wpgctl/internal/config"
)

const (
	// DriverMySQL 使用 site.yaml 的 middleware.mysql。
	DriverMySQL = "mysql"
	// DriverPgSQL 使用 site.yaml 的 middleware.pgsql。
	DriverPgSQL = "pgsql"
	maxSQLBytes = 50 << 20
)

// FileOptions 现场自选 SQL 文件执行（不强制 V{ver}_{seq}__ 命名）。
type FileOptions struct {
	Site     *config.SiteConfig
	Files    []string
	Driver   string // mysql / pgsql，空则按 MySQL.Disabled 推断
	Database string // 空：MySQL 不切库；PgSQL 连 postgres
	Log      func(string)
}

// ApplyFiles 按选择顺序执行 .sql。失败立即停止，已执行的不回滚。
func ApplyFiles(opts FileOptions) (*Result, error) {
	if opts.Site == nil {
		return nil, fmt.Errorf("site 不能为空")
	}
	log := opts.Log
	if log == nil {
		log = func(string) {}
	}
	files, err := normalizeSQLFiles(opts.Files)
	if err != nil {
		return nil, err
	}
	driver, conn, err := resolveDriver(opts.Site, opts.Driver)
	if err != nil {
		return nil, err
	}
	db, err := openDB(driver, conn, strings.TrimSpace(opts.Database))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败（%s %s:%d）: %w", driver, conn.Host, conn.Port, err)
	}
	log(fmt.Sprintf("已连接 %s %s:%d 库=%s", driver, conn.Host, conn.Port, displayDB(driver, opts.Database)))

	res := &Result{}
	for _, path := range files {
		base := filepath.Base(path)
		log("执行 " + path)
		if err := execSQLFile(db, driver, path); err != nil {
			return res, fmt.Errorf("执行 %s 失败: %w", base, err)
		}
		res.Applied = append(res.Applied, base)
		log("完成 " + base)
	}
	return res, nil
}

func displayDB(driver, database string) string {
	name := strings.TrimSpace(database)
	if name != "" {
		return name
	}
	if driver == DriverPgSQL {
		return "postgres"
	}
	return "（未指定库）"
}

func resolveDriver(site *config.SiteConfig, want string) (string, config.DBConn, error) {
	d := strings.ToLower(strings.TrimSpace(want))
	if d == "" {
		if site.Middleware.MySQL.Disabled {
			d = DriverPgSQL
		} else {
			d = DriverMySQL
		}
	}
	switch d {
	case DriverMySQL, "mariadb":
		if site.Middleware.MySQL.Disabled {
			return "", config.DBConn{}, fmt.Errorf("site.yaml 已跳过 MySQL，请改用 PostgreSQL")
		}
		c := site.Middleware.MySQL
		if strings.TrimSpace(c.Host) == "" {
			return "", config.DBConn{}, fmt.Errorf("middleware.mysql.host 为空")
		}
		if c.Port == 0 {
			c.Port = 3306
		}
		return DriverMySQL, c, nil
	case DriverPgSQL, "postgres", "postgresql":
		c := site.Middleware.PgSQL
		if c.Port == 0 {
			c.Port = 5433
		}
		if strings.TrimSpace(c.Host) == "" {
			return "", config.DBConn{}, fmt.Errorf("middleware.pgsql.host 为空")
		}
		return DriverPgSQL, c, nil
	default:
		return "", config.DBConn{}, fmt.Errorf("不支持的数据库类型 %s（请选 mysql 或 pgsql）", want)
	}
}

func openDB(driver string, c config.DBConn, database string) (*sql.DB, error) {
	switch driver {
	case DriverMySQL:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true&charset=utf8mb4",
			c.User, c.Password, c.Host, c.Port, database)
		return sql.Open("mysql", dsn)
	case DriverPgSQL:
		if database == "" {
			database = "postgres"
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			c.Host, c.Port, c.User, c.Password, database)
		return sql.Open("postgres", dsn)
	default:
		return nil, fmt.Errorf("不支持的驱动 %s", driver)
	}
}

func normalizeSQLFiles(files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("请至少选择一个 .sql 文件")
	}
	out := make([]string, 0, len(files))
	seen := map[string]struct{}{}
	for _, raw := range files {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("路径无效: %s", p)
		}
		if !strings.EqualFold(filepath.Ext(abs), ".sql") {
			return nil, fmt.Errorf("不是 .sql 文件: %s", abs)
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("无法读取 %s: %w", abs, err)
		}
		if st.IsDir() {
			return nil, fmt.Errorf("%s 是目录，请选择具体的 .sql 文件", abs)
		}
		if st.Size() > maxSQLBytes {
			return nil, fmt.Errorf("%s 超过 %d MB，请拆分后再执行", filepath.Base(abs), maxSQLBytes>>20)
		}
		key := filepath.Clean(abs)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, abs)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("请至少选择一个 .sql 文件")
	}
	return out, nil
}

func execSQLFile(db *sql.DB, driver, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return fmt.Errorf("文件为空")
	}
	if driver == DriverMySQL {
		_, err := db.Exec(text)
		return err
	}
	stmts := splitSQL(text)
	if len(stmts) == 0 {
		return fmt.Errorf("没有可执行的 SQL 语句")
	}
	for i, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("第 %d 条语句: %w", i+1, err)
		}
	}
	return nil
}

// splitSQL 按分号切开顶层语句，忽略引号、$tag$ 与注释里的分号。
func splitSQL(src string) []string {
	var out []string
	var b strings.Builder
	inSingle, inLineComment, inBlockComment := false, false, false
	dollarTag := ""
	runes := []rune(src)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if inLineComment {
			b.WriteRune(ch)
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			b.WriteRune(ch)
			if ch == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				b.WriteRune('/')
				i++
				inBlockComment = false
			}
			continue
		}
		if dollarTag != "" {
			if ch == '$' {
				if tag, n := dollarTagAt(runes, i); n > 0 && tag == dollarTag {
					b.WriteString(tag)
					i += n - 1
					dollarTag = ""
					continue
				}
			}
			b.WriteRune(ch)
			continue
		}
		if inSingle {
			b.WriteRune(ch)
			if ch == '\'' {
				if i+1 < len(runes) && runes[i+1] == '\'' {
					b.WriteRune('\'')
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if ch == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			b.WriteString("--")
			i++
			inLineComment = true
			continue
		}
		if ch == '/' && i+1 < len(runes) && runes[i+1] == '*' {
			b.WriteString("/*")
			i++
			inBlockComment = true
			continue
		}
		if ch == '\'' {
			inSingle = true
			b.WriteRune(ch)
			continue
		}
		if ch == '$' {
			if tag, n := dollarTagAt(runes, i); n > 0 {
				dollarTag = tag
				b.WriteString(tag)
				i += n - 1
				continue
			}
		}
		if ch == ';' {
			stmt := strings.TrimSpace(b.String())
			b.Reset()
			if stmt != "" && !sqlCommentOnly(stmt) {
				out = append(out, stmt)
			}
			continue
		}
		b.WriteRune(ch)
	}
	if tail := strings.TrimSpace(b.String()); tail != "" && !sqlCommentOnly(tail) {
		out = append(out, tail)
	}
	return out
}

// dollarTagAt 从 runes[i] 读取 $tag$；不是美元引用则返回空。
func dollarTagAt(runes []rune, i int) (string, int) {
	if i >= len(runes) || runes[i] != '$' {
		return "", 0
	}
	j := i + 1
	for j < len(runes) {
		ch := runes[j]
		if ch == '$' {
			return string(runes[i : j+1]), j - i + 1
		}
		if !(unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_') {
			return "", 0
		}
		j++
	}
	return "", 0
}

// sqlCommentOnly 判断去掉前置注释后是否还有可执行内容。
// 不能只看 HasPrefix("--")：现场脚本常在语句前写一行注释，整段丢掉会漏执行。
func sqlCommentOnly(s string) bool {
	t := strings.TrimSpace(s)
	for t != "" {
		if strings.HasPrefix(t, "--") {
			i := strings.IndexByte(t, '\n')
			if i < 0 {
				return true
			}
			t = strings.TrimSpace(t[i+1:])
			continue
		}
		if strings.HasPrefix(t, "/*") {
			i := strings.Index(t, "*/")
			if i < 0 {
				return true
			}
			t = strings.TrimSpace(t[i+2:])
			continue
		}
		return false
	}
	return true
}
