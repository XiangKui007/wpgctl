// Package status 实现 status / logs / diag 运维辅助命令（方案 §6.11）。
package status

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wpg/wpgctl/internal/config"
	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/precheck"
	"github.com/wpg/wpgctl/internal/state"
	"github.com/wpg/wpgctl/internal/util"
)

// ServiceListResult 服务状态查询结果。
type ServiceListResult struct {
	Services []dockerx.ComposeService `json:"services"`
	Nodes    []NodeSnapshot           `json:"nodes,omitempty"`
	Source   string                   `json:"source,omitempty"` // rendered | docker | cluster
	Warning  string                   `json:"warning,omitempty"`
	Running  int                      `json:"running"`
	Stopped  int                      `json:"stopped"`
	Total    int                      `json:"total"`
}

// NodeSnapshot 一台 Docker 主机的汇总（本机或 SSH 从机）。
type NodeSnapshot struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Local   bool   `json:"local"`
	Error   string `json:"error,omitempty"`
	Running int    `json:"running"`
	Stopped int    `json:"stopped"`
	Total   int    `json:"total"`
}

func summarizeServices(list []dockerx.ComposeService) (running, stopped int) {
	for _, s := range list {
		if containerRunning(s) {
			running++
		} else {
			stopped++
		}
	}
	return running, stopped
}

func containerRunning(s dockerx.ComposeService) bool {
	st := strings.ToLower(strings.TrimSpace(s.State))
	if st == "running" || st == "up" {
		return true
	}
	status := strings.ToLower(s.Status)
	return strings.HasPrefix(status, "up")
}

func sortServices(list []dockerx.ComposeService) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Local != list[j].Local {
			return list[i].Local
		}
		if list[i].NodeIP != list[j].NodeIP {
			return list[i].NodeIP < list[j].NodeIP
		}
		ri, rj := containerRunning(list[i]), containerRunning(list[j])
		if ri != rj {
			return ri
		}
		ni := list[i].Service
		if ni == "" {
			ni = list[i].Name
		}
		nj := list[j].Service
		if nj == "" {
			nj = list[j].Name
		}
		return ni < nj
	})
}

// QueryServices 查询运行状态：优先 rendered compose，否则回退 docker ps（现场逐步部署）。
func QueryServices(composeRoot string) (ServiceListResult, error) {
	d := dockerx.New()
	out := ServiceListResult{}

	if composeRoot != "" && util.DirExists(composeRoot) {
		composeFile := findComposeProject(composeRoot)
		if composeFile != "" {
			dir := filepath.Dir(composeFile)
			file := filepath.Base(composeFile)
			list, err := d.ComposePs(dir, file)
			if err == nil && len(list) > 0 {
				sortServices(list)
				run, stop := summarizeServices(list)
				out.Services = list
				out.Source = "rendered"
				out.Running, out.Stopped, out.Total = run, stop, len(list)
				return out, nil
			}
		}
	}

	list, err := d.ListContainers()
	if err != nil {
		return out, err
	}
	sortServices(list)
	run, stop := summarizeServices(list)
	out.Services = list
	out.Source = "docker"
	out.Running, out.Stopped, out.Total = run, stop, len(list)
	if len(list) == 0 {
		out.Warning = "未发现运行中的容器。若使用现场逐步部署，请确认各模块 compose up 已执行。"
	} else {
		out.Warning = "现场逐步部署模式：无 rendered compose 项目，已用 docker ps 汇总。"
	}
	return out, nil
}

func findComposeProject(root string) string {
	if root == "" || !util.DirExists(root) {
		return ""
	}
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml"} {
		p := filepath.Join(root, name)
		if util.FileExists(p) {
			return p
		}
	}
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		base := info.Name()
		if base == "docker-compose.yml" || base == "docker-compose.yaml" || base == "compose.yml" {
			found = path
		}
		return nil
	})
	return found
}

// ShowStatus 表格输出服务运行状态。
func ShowStatus(site *config.SiteConfig, composeDir string) error {
	res, err := QueryServices(composeDir)
	if err != nil {
		return fmt.Errorf("获取服务状态失败: %w", err)
	}
	list := res.Services
	if res.Warning != "" {
		fmt.Println("提示:", res.Warning)
	}
	fmt.Printf("%-24s %-12s %-16s %-40s\n", "服务", "状态", "健康", "镜像")
	fmt.Println("--------------------------------------------------------------------------------")
	for _, s := range list {
		name := s.Service
		if name == "" {
			name = s.Name
		}
		fmt.Printf("%-24s %-12s %-16s %-40s\n", name, s.State, s.Health, s.Image)
	}
	if site != nil {
		st, err := state.NewStore()
		if err == nil {
			if latest, _ := st.LatestSuccess(site.Site.Code); latest != nil {
				fmt.Printf("\n当前版本: %s（%s）\n", latest.PackageVer, latest.Time.Format(time.RFC3339))
			}
		}
	}
	return nil
}

// ShowLogs 输出指定服务日志。
func ShowLogs(service string, tail int, follow bool) error {
	d := dockerx.New()
	out, err := d.Logs(service, tail, follow)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

// DiagOptions 诊断包选项。
type DiagOptions struct {
	Site       *config.SiteConfig
	Manifest   *config.Manifest
	ComposeDir string
	RenderDir  string
	Output     string
}

// CollectDiag 一键打包诊断材料。
func CollectDiag(opts DiagOptions) (string, error) {
	out := opts.Output
	if out == "" {
		out = fmt.Sprintf("wpgctl-diag-%s.tar.gz", time.Now().Format("20060102150405"))
	}
	tmp, err := os.MkdirTemp("", "wpgctl-diag-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	// precheck
	rep, err := precheck.Run(precheck.Options{Site: opts.Site, Manifest: opts.Manifest})
	if err == nil {
		_ = precheck.WriteReportFile(filepath.Join(tmp, "precheck.json"), rep)
	}

	// compose ps
	d := dockerx.New()
	if list, err := d.ComposePs(opts.ComposeDir, ""); err == nil {
		data, _ := json.MarshalIndent(list, "", "  ")
		_ = os.WriteFile(filepath.Join(tmp, "compose-ps.json"), data, 0o644)
		for _, s := range list {
			name := s.Name
			if name == "" {
				name = s.Service
			}
			logs, _ := d.Logs(name, 200, false)
			_ = os.WriteFile(filepath.Join(tmp, "logs-"+name+".txt"), []byte(logs), 0o644)
		}
	}

	// 部署历史
	if st, err := state.NewStore(); err == nil {
		if list, err := st.List(); err == nil {
			data, _ := json.MarshalIndent(list, "", "  ")
			_ = os.WriteFile(filepath.Join(tmp, "deployments.json"), data, 0o644)
		}
	}

	// 渲染配置（敏感字段打码）
	if opts.RenderDir != "" && util.DirExists(opts.RenderDir) {
		_ = copyMasked(opts.RenderDir, filepath.Join(tmp, "rendered"))
	}

	if err := tarGzDir(tmp, out); err != nil {
		return "", err
	}
	util.Successf("诊断包已生成: %s", out)
	return out, nil
}

func copyMasked(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		dest := filepath.Join(dst, rel)
		if info.IsDir() {
			return util.EnsureDir(dest)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		masked := maskSecrets(string(data))
		_ = util.EnsureDir(filepath.Dir(dest))
		return os.WriteFile(dest, []byte(masked), 0o644)
	})
}

func maskSecrets(s string) string {
	keys := []string{"password", "PASSWORD", "secret", "SECRET", "requirepass"}
	lines := splitLines(s)
	for i, line := range lines {
		for _, k := range keys {
			if containsFold(line, k) && (containsAny(line, "=") || containsAny(line, ":")) {
				lines[i] = maskLine(line)
				break
			}
		}
	}
	return joinLines(lines)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	n := 0
	for _, l := range lines {
		n += len(l) + 1
	}
	b := make([]byte, 0, n)
	for i, l := range lines {
		b = append(b, l...)
		if i < len(lines)-1 {
			b = append(b, '\n')
		}
	}
	return string(b)
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		findFold(s, sub))
}

func findFold(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		ok := true
		for j := 0; j < len(sub); j++ {
			a, b := s[i+j], sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func containsAny(s string, chars string) bool {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return true
			}
		}
	}
	return false
}

func maskLine(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] == '=' || line[i] == ':' {
			return line[:i+1] + " ******"
		}
	}
	return "******"
}

func tarGzDir(src, dest string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, file)
		file.Close()
		return err
	})
}
