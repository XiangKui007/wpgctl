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

// EnvPatchHosts 仅替换 IP/Host 类变量，密码等保持原样（现场 SOP：只改 IP）。
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
	repl := map[string]string{
		"NACOS_HOST":          nacosHost,
		"NACOS_PORT":          strconv.Itoa(m.Nacos.Port),
		"NACOS_NAMESPACE":     ns,
		"REGISTER_HOST":       nacosHost,
		"REGISTER_PORT":       strconv.Itoa(m.Nacos.Port),
		"REGISTER_NAMESPACE":  ns,
		"DATASOURCE_HOST":     pgsqlHost,
		"DATASOURCE_ADDR":     pgsqlHost,
		"DATASOURCE_PORT":     strconv.Itoa(m.PgSQL.Port),
		"REDIS_HOST":           redisHost,
		"WPG_REDIS_HOST":       redisHost,
		"REDIS_PORT":           strconv.Itoa(m.Redis.Port),
		"KAFKA_HOST":           kafkaHost,
		"KAFKA_PORT":           strconv.Itoa(m.Kafka.Port),
		"MINIO_HOST": nacosHost, // 常见与业务同机；可在 site 扩展前默认 app 节点
	}
	if !m.MySQL.Disabled && mysqlHost != "" {
		repl["WPG_MYSQL_HOST"] = mysqlHost
		repl["WPG_MYSQL_PORT"] = strconv.Itoa(m.MySQL.Port)
	}
	return repl
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
