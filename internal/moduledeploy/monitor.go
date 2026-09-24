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

// MonitorPiece 监控包里要单独 compose up 的一块。
type MonitorPiece struct {
	Name string // center / prometheus / node / cadvisor / kafka / mysql / pgsql / redis
	Dir  string
	Host string // 部署到这台机器的 IP
}

var reHTTPSd = regexp.MustCompile(`(url:\s*['"])http://[^/'"\s]+(/prometheus/hp['"])`)
var reIPv4Port = regexp.MustCompile(`\d{1,3}(?:\.\d{1,3}){3}:\d+`)

// LocateMonitorPieces 在 monitor 模块目录下找出中心、Prometheus 和 otherServices。
// 目录还没解压时，调用方应先 PrepareModuleArchives。
func LocateMonitorPieces(monitorDir string) (center, prometheus string, exporters map[string]string, err error) {
	monitorDir = filepath.Clean(strings.TrimSpace(monitorDir))
	if !util.DirExists(monitorDir) {
		return "", "", nil, fmt.Errorf("监控目录不存在: %s", monitorDir)
	}
	center = monitorDir
	if composeInDir(monitorDir) == "" {
		return "", "", nil, fmt.Errorf("监控目录缺少 docker-compose: %s", monitorDir)
	}
	prometheus = findNamedComposeDir(monitorDir, "prometheus")
	exporters = map[string]string{}
	for _, name := range []string{"node", "cadvisor", "kafka", "mysql", "pgsql", "redis"} {
		if dir := findNamedComposeDir(monitorDir, name); dir != "" {
			exporters[name] = dir
		}
	}
	return center, prometheus, exporters, nil
}

func findNamedComposeDir(root, name string) string {
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || found != "" {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		base := strings.ToLower(info.Name())
		if base == "data" || base == "logs" || base == "log" || base == ".git" {
			return filepath.SkipDir
		}
		if !strings.EqualFold(info.Name(), name) {
			return nil
		}
		if composeInDir(path) != "" {
			found = path
			return filepath.SkipDir
		}
		nested := filepath.Join(path, info.Name())
		if composeInDir(nested) != "" {
			found = nested
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

// PatchMonitorEnv 改监控平台 .env 的主机与账号。端口保持文件里的原值。
func PatchMonitorEnv(envPath string, site *config.SiteConfig, monitorIP string) ([]string, error) {
	changed, err := EnvPatchHosts(envPath, site)
	if err != nil {
		return nil, err
	}
	extra, err := patchEnvKeys(envPath, monitorEnvExtras(site, monitorIP))
	if err != nil {
		return changed, err
	}
	return append(changed, extra...), nil
}

func monitorEnvExtras(site *config.SiteConfig, monitorIP string) map[string]string {
	m := site.Middleware
	repl := map[string]string{
		"NACOS_USERNAME":      strings.TrimSpace(m.Nacos.Username),
		"NACOS_PASSWORD":      m.Nacos.Password,
		"DATASOURCE_USER":     strings.TrimSpace(m.PgSQL.User),
		"DATASOURCE_PASSWORD": m.PgSQL.Password,
		"REDIS_PASSWORD":      m.Redis.Password,
		"PROMETHEUS_HOST":     strings.TrimSpace(monitorIP),
	}
	if portal := nginxHost(site); portal != "" {
		repl["METRIC_URL"] = portal
	}
	return repl
}

func nginxHost(site *config.SiteConfig) string {
	for _, n := range site.Nodes {
		for _, s := range n.Services {
			if strings.EqualFold(s, "nginx") && strings.TrimSpace(n.IP) != "" {
				return strings.TrimSpace(n.IP)
			}
		}
	}
	return strings.TrimSpace(site.Middleware.Nacos.Host)
}

func patchEnvKeys(envPath string, repl map[string]string) ([]string, error) {
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
		key, val, ok := strings.Cut(trim, "=")
		if !ok || trim == "" || strings.HasPrefix(trim, "#") {
			out.WriteString(line + "\n")
			continue
		}
		key = strings.TrimSpace(key)
		nv, exists := repl[key]
		if !exists || strings.TrimSpace(nv) == "" {
			out.WriteString(line + "\n")
			continue
		}
		if key == "METRIC_URL" {
			nv = replaceHostKeepPort(val, nv)
		}
		if strings.TrimSpace(val) == nv {
			out.WriteString(line + "\n")
			continue
		}
		changed = append(changed, key)
		out.WriteString(key + "=" + nv + "\n")
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return nil, nil
	}
	return changed, os.WriteFile(envPath, []byte(out.String()), 0o644)
}

func replaceHostKeepPort(old, host string) string {
	old = strings.TrimSpace(old)
	host = strings.TrimSpace(host)
	if i := strings.LastIndex(old, ":"); i > 0 && !strings.Contains(old[i+1:], "/") {
		return host + old[i:]
	}
	return host
}

// PatchPrometheusSD 把 http_sd 地址改成监控平台 IP:18099，remote_write 不动。
func PatchPrometheusSD(path, monitorIP string) error {
	monitorIP = strings.TrimSpace(monitorIP)
	if monitorIP == "" {
		return fmt.Errorf("监控平台 IP 为空")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	next := reHTTPSd.ReplaceAllString(string(data), "${1}http://"+monitorIP+":18099${2}")
	if next == string(data) {
		return fmt.Errorf("prometheus.yml 未找到 http_sd url")
	}
	return os.WriteFile(path, []byte(next), 0o644)
}

// PatchExporterCompose 改 exporter compose 的 command 里的目标地址，不动 image 行。
func PatchExporterCompose(path, kind, host string, port int, password string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)
	var b strings.Builder
	inCmd := false
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "command:") || trim == "command:" {
			inCmd = true
		} else if inCmd && len(line) > 0 && line[0] != ' ' && line[0] != '-' && line[0] != '\t' {
			inCmd = false
		}
		if inCmd {
			line = patchExporterLine(line, kind, host, port, password)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	out := strings.TrimRight(b.String(), "\n") + "\n"
	return os.WriteFile(path, []byte(out), 0o644)
}

// MonitorFile 监控包里可打开或改地址的文件。
type MonitorFile struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Path   string `json:"path"`
	Kind   string `json:"kind"` // env / yml / compose
	Exists bool   `json:"exists"`
}

// ListMonitorFiles 解析监控目录下的 .env、prometheus.yml 和各块 compose。
func ListMonitorFiles(monitorDir string) (string, []MonitorFile, error) {
	center, promDir, pieces, err := LocateMonitorPieces(monitorDir)
	if err != nil {
		return monitorDir, nil, err
	}
	var files []MonitorFile
	files = append(files, monitorFile("env", "监控 .env", filepath.Join(center, ".env"), "env"))
	files = append(files, monitorFile("center", "监控平台 compose", composePath(center), "compose"))
	if promDir != "" {
		files = append(files, monitorFile("prometheus", "Prometheus compose", composePath(promDir), "compose"))
		yml := filepath.Join(promDir, "config", "prometheus.yml")
		if !util.FileExists(yml) {
			yml = filepath.Join(filepath.Dir(promDir), "config", "prometheus.yml")
		}
		files = append(files, monitorFile("prometheus-yml", "prometheus.yml", yml, "yml"))
	}
	labels := map[string]string{
		"node": "node compose", "cadvisor": "cadvisor compose",
		"kafka": "Kafka 采集 compose", "mysql": "MySQL 采集 compose",
		"pgsql": "PostgreSQL 采集 compose", "redis": "Redis 采集 compose",
	}
	for _, name := range []string{"node", "cadvisor", "kafka", "mysql", "pgsql", "redis"} {
		dir := pieces[name]
		if dir == "" {
			if zip := findServiceZip(monitorDir, name); zip != "" {
				files = append(files, MonitorFile{ID: name, Label: labels[name], Path: zip, Kind: "zip", Exists: true})
				continue
			}
			files = append(files, MonitorFile{ID: name, Label: labels[name], Kind: "compose", Exists: false})
			continue
		}
		files = append(files, monitorFile(name, labels[name], composePath(dir), "compose"))
	}
	return center, files, nil
}

// OtherServicesDir 返回 monitor/otherServices，没有则空。
func OtherServicesDir(monitorDir string) string {
	var found string
	_ = filepath.Walk(monitorDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || found != "" || !info.IsDir() {
			return nil
		}
		if strings.EqualFold(info.Name(), "otherServices") {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

func findServiceZip(monitorDir, name string) string {
	var found string
	_ = filepath.Walk(monitorDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || found != "" {
			return nil
		}
		if info.IsDir() {
			base := strings.ToLower(info.Name())
			if base == "data" || base == "logs" || base == "log" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(info.Name(), name+".zip") {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

func monitorFile(id, label, path, kind string) MonitorFile {
	return MonitorFile{ID: id, Label: label, Path: path, Kind: kind, Exists: path != "" && util.FileExists(path)}
}

func composePath(dir string) string {
	if p := composeInDir(dir); p != "" {
		return p
	}
	return filepath.Join(dir, "docker-compose.yml")
}

func patchExporterLine(line, kind, host string, port int, password string) string {
	hp := host
	if port > 0 {
		hp = host + ":" + strconv.Itoa(port)
	}
	switch kind {
	case "kafka":
		line = regexp.MustCompile(`--kafka\.server=\S+`).ReplaceAllString(line, "--kafka.server="+hp)
	case "redis":
		line = regexp.MustCompile(`redis://\S+`).ReplaceAllString(line, "redis://"+hp)
		if password != "" {
			line = regexp.MustCompile(`--redis\.password=\S+`).ReplaceAllString(line, "--redis.password="+password)
		}
	default:
		line = reIPv4Port.ReplaceAllString(line, hp)
	}
	return line
}
