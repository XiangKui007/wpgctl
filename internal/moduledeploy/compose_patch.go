package moduledeploy

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

var reComposeEnvLine = regexp.MustCompile(`^(\s*-\s*)([A-Za-z0-9_]+)=(.*)$`)

// ComposeEnvReplacements 需写入 docker-compose environment 的变量（无 .env 的模块如 kafka）。
// KAFKA_ADVERTISED_LISTENERS 使用「跑 Kafka 那台机」的 IP（middleware.kafka.host，或节点 services 含 kafka）。
func ComposeEnvReplacements(site *config.SiteConfig) map[string]string {
	if site == nil {
		return nil
	}
	host := kafkaAdvertiseHost(site)
	port := site.Middleware.Kafka.Port
	if port <= 0 {
		port = 9092
	}
	if host == "" {
		return nil
	}
	return map[string]string{
		"KAFKA_ADVERTISED_LISTENERS": fmt.Sprintf("PLAINTEXT://%s:%d", host, port),
	}
}

func kafkaAdvertiseHost(site *config.SiteConfig) string {
	if h := strings.TrimSpace(site.Middleware.Kafka.Host); h != "" {
		return h
	}
	for _, n := range site.Nodes {
		for _, s := range n.Services {
			if strings.EqualFold(strings.TrimSpace(s), "kafka") {
				return strings.TrimSpace(n.IP)
			}
		}
	}
	if len(site.Nodes) > 0 {
		return strings.TrimSpace(site.Nodes[0].IP)
	}
	return ""
}

// PatchComposeEnv 按 key 精确匹配 compose 中 `- KEY=VALUE` 行并替换 VALUE。
func PatchComposeEnv(composePath string, repl map[string]string) ([]string, error) {
	if len(repl) == 0 {
		return nil, nil
	}
	if !util.FileExists(composePath) {
		return nil, fmt.Errorf("compose 不存在: %s", composePath)
	}
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, err
	}
	var out strings.Builder
	var changed []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := sc.Text()
		m := reComposeEnvLine.FindStringSubmatch(line)
		if len(m) == 4 {
			key := m[2]
			if newVal, ok := repl[key]; ok {
				oldVal := strings.TrimSpace(m[3])
				if oldVal != newVal {
					changed = append(changed, key)
					out.WriteString(m[1] + key + "=" + newVal + "\n")
					continue
				}
			}
		}
		out.WriteString(line + "\n")
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(changed) == 0 {
		return changed, nil
	}
	tmp := composePath + ".wpgctl.tmp"
	if err := os.WriteFile(tmp, []byte(out.String()), 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, composePath); err != nil {
		_ = os.WriteFile(composePath, []byte(out.String()), 0o644)
		_ = os.Remove(tmp)
	}
	return changed, nil
}

// PatchComposeEnvForSite 对模块目录内全部 compose 应用站点级 environment 补丁。
func PatchComposeEnvForSite(moduleDir string, site *config.SiteConfig) ([]string, error) {
	projects, err := findComposeProjects(moduleDir)
	if err != nil {
		// 市政/模型等目录若尚未找到 compose，Kafka 补丁可跳过，后续 compose up 会给出明确错误
		return nil, nil
	}
	repl := ComposeEnvReplacements(site)
	if len(repl) == 0 {
		return nil, nil
	}
	var all []string
	for _, compose := range projects {
		changed, err := PatchComposeEnv(compose, repl)
		if err != nil {
			return all, err
		}
		all = append(all, changed...)
	}
	return all, nil
}
