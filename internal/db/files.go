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
	Database string // 空则按 site.yaml / 脚本 \c / CREATE DATABASE / 文件名推断
	Log      func(string)
}

type applySession struct {
	db       *sql.DB
	driver   string
	conn     config.DBConn
	database string
	log      func(string)
	// created 本次运行新建的库名，汇总时标出「新建」，现场一眼能确认建库成功。
	created map[string]bool
}

// DBSummary 一次执行里某个库的落表结果，供日志汇总与前端展示。
type DBSummary struct {
	Database string   `json:"database"`
	Created  bool     `json:"created"`
	Files    []string `json:"files"`
	Tables   []string `json:"tables"`
}

// ApplyFiles 按选择顺序执行 .sql。失败立即停止，已执行的不回滚。
// 库名：表单 / site.yaml 指定则全部文件都打进该库；否则**每个文件单独**按脚本 \c/CREATE DATABASE → 文件名推断，
// 多选 wpg_waterwork_pg.sql + wpg_intelligent_model_pg.sql 时各进各的库，不会都落进第一个文件的库。
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
	fixed, fixedSrc := fixedTargetDB(driver, opts.Database, opts.Site)
	sess := &applySession{driver: driver, conn: conn, log: log, created: map[string]bool{}}
	defer sess.close()

	res := &Result{}
	summary := map[string]*DBSummary{}
	var order []string
	for _, path := range files {
		base := filepath.Base(path)
		target, src := fixed, fixedSrc
		if target == "" {
			target, src = resolveTargetDB(driver, "", nil, []string{path}, log)
		}
		if err := sess.ensureOn(target); err != nil {
			return res, err
		}
		log(fmt.Sprintf("已连接 %s %s:%d 库=%s（%s）", driver, conn.Host, conn.Port, sess.database, src))
		log("执行 " + path)
		if err := sess.execFile(path); err != nil {
			res.Database = sess.database
			return res, fmt.Errorf("执行 %s 失败: %w", base, err)
		}
		res.Applied = append(res.Applied, base)
		tables := sess.listTables()
		res.Database = sess.database
		res.Tables = tables
		if len(tables) > 0 {
			log(fmt.Sprintf("当前库 %s 现有表 %d 张：%s", sess.database, len(tables), strings.Join(tables, ", ")))
		} else {
			log(fmt.Sprintf("WARN: 当前库 %s 查不到业务表。若你看的是别的库，说明表建错地方了", sess.database))
		}
		log("完成 " + base)
		s, ok := summary[sess.database]
		if !ok {
			s = &DBSummary{Database: sess.database}
			summary[sess.database] = s
			order = append(order, sess.database)
		}
		s.Files = append(s.Files, base)
		s.Tables = tables
		s.Created = sess.created[sess.database]
	}
	for _, name := range order {
		res.Databases = append(res.Databases, *summary[name])
	}
	logApplySummary(log, res.Databases)
	return res, nil
}

// logApplySummary 收尾时按库汇总：哪个库是本次新建、落了多少表，避免现场只看到一串「成功」却不知表在哪。
func logApplySummary(log func(string), list []DBSummary) {
	if len(list) == 0 {
		return
	}
	log("—— 执行结果 ——")
	for _, s := range list {
		tag := "已存在"
		if s.Created {
			tag = "本次新建"
		}
		if len(s.Tables) == 0 {
			log(fmt.Sprintf("库 %s（%s）：来自 %s，但查不到任何表，请检查脚本是否只含数据/函数", s.Database, tag, strings.Join(s.Files, ", ")))
			continue
		}
		log(fmt.Sprintf("库 %s（%s）：来自 %s，共 %d 张表", s.Database, tag, strings.Join(s.Files, ", "), len(s.Tables)))
	}
}

// fixedTargetDB 表单或 site.yaml 显式给了库名，则所有文件都进该库。
func fixedTargetDB(driver, requested string, site *config.SiteConfig) (name, source string) {
	if n := strings.TrimSpace(requested); n != "" {
		return n, "表单指定"
	}
	if n := siteDatabase(site, driver); n != "" {
		return n, "site.yaml"
	}
	return "", ""
}

// resolveTargetDB 单个文件的目标库：表单 → site.yaml → 脚本 \c/CREATE DATABASE/USE → 文件名。
func resolveTargetDB(driver, requested string, site *config.SiteConfig, files []string, log func(string)) (name, source string) {
	if n, src := fixedTargetDB(driver, requested, site); n != "" {
		return n, src
	}
	if len(files) > 0 {
		if raw, err := os.ReadFile(files[0]); err == nil {
			if n := inferDBFromSQL(string(raw)); n != "" {
				return n, "脚本 CREATE DATABASE / \\c / USE"
			}
		}
		if n := inferDBFromFilename(files[0]); n != "" {
			return n, "文件名推断"
		}
	}
	if driver == DriverPgSQL {
		log("WARN: 未指定库名，将连 postgres。CREATE TABLE 会建在 postgres 里，业务库仍可能是空壳")
		return "postgres", "pgsql 默认库"
	}
	return "", "未指定库"
}

// ensureOn 保证会话连在 name 上：首次打开或库名变化时（重新）连接，必要时先建库。
func (s *applySession) ensureOn(name string) error {
	name = strings.TrimSpace(name)
	if s.db == nil {
		return s.open(name)
	}
	return s.switchTo(name)
}

func siteDatabase(site *config.SiteConfig, driver string) string {
	if site == nil {
		return ""
	}
	if driver == DriverPgSQL {
		return strings.TrimSpace(site.Middleware.PgSQL.Database)
	}
	return strings.TrimSpace(site.Middleware.MySQL.Database)
}

func (s *applySession) close() {
	if s != nil && s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
}

// open 连到 name；库不存在时先在管理库建库再连。pgsql 管理库为 postgres。
func (s *applySession) open(name string) error {
	name = strings.TrimSpace(name)
	admin := adminDB(s.driver)
	try := name
	if try == "" {
		try = admin
	}
	db, err := openDB(s.driver, s.conn, try)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		if name == "" || name == admin {
			return fmt.Errorf("连接数据库失败（%s %s:%d 库=%s）: %w", s.driver, s.conn.Host, s.conn.Port, try, err)
		}
		s.log(fmt.Sprintf("库 %s 还不存在，先在 %s 上创建…", name, admin))
		if err := s.createDBViaAdmin(name); err != nil {
			return err
		}
		db, err = openDB(s.driver, s.conn, name)
		if err != nil {
			return err
		}
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return fmt.Errorf("已建库 %s 但连不上: %w", name, err)
		}
	}
	if s.db != nil {
		_ = s.db.Close()
	}
	s.db = db
	if try == "" {
		s.database = "（未指定库）"
	} else {
		s.database = try
	}
	return nil
}

func adminDB(driver string) string {
	if driver == DriverPgSQL {
		return "postgres"
	}
	return ""
}

func (s *applySession) createDBViaAdmin(name string) error {
	if !validIdent(name) {
		return fmt.Errorf("非法库名 %q", name)
	}
	admin, err := openDB(s.driver, s.conn, adminDB(s.driver))
	if err != nil {
		return err
	}
	defer admin.Close()
	if err := admin.Ping(); err != nil {
		return fmt.Errorf("连管理库失败，无法创建 %s: %w", name, err)
	}
	created, err := execCreateDatabase(admin, s.driver, name)
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("创建库 %s 失败: %w", name, err)
	}
	s.markCreated(name, created && err == nil)
	return nil
}

// markCreated 记录建库结果并写日志；created=false 表示库本来就在。
func (s *applySession) markCreated(name string, created bool) {
	if created {
		if s.created == nil {
			s.created = map[string]bool{}
		}
		s.created[name] = true
		s.log("已创建库 " + name)
		return
	}
	s.log("库 " + name + " 已存在")
}

// execCreateDatabase 建库；返回 created=true 表示这次真的新建了，false 表示本来就有。
func execCreateDatabase(db *sql.DB, driver, name string) (bool, error) {
	if !validIdent(name) {
		return false, fmt.Errorf("非法库名 %q", name)
	}
	switch driver {
	case DriverPgSQL:
		var one int
		if err := db.QueryRow(`SELECT 1 FROM pg_database WHERE datname=$1`, name).Scan(&one); err == nil && one == 1 {
			return false, nil
		}
		_, err := db.Exec(`CREATE DATABASE "` + name + `"`)
		return err == nil, err
	case DriverMySQL:
		var found string
		if err := db.QueryRow(`SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?`, name).Scan(&found); err == nil && found != "" {
			return false, nil
		}
		_, err := db.Exec("CREATE DATABASE IF NOT EXISTS `" + name + "` DEFAULT CHARSET utf8mb4")
		return err == nil, err
	default:
		return false, fmt.Errorf("不支持的驱动 %s", driver)
	}
}

func (s *applySession) switchTo(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == s.database {
		return nil
	}
	s.log("切换到库 " + name)
	return s.open(name)
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

func (s *applySession) execFile(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return fmt.Errorf("文件为空")
	}
	if s.driver == DriverMySQL && strings.Contains(strings.ToUpper(text), "DELIMITER") {
		s.log("脚本含 DELIMITER，整文件一次执行（无法逐条记日志）")
		_, err := s.db.Exec(text)
		return err
	}
	stmts := splitSQL(markPsqlMetaLines(text))
	if len(stmts) == 0 {
		return fmt.Errorf("没有可执行的 SQL 语句")
	}
	s.log(fmt.Sprintf("文件 %s 拆成 %d 条", filepath.Base(path), len(stmts)))
	var createdTables, inserts, okN int
	for i, stmt := range stmts {
		n := i + 1
		kind, dbName := parseSwitchDB(stmt)
		preview := stmtPreview(stmt, 100)
		if isPsqlMeta(stmt) {
			if kind == "connect" && dbName != "" {
				s.log(fmt.Sprintf("(%d/%d) \\c %s", n, len(stmts), dbName))
				if err := s.switchTo(dbName); err != nil {
					return fmt.Errorf("第 %d 条 \\c %s: %w", n, dbName, err)
				}
				okN++
				continue
			}
			s.log(fmt.Sprintf("(%d/%d) 跳过 psql 元命令：%s", n, len(stmts), preview))
			continue
		}
		if kind == "use" && dbName != "" {
			s.log(fmt.Sprintf("(%d/%d) USE %s", n, len(stmts), dbName))
			if err := s.switchTo(dbName); err != nil {
				return fmt.Errorf("第 %d 条 USE %s: %w", n, dbName, err)
			}
			okN++
			continue
		}
		if kind == "createdb" && dbName != "" {
			s.log(fmt.Sprintf("(%d/%d) CREATE DATABASE %s", n, len(stmts), dbName))
			created, err := execCreateDatabase(s.db, s.driver, dbName)
			if err != nil && !isAlreadyExists(err) {
				// 连在目标库上无法 CREATE 自己时，改走管理库
				if err2 := s.createDBViaAdmin(dbName); err2 != nil {
					return fmt.Errorf("第 %d 条 CREATE DATABASE %s: %w", n, dbName, err)
				}
			} else {
				s.markCreated(dbName, created && err == nil)
			}
			if err := s.switchTo(dbName); err != nil {
				return fmt.Errorf("第 %d 条建库后切到 %s: %w", n, dbName, err)
			}
			okN++
			continue
		}
		if _, err := s.db.Exec(stmt); err != nil {
			s.log(fmt.Sprintf("(%d/%d) ERROR %s", n, len(stmts), preview))
			return fmt.Errorf("第 %d 条: %w\n语句: %s", n, err, preview)
		}
		okN++
		switch stmtKind(stmt) {
		case "createtable":
			createdTables++
			if t := parseCreateTable(stmt); t != "" {
				s.log(fmt.Sprintf("(%d/%d) CREATE TABLE %s  → 成功", n, len(stmts), t))
			} else {
				s.log(fmt.Sprintf("(%d/%d) %s  → 成功", n, len(stmts), preview))
			}
		case "insert", "copy":
			inserts++
		case "create", "alter", "drop":
			s.log(fmt.Sprintf("(%d/%d) %s  → 成功", n, len(stmts), preview))
		default:
			if n == 1 || n == len(stmts) || n%20 == 0 {
				s.log(fmt.Sprintf("(%d/%d) %s  → 成功", n, len(stmts), preview))
			}
		}
	}
	s.log(fmt.Sprintf("本文件执行成功 %d/%d 条（建表 %d，INSERT/COPY %d）", okN, len(stmts), createdTables, inserts))
	if createdTables == 0 && inserts == 0 {
		s.log("WARN: 文件里没有 CREATE TABLE / INSERT，请确认选的是表结构脚本")
	}
	return nil
}

func (s *applySession) listTables() []string {
	if s.db == nil {
		return nil
	}
	var rows *sql.Rows
	var err error
	switch s.driver {
	case DriverPgSQL:
		rows, err = s.db.Query(`SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY 1`)
	case DriverMySQL:
		rows, err = s.db.Query(`SHOW TABLES`)
	default:
		return nil
	}
	if err != nil {
		s.log("WARN: 查询表清单失败: " + err.Error())
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
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
	return stripLeadingSQLComments(s) == ""
}
