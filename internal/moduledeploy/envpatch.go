package moduledeploy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

// EnvPatchHosts 按 site.yaml 回写 .env 中已有键。
// 市政水厂 PgSQL 版未改 MYSQL_* 键名：MySQL 已跳过时，把 PgSQL 的 host/port/user/password 填进这些键。
func EnvPatchHosts(envPath string, site *config.SiteConfig) ([]string, error) {
	if site == nil {
		return nil, fmt.Errorf("site 不能为空")
	}
	if !util.FileExists(envPath) {
		return nil, fmt.Errorf(".env 不存在: %s", envPath)
	}
	repl := hostReplacements(site)
	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil, err
	}
	var out strings.Builder
	var changed []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			out.WriteString(line + "\n")
			continue
		}
		key, val, ok := strings.Cut(trim, "=")
		if !ok {
			out.WriteString(line + "\n")
			continue
		}
		key = strings.TrimSpace(key)
		if newVal, ok := repl[key]; ok && newVal != "" && val != newVal {
			changed = append(changed, key)
			out.WriteString(key + "=" + newVal + "\n")
		} else {
			out.WriteString(line + "\n")
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return changed, nil
	}
	tmp := envPath + ".wpgctl.tmp"
	if err := os.WriteFile(tmp, []byte(out.String()), 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, envPath); err != nil {
		_ = os.WriteFile(envPath, []byte(out.String()), 0o644)
		_ = os.Remove(tmp)
	}
	return changed, nil
}

func hostReplacements(site *config.SiteConfig) map[string]string {
	m := site.Middleware
	ns := m.Nacos.Namespace
	if ns == "" {
		ns = site.Site.Code
	}
	nacosHost := m.Nacos.Host
	kafkaHost := m.Kafka.Host
	redisHost := m.Redis.Host
	mysqlHost := m.MySQL.Host
	pgsqlHost := m.PgSQL.Host
	if pgsqlHost == "" && !m.MySQL.Disabled {
		pgsqlHost = mysqlHost
	}
	pgsqlPort := m.PgSQL.Port
	if pgsqlPort == 0 {
		pgsqlPort = 5433
	}
	repl := map[string]string{
		"NACOS_HOST":         nacosHost,
		"NACOS_PORT":         strconv.Itoa(m.Nacos.Port),
		"NACOS_NAMESPACE":    ns,
		"REGISTER_HOST":      nacosHost,
		"REGISTER_PORT":      strconv.Itoa(m.Nacos.Port),
		"REGISTER_NAMESPACE": ns,
		"DATASOURCE_HOST":    pgsqlHost,
		"DATASOURCE_ADDR":    pgsqlHost,
		"DATASOURCE_PORT":    strconv.Itoa(pgsqlPort),
		"REDIS_HOST":         redisHost,
		"WPG_REDIS_HOST":     redisHost,
		"REDIS_PORT":         strconv.Itoa(m.Redis.Port),
		"KAFKA_HOST":         kafkaHost,
		"KAFKA_PORT":         strconv.Itoa(m.Kafka.Port),
		"MINIO_HOST":         nacosHost, // 常见与业务同机；可在 site 扩展前默认 app 节点
	}
	putMySQLKeyedDB(repl, m)
	return repl
}

func putKeys(repl map[string]string, val string, keys ...string) {
	val = strings.TrimSpace(val)
	if val == "" {
		return
	}
	for _, k := range keys {
		repl[k] = val
	}
}

// putMySQLKeyedDB 把当前库连接写入 MYSQL_* / WPG_MYSQL_*。
// 现场市政包从 MySQL 切 PgSQL 后键名未改，因此跳过 MySQL 时写入 PgSQL 的 IP/端口/账号/密码。
func putMySQLKeyedDB(repl map[string]string, m config.MiddlewareConfig) {
	host, port, user, pass := mysqlKeyedDB(m)
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

func mysqlKeyedDB(m config.MiddlewareConfig) (host string, port int, user, pass string) {
	if m.MySQL.Disabled {
		port = m.PgSQL.Port
		if port == 0 {
			port = 5433
		}
		return strings.TrimSpace(m.PgSQL.Host), port, strings.TrimSpace(m.PgSQL.User), m.PgSQL.Password
	}
	port = m.MySQL.Port
	if port == 0 {
		port = 3306
	}
	return strings.TrimSpace(m.MySQL.Host), port, strings.TrimSpace(m.MySQL.User), m.MySQL.Password
}

// PatchEnvTree 递归 patch 目录下全部 .env（业务模块含子目录）。
func PatchEnvTree(root string, site *config.SiteConfig) ([]string, error) {
	var all []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.ToLower(info.Name()) != ".env" {
			return nil
		}
		changed, err := EnvPatchHosts(path, site)
		if err != nil {
			return err
		}
		for _, k := range changed {
			all = append(all, filepath.Join(filepath.Dir(path), k+" in .env"))
		}
		return nil
	})
	return all, err
}
