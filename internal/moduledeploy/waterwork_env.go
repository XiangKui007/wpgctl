package moduledeploy

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
)

// isWaterworkOrModelEnv 市政水厂 / 模型服务的 .env：PgSQL 版仍用 MYSQL_* 键名。
func isWaterworkOrModelEnv(path string) bool {
	p := strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(p, "/gis/") || strings.Contains(p, "/giscenter") || strings.Contains(p, "/gisdefault") {
		return false
	}
	return strings.Contains(p, "waterwork") ||
		strings.Contains(p, "intelligent-model") ||
		strings.Contains(p, "intelligent_model")
}

// putWaterworkEnvKeys 把业务 PgSQL 写入 MYSQL_* / WPG_MYSQL_*。
// 与是否勾选跳过 MySQL 无关：市政/模型包是 PgSQL 版，两套 .env 内容相同，都要改。
func putWaterworkEnvKeys(repl map[string]string, site *config.SiteConfig, envPath string) {
	if repl == nil || site == nil || !isWaterworkOrModelEnv(envPath) {
		return
	}
	m := site.Middleware
	host := strings.TrimSpace(m.PgSQL.Host)
	if h := hostOfService(site, "pgsql", []string{"database"}); h != "" {
		host = h
	}
	port := m.PgSQL.Port
	if port == 0 {
		port = 5433
	}
	user := strings.TrimSpace(m.PgSQL.User)
	pass := m.PgSQL.Password
	putKeys(repl, host,
		"MYSQL_HOST", "MYSQL_ADDR", "MYSQL_IP", "MYSQL_HOSTNAME",
		"WPG_MYSQL_HOST")
	if port > 0 {
		putKeys(repl, strconv.Itoa(port), "MYSQL_PORT", "WPG_MYSQL_PORT")
	}
	putKeys(repl, user,
		"MYSQL_USER", "MYSQL_USERNAME", "MYSQL_USER_NAME",
		"WPG_MYSQL_USER", "WPG_MYSQL_USERNAME")
	putKeys(repl, pass,
		"MYSQL_PASSWORD", "MYSQL_PASSWD", "MYSQL_PWD", "MYSQL_PSWD", "MYSQL_PASS",
		"WPG_MYSQL_PASSWORD", "WPG_MYSQL_PASSWD", "WPG_MYSQL_PSWD")
}
