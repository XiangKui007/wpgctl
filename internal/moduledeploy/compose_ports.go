package moduledeploy

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
	"gopkg.in/yaml.v3"
)

var reEnvSubst = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

type composeDoc struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	NetworkMode string        `yaml:"network_mode"`
	Ports       []interface{} `yaml:"ports"`
	Environment interface{}   `yaml:"environment"`
	EnvFile     interface{}   `yaml:"env_file"`
}

// CollectListenPorts 收集 moduleDir 下各 compose 实际会监听的 TCP 端口。
// composeFiles 为空时自行查找；解析不到时用 site.yaml 中与模块名匹配的中间件端口兜底。
// Nacos 额外放行 HTTP 端口 + 1000（gRPC，如 8848→9848）；nginx 额外解析 conf 里的 listen，且固定含 8877。
func CollectListenPorts(moduleDir string, composeFiles []string, site *config.SiteConfig) []int {
	dir := strings.TrimSpace(moduleDir)
	if len(composeFiles) == 0 && dir != "" {
		if found, err := findComposeProjects(dir); err == nil {
			composeFiles = found
		}
	}
	var ports []int
	for _, compose := range composeFiles {
		ports = append(ports, portsFromComposeFile(compose)...)
	}
	ports = uniqueInts(append(ports, siteModulePorts(dir, site)...))
	if IsNacosModule(dir) {
		ports = uniqueInts(append(ports, NacosListenPorts(site)...))
	}
	if IsNginxModule(dir) {
		ports = uniqueInts(append(ports, NginxListenPorts(dir)...))
	}
	return ports
}

func portsFromComposeFile(composePath string) []int {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil
	}
	var doc composeDoc
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Services) == 0 {
		return nil
	}
	composeDir := filepath.Dir(composePath)
	baseEnv := loadDotEnv(filepath.Join(composeDir, ".env"))
	var ports []int
	for _, svc := range doc.Services {
		env := cloneEnv(baseEnv)
		mergeEnvFile(env, composeDir, svc.EnvFile)
		mergeComposeEnvironment(env, svc.Environment)
		hostNet := isHostNetwork(svc.NetworkMode, env)
		for _, item := range svc.Ports {
			if p := publishedPort(item, env); p > 0 {
				ports = append(ports, p)
			}
		}
		if hostNet || len(svc.Ports) == 0 {
			ports = append(ports, listenPortsFromEnv(env)...)
		}
	}
	return uniqueInts(ports)
}

func isHostNetwork(mode string, env map[string]string) bool {
	mode = substEnv(strings.TrimSpace(mode), env)
	return strings.EqualFold(mode, "host")
}

func publishedPort(item interface{}, env map[string]string) int {
	switch v := item.(type) {
	case string:
		return parsePublishedPort(substEnv(v, env))
	case int, int64, uint64, float64:
		return parsePortNumber(asString(v))
	case map[string]interface{}:
		return publishedFromMap(v, env)
	case map[interface{}]interface{}:
		m := map[string]interface{}{}
		for k, val := range v {
			m[asString(k)] = val
		}
		if p := publishedFromMap(m, env); p > 0 {
			return p
		}
		if len(v) == 1 {
			for k := range v {
				if n := parsePortNumber(asString(k)); n > 0 {
					return n
				}
			}
		}
		return 0
	default:
		return 0
	}
}

func publishedFromMap(v map[string]interface{}, env map[string]string) int {
	proto := strings.ToLower(strings.TrimSpace(asString(mapValOK(v, "protocol"))))
	if proto == "udp" {
		return 0
	}
	if raw, ok := v["published"]; ok {
		return parsePublishedPort(substEnv(asString(raw), env))
	}
	if len(v) == 1 {
		for k := range v {
			if p := parsePortNumber(substEnv(k, env)); p > 0 {
				return p
			}
		}
	}
	return 0
}

func mapValOK(m map[string]interface{}, key string) interface{} {
	if v, ok := m[key]; ok {
		return v
	}
	return nil
}

// parsePublishedPort 解析 compose 短语法中的宿主机端口（忽略 UDP）。
func parsePublishedPort(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if i := strings.Index(s, "/"); i >= 0 {
		proto := strings.ToLower(strings.TrimSpace(s[i+1:]))
		s = strings.TrimSpace(s[:i])
		if proto == "udp" {
			return 0
		}
	}
	if strings.HasPrefix(s, "[") {
		rb := strings.Index(s, "]")
		if rb >= 0 {
			rest := strings.TrimPrefix(s[rb+1:], ":")
			return parsePublishedPort(rest)
		}
	}
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 0:
		return 0
	case 1:
		return parsePortNumber(parts[0])
	case 2:
		return parsePortNumber(parts[0])
	default:
		return parsePortNumber(parts[len(parts)-2])
	}
}

func parsePortNumber(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "-") {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 65535 {
		return 0
	}
	return n
}

func listenPortsFromEnv(env map[string]string) []int {
	var ports []int
	for k, v := range env {
		if !isListenEnvKey(k) {
			continue
		}
		if p := parsePortNumber(substEnv(v, env)); p > 0 {
			ports = append(ports, p)
		}
	}
	return ports
}

// isListenEnvKey 判断是否为本进程监听端口，排除 MYSQL_PORT 等客户端连库端口。
func isListenEnvKey(key string) bool {
	u := strings.ToUpper(strings.TrimSpace(key))
	if u == "" || strings.Contains(u, "PASSWORD") || strings.Contains(u, "PASSWD") {
		return false
	}
	switch u {
	case "PORT", "SERVER_PORT", "LISTEN_PORT", "HTTP_PORT", "HTTPS_PORT",
		"GRPC_PORT", "APP_PORT", "WEB_PORT", "NACOS_APPLICATION_PORT", "MYSQL_TCP_PORT":
		return true
	}
	return strings.HasSuffix(u, "_SERVER_PORT") ||
		strings.HasSuffix(u, "_LISTEN_PORT") ||
		strings.HasSuffix(u, "_HTTP_PORT") ||
		strings.HasSuffix(u, "_GRPC_PORT")
}

func siteModulePorts(moduleDir string, site *config.SiteConfig) []int {
	if site == nil {
		return nil
	}
	name := strings.ToLower(filepath.Base(strings.TrimSpace(moduleDir)))
	m := site.Middleware
	switch {
	case name == "nacos" || strings.HasPrefix(name, "nacos-"):
		return NacosListenPorts(site)
	case name == "redis" || strings.HasPrefix(name, "redis-"):
		if m.Redis.Port > 0 {
			return []int{m.Redis.Port}
		}
	case name == "mysql" || strings.HasPrefix(name, "mysql-"):
		if !m.MySQL.Disabled && m.MySQL.Port > 0 {
			return []int{m.MySQL.Port}
		}
	case name == "pgsql" || name == "postgres" || name == "postgresql" || name == "postgis" ||
		strings.HasPrefix(name, "pgsql-") || strings.HasPrefix(name, "postgres"):
		if m.PgSQL.Port > 0 {
			return []int{m.PgSQL.Port}
		}
	case name == "kafka" || strings.HasPrefix(name, "kafka-"):
		if m.Kafka.Port > 0 {
			return []int{m.Kafka.Port}
		}
	}
	return nil
}

func loadDotEnv(path string) map[string]string {
	out := map[string]string{}
	if !util.FileExists(path) {
		return out
	}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func mergeEnvFile(env map[string]string, composeDir string, raw interface{}) {
	for _, name := range stringList(raw) {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		p := name
		if !filepath.IsAbs(p) {
			p = filepath.Join(composeDir, name)
		}
		for k, v := range loadDotEnv(p) {
			env[k] = v
		}
	}
}

func mergeComposeEnvironment(env map[string]string, raw interface{}) {
	if raw == nil {
		return
	}
	switch v := raw.(type) {
	case map[string]interface{}:
		for k, val := range v {
			env[k] = substEnv(asString(val), env)
		}
	case map[interface{}]interface{}:
		for k, val := range v {
			env[asString(k)] = substEnv(asString(val), env)
		}
	case []interface{}:
		for _, item := range v {
			switch e := item.(type) {
			case string:
				k, val, ok := strings.Cut(e, "=")
				if ok {
					env[strings.TrimSpace(k)] = substEnv(strings.TrimSpace(val), env)
				}
			case map[string]interface{}:
				for k, val := range e {
					env[k] = substEnv(asString(val), env)
				}
			}
		}
	}
}

func substEnv(s string, env map[string]string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	return reEnvSubst.ReplaceAllStringFunc(s, func(m string) string {
		sub := reEnvSubst.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		if v, ok := env[sub[1]]; ok && v != "" {
			return v
		}
		if len(sub) >= 3 {
			return sub[2]
		}
		return ""
	})
}

func cloneEnv(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stringList(raw interface{}) []string {
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	case []interface{}:
		var out []string
		for _, item := range v {
			if s := strings.TrimSpace(asString(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func asString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float64:
		return strconv.Itoa(int(t))
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func uniqueInts(in []int) []int {
	seen := map[int]struct{}{}
	var out []int
	for _, p := range in {
		if p <= 0 {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
