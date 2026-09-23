package moduledeploy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

// EnvPatchHosts 按 site.yaml 回写 .env 中已有键（不新增键）。
// 市政水厂 PgSQL 版未改 MYSQL_* 键名：MySQL 已跳过时，把 PgSQL 的 host/port/user/password 填进这些键。
// MinIO / InfluxDB / EMQX / PostGIS / WaterJob 的 IP 来自节点 services 分配，不误用 Nacos 地址。
// GIS 模块另见 putGISEnvKeys：回写 WPG_PGSQL_* / WPG_MONGODB_*（与业务 PgSQL 不是同一套账号）。
func EnvPatchHosts(envPath string, site *config.SiteConfig) ([]string, error) {
	if site == nil {
		return nil, fmt.Errorf("site 不能为空")
	}
	if !util.FileExists(envPath) {
		return nil, fmt.Errorf(".env 不存在: %s", envPath)
	}
	repl := hostReplacements(site)
	putGISEnvKeys(repl, site, envPath)
	putWaterworkEnvKeys(repl, site, envPath)
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
		if nv, ok := patchedEnvValue(key, val, repl, site); ok {
			changed = append(changed, key)
			out.WriteString(key + "=" + nv + "\n")
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
	}
	putBinderKeys(repl, site)
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

// svcConn 某中间件在现场的连接（IP 来自节点规划，端口用组件默认值）。
type svcConn struct {
	host string
	port int
	user string
	pass string
}

func connOf(site *config.SiteConfig, service, fallback string, port int, dbRole bool) svcConn {
	roles := []string{"middleware"}
	if dbRole {
		roles = []string{"database"}
	}
	host := hostOfService(site, service, roles)
	if host == "" {
		host = strings.TrimSpace(fallback)
	}
	return svcConn{host: host, port: port}
}

// hostOfService 按 nodes[].services 找服务所在 IP；没有 services 时退回角色。
// waterjob 同时认 water-job / water-job-biz。
func hostOfService(site *config.SiteConfig, service string, fallbackRoles []string) string {
	if site == nil || strings.TrimSpace(service) == "" {
		return ""
	}
	wants := map[string]struct{}{}
	for _, alias := range moduleNameAliases(service) {
		a := strings.ToLower(strings.TrimSpace(alias))
		if a != "" {
			wants[a] = struct{}{}
		}
	}
	for _, n := range site.Nodes {
		for _, s := range n.Services {
			if _, ok := wants[strings.ToLower(strings.TrimSpace(s))]; ok {
				return strings.TrimSpace(n.IP)
			}
		}
	}
	if len(fallbackRoles) == 0 {
		return ""
	}
	roleWant := map[string]struct{}{}
	for _, r := range fallbackRoles {
		roleWant[strings.ToLower(strings.TrimSpace(r))] = struct{}{}
	}
	for _, n := range site.Nodes {
		for _, r := range n.Roles {
			if _, ok := roleWant[strings.ToLower(strings.TrimSpace(r))]; ok {
				return strings.TrimSpace(n.IP)
			}
		}
	}
	return ""
}

func patchedEnvValue(key, old string, repl map[string]string, site *config.SiteConfig) (string, bool) {
	k := strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
	b := binderForKey(key)
	spec := connFromBinder(site, b)
	if b != nil && spec.host != "" && (isURLEnvKey(k) || looksLikeEndpoint(old)) {
		nv := replaceHostPortInEndpoint(old, spec.host, spec.port)
		if nv != "" && nv != old {
			return nv, true
		}
	}
	if nv, ok := repl[key]; ok && nv != "" && nv != old {
		return nv, true
	}
	if b == nil || spec.host == "" {
		return "", false
	}
	switch {
	case isPortEnvKey(k):
		if spec.port <= 0 {
			return "", false
		}
		nv := strconv.Itoa(spec.port)
		return nv, nv != old
	case isUserEnvKey(k):
		if spec.user == "" || spec.user == old {
			return "", false
		}
		return spec.user, true
	case isPassEnvKey(k):
		if spec.pass == "" || spec.pass == old {
			return "", false
		}
		return spec.pass, true
	case isTokenEnvKey(k):
		if b.Token == "" || b.Token == old {
			return "", false
		}
		return b.Token, true
	case isHostEnvKey(k):
		return spec.host, spec.host != old
	default:
		return "", false
	}
}

func looksLikeEndpoint(v string) bool {
	v = strings.TrimSpace(v)
	return strings.Contains(v, "://") || reHostPort.MatchString(v)
}

func isHostEnvKey(k string) bool {
	return strings.HasSuffix(k, "_HOST") || strings.HasSuffix(k, "_HOSTNAME") ||
		strings.HasSuffix(k, "_IP") || strings.HasSuffix(k, "_ADDR") ||
		strings.HasSuffix(k, "_SERVER") || k == "MQTT_BROKER"
}

func isPortEnvKey(k string) bool {
	return strings.HasSuffix(k, "_PORT")
}

func isUserEnvKey(k string) bool {
	return strings.HasSuffix(k, "_USER") || strings.HasSuffix(k, "_USERNAME") || strings.HasSuffix(k, "_USER_NAME")
}

func isPassEnvKey(k string) bool {
	return strings.HasSuffix(k, "_PASSWORD") || strings.HasSuffix(k, "_PASSWD") ||
		strings.HasSuffix(k, "_PWD") || strings.HasSuffix(k, "_PSWD") || strings.HasSuffix(k, "_PASS")
}

func isTokenEnvKey(k string) bool {
	return strings.HasSuffix(k, "_TOKEN") || strings.HasSuffix(k, "_ACCESS_TOKEN")
}

func isURLEnvKey(k string) bool {
	return strings.Contains(k, "_URL") || strings.Contains(k, "ENDPOINT") ||
		strings.Contains(k, "JDBC") || strings.HasSuffix(k, "_URI")
}

var reEndpoint = regexp.MustCompile(`(?i)^(.*?://)([^:/\[\]]+|\[[0-9a-f:]+\])(:\d+)?(.*)$`)
var reHostPort = regexp.MustCompile(`^([^:/]+):(\d+)(.*)$`)

// replaceHostPortInEndpoint 改 URL / JDBC / host:port 里的地址，保留 path、库名。
func replaceHostPortInEndpoint(old, host string, port int) string {
	old = strings.TrimSpace(old)
	host = strings.TrimSpace(host)
	if old == "" || host == "" {
		return ""
	}
	if m := reEndpoint.FindStringSubmatch(old); m != nil {
		p := m[3]
		if port > 0 {
			p = ":" + strconv.Itoa(port)
		}
		return m[1] + host + p + m[4]
	}
	if m := reHostPort.FindStringSubmatch(old); m != nil {
		p := m[2]
		if port > 0 {
			p = strconv.Itoa(port)
		}
		return host + ":" + p + m[3]
	}
	return ""
}

// PatchEnvTree 递归 patch 目录下全部 .env（市政水厂 center/device、GIS 子目录、业务模块）。
// 跳过 data/logs，避免走进容器数据盘导致漏改另一套 .env。
func PatchEnvTree(root string, site *config.SiteConfig) ([]string, error) {
	var all []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			lower := strings.ToLower(info.Name())
			if lower == "data" || lower == "logs" || lower == "log" || lower == ".git" || lower == "bak" {
				return filepath.SkipDir
			}
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

// envPatchWalkRoots 决定从哪一层扫 .env。市政水厂即使只部署 center，也从包根扫，把 device 那份同样改掉。
func envPatchWalkRoots(moduleDir string) []string {
	dir := filepath.Clean(strings.TrimSpace(moduleDir))
	if dir == "" {
		return nil
	}
	if waterworkServiceKind(filepath.Base(dir)) != "" {
		parent := filepath.Dir(dir)
		if len(findNamedWaterworkComposes(parent)) > 0 {
			return []string{parent}
		}
	}
	return []string{dir}
}
