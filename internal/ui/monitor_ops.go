package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/fetch"
	"github.com/wpg/wpgctl/internal/moduledeploy"
	"github.com/wpg/wpgctl/internal/remotedeploy"
	"github.com/wpg/wpgctl/internal/util"
)

// handleMonitorLayout 解析已选 platform 根下的 monitor 目录和可改文件。
func (s *Server) handleMonitorLayout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	root := strings.TrimSpace(r.URL.Query().Get("root"))
	if root == "" {
		s.writeJSON(w, 400, map[string]string{"error": "请提供 platform 根目录"})
		return
	}
	dir := moduledeploy.ModulePath(root, "monitor")
	if !util.DirExists(dir) {
		s.writeJSON(w, 200, map[string]any{"dir": dir, "exists": false, "files": []moduledeploy.MonitorFile{}})
		return
	}
	center, files, err := moduledeploy.ListMonitorFiles(dir)
	if err != nil {
		s.writeJSON(w, 200, map[string]any{"dir": dir, "exists": true, "error": err.Error(), "files": []moduledeploy.MonitorFile{}})
		return
	}
	s.writeJSON(w, 200, map[string]any{"dir": center, "exists": true, "files": files})
}

// handleMonitorPatch 按站点改监控 .env、prometheus.yml 和采集 compose，不启动容器。
func (s *Server) handleMonitorPatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		ModuleRoot string   `json:"moduleRoot"`
		Only       []string `json:"only"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	root := strings.TrimSpace(body.ModuleRoot)
	if root == "" {
		s.writeJSON(w, 400, map[string]string{"error": "请填写 platform 根目录"})
		return
	}
	dir := moduledeploy.ModulePath(root, "monitor")
	site, err := config.LoadSite(s.opts.SitePath)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	lines, err := s.patchMonitorAddresses(site, dir, body.Only)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "dir": dir, "lines": lines})
}

// patchMonitorAddresses 改地址。only 为空则改 .env、prometheus.yml 和已勾选的采集 compose。
func (s *Server) patchMonitorAddresses(site *config.SiteConfig, monitorDir string, only []string) ([]string, error) {
	center, promDir, pieces, err := moduledeploy.LocateMonitorPieces(monitorDir)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, id := range only {
		want[id] = true
	}
	all := len(want) == 0
	centerIP := monitorCenterIP(site)
	var lines []string
	if (all || want["env"]) && util.FileExists(filepath.Join(center, ".env")) {
		changed, eerr := moduledeploy.PatchMonitorEnv(filepath.Join(center, ".env"), site, centerIP)
		if eerr != nil {
			return lines, eerr
		}
		lines = append(lines, "已更新监控 .env: "+strings.Join(changed, ", "))
	}
	if all || want["prometheus-yml"] {
		if promDir != "" {
			cfg := filepath.Join(promDir, "config", "prometheus.yml")
			if !util.FileExists(cfg) {
				cfg = filepath.Join(filepath.Dir(promDir), "config", "prometheus.yml")
			}
			if util.FileExists(cfg) {
				if eerr := moduledeploy.PatchPrometheusSD(cfg, centerIP); eerr != nil {
					return lines, eerr
				}
				lines = append(lines, "已更新 Prometheus http_sd → "+centerIP+":18099")
			}
		}
	}
	names := []string{"kafka", "mysql", "pgsql", "redis"}
	if !all {
		names = nil
		for _, id := range []string{"kafka", "mysql", "pgsql", "redis"} {
			if want[id] {
				names = append(names, id)
			}
		}
	}
	for _, name := range names {
		dir := pieces[name]
		if dir == "" {
			lines = append(lines, "未找到 "+name+" compose，跳过")
			continue
		}
		host, port, pass := exporterTarget(site, name)
		compose := composeFile(dir)
		if compose == "" || host == "" {
			lines = append(lines, name+" 无 compose 或目标地址，跳过")
			continue
		}
		if eerr := moduledeploy.PatchExporterCompose(compose, name, host, port, pass); eerr != nil {
			return lines, eerr
		}
		lines = append(lines, fmt.Sprintf("已更新 %s compose → %s:%d", name, host, port))
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("没有可修改的文件")
	}
	return lines, nil
}

func composeFile(dir string) string {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml"} {
		p := filepath.Join(dir, name)
		if util.FileExists(p) {
			return p
		}
	}
	return ""
}

// handleMonitorExpand 解开 otherServices 下的 zip，并展开其中的 tar.zip。
func (s *Server) handleMonitorExpand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		ModuleRoot string `json:"moduleRoot"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	root := strings.TrimSpace(body.ModuleRoot)
	if root == "" {
		s.writeJSON(w, 400, map[string]string{"error": "请填写 platform 根目录"})
		return
	}
	monitorDir := moduledeploy.ModulePath(root, "monitor")
	if !util.DirExists(monitorDir) {
		s.writeJSON(w, 400, map[string]string{"error": "监控目录不存在: " + monitorDir})
		return
	}
	job := s.newJob("monitor-expand")
	go func() {
		s.appendLog(job, "解压 otherServices …")
		steps, err := expandMonitorBundles(monitorDir)
		for _, line := range steps {
			s.appendLog(job, line)
		}
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		s.okJob(job, "采集包已解压")
	}()
	s.writeJSON(w, 202, job)
}

// expandMonitorBundles 解 otherServices 的 zip，并展开里面的 tar.zip。
// 监控根目录已有镜像 tar 时，整包解压会直接跳过，所以采集包要单独解。
func expandMonitorBundles(monitorDir string) ([]string, error) {
	var steps []string
	if svc := moduledeploy.OtherServicesDir(monitorDir); svc != "" {
		res, err := fetchExpand(svc)
		if res != nil {
			steps = append(steps, res...)
		}
		if err != nil && !errors.Is(err, fetch.ErrNoArchivesFound) {
			return steps, err
		}
	}
	more, err := moduledeploy.PrepareModuleArchives(monitorDir)
	steps = append(steps, more...)
	if err != nil {
		return steps, err
	}
	return steps, nil
}

func fetchExpand(dir string) ([]string, error) {
	res, err := fetch.ExpandArchives(dir)
	if res == nil {
		return nil, err
	}
	return res.Steps, err
}

// handleMonitorDeploy 部署监控中心、Prometheus，以及按机器分发的采集端。
func (s *Server) handleMonitorDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		ModuleRoot  string   `json:"moduleRoot"`
		Agents      []string `json:"agents"`
		Exporters   []string `json:"exporters"`
		SSHPassword string   `json:"sshPassword"`
		SSHKeyPath  string   `json:"sshKeyPath"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	root := strings.TrimSpace(body.ModuleRoot)
	if root == "" {
		s.writeJSON(w, 400, map[string]string{"error": "请填写 platform 根目录"})
		return
	}
	monitorDir := moduledeploy.ModulePath(root, "monitor")
	if !util.DirExists(monitorDir) {
		s.writeJSON(w, 400, map[string]string{"error": "监控目录不存在: " + monitorDir})
		return
	}
	job := s.newJob("monitor-deploy")
	go func() {
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		s.appendLog(job, "解压监控包…")
		steps, err := expandMonitorBundles(monitorDir)
		for _, line := range steps {
			s.appendLog(job, line)
		}
		if err != nil {
			s.appendLog(job, "WARN: "+err.Error())
		}
		center, promDir, pieces, err := moduledeploy.LocateMonitorPieces(monitorDir)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		centerIP := monitorCenterIP(site)
		if centerIP == "" {
			s.failJob(job, "没有可用的监控中心机器")
			return
		}
		envPath := filepath.Join(center, ".env")
		if util.FileExists(envPath) {
			changed, eerr := moduledeploy.PatchMonitorEnv(envPath, site, centerIP)
			if eerr != nil {
				s.failJob(job, eerr.Error())
				return
			}
			s.appendLog(job, "已更新监控 .env: "+strings.Join(changed, ", "))
		}
		if promDir != "" {
			cfg := filepath.Join(promDir, "config", "prometheus.yml")
			if !util.FileExists(cfg) {
				cfg = filepath.Join(filepath.Dir(promDir), "config", "prometheus.yml")
			}
			if util.FileExists(cfg) {
				if eerr := moduledeploy.PatchPrometheusSD(cfg, centerIP); eerr != nil {
					s.failJob(job, eerr.Error())
					return
				}
				s.appendLog(job, "已更新 Prometheus http_sd → "+centerIP+":18099")
			} else {
				s.appendLog(job, "WARN: 未找到 prometheus.yml")
			}
		}
		agents := cleanIPs(body.Agents)
		if len(agents) == 0 {
			agents = cleanIPs(site.Monitor.Agents)
		}
		if len(agents) == 0 {
			for _, n := range site.Nodes {
				if ip := strings.TrimSpace(n.IP); ip != "" {
					agents = append(agents, ip)
				}
			}
		}
		exporters := body.Exporters
		if exporters == nil {
			exporters = site.Monitor.Exporters
		}
		for _, name := range exporters {
			dir := pieces[name]
			if dir == "" {
				s.appendLog(job, "WARN: 未找到 "+name+" 采集目录，跳过")
				continue
			}
			host, port, pass := exporterTarget(site, name)
			compose := filepath.Join(dir, "docker-compose.yml")
			if !util.FileExists(compose) {
				compose = filepath.Join(dir, "docker-compose.yaml")
			}
			if util.FileExists(compose) && host != "" {
				if eerr := moduledeploy.PatchExporterCompose(compose, name, host, port, pass); eerr != nil {
					s.failJob(job, eerr.Error())
					return
				}
				s.appendLog(job, fmt.Sprintf("已更新 %s 采集目标 → %s:%d", name, host, port))
			}
		}

		var units []moduledeploy.MonitorPiece
		units = append(units, moduledeploy.MonitorPiece{Name: "监控平台", Dir: center, Host: centerIP})
		if promDir != "" {
			units = append(units, moduledeploy.MonitorPiece{Name: "Prometheus", Dir: promDir, Host: centerIP})
		}
		for _, ip := range agents {
			if dir := pieces["node"]; dir != "" {
				units = append(units, moduledeploy.MonitorPiece{Name: "node", Dir: dir, Host: ip})
			}
			if dir := pieces["cadvisor"]; dir != "" {
				units = append(units, moduledeploy.MonitorPiece{Name: "cadvisor", Dir: dir, Host: ip})
			}
		}
		for _, name := range exporters {
			dir := pieces[name]
			if dir == "" {
				continue
			}
			host, _, _ := exporterTarget(site, name)
			if host == "" {
				host = centerIP
			}
			units = append(units, moduledeploy.MonitorPiece{Name: name + " exporter", Dir: dir, Host: host})
		}
		if pieces["node"] == "" {
			s.appendLog(job, "WARN: 未找到 node 采集目录")
		}
		if pieces["cadvisor"] == "" {
			s.appendLog(job, "WARN: 未找到 cadvisor 采集目录")
		}

		for _, u := range units {
			s.appendLog(job, fmt.Sprintf("—— %s → %s ——", u.Name, u.Host))
			if err := s.deployMonitorPiece(job, site, u, body.SSHPassword, body.SSHKeyPath); err != nil {
				s.failJob(job, u.Name+" 失败: "+err.Error())
				return
			}
		}
		s.okJob(job, fmt.Sprintf("监控部署完成（中心 %s，采集 %d 台）", centerIP, len(agents)))
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) deployMonitorPiece(job *Job, site *config.SiteConfig, u moduledeploy.MonitorPiece, sshPassword, sshKey string) error {
	opts := moduledeploy.Options{
		ModuleDir: u.Dir,
		Site:      site,
		Expand:    true,
		Load:      true,
		PatchEnv:  false,
		ComposeUp: true,
	}
	node, ok := remotedeploy.FindNode(site, u.Host)
	if ok && remotedeploy.IsRemote(node) {
		_, err := remotedeploy.RunModule(
			remotedeploy.Target{Node: node, SSHPassword: sshPassword, SSHKeyPath: sshKey, SitePath: s.opts.SitePath},
			remotedeploy.ModuleOptions{
				ModuleDir: u.Dir,
				Expand:    true,
				Load:      true,
				PatchEnv:  false,
				ComposeUp: true,
				SyncFiles: true,
				ForceSync: true,
			},
			func(line string) { s.appendLog(job, line) },
		)
		return err
	}
	s.appendLog(job, u.Name+" 在本机执行")
	res, err := moduledeploy.Run(opts)
	if res != nil {
		for _, step := range res.Steps {
			s.appendLog(job, step)
		}
	}
	return err
}

func monitorCenterIP(site *config.SiteConfig) string {
	for _, n := range site.Nodes {
		for _, svc := range n.Services {
			if strings.EqualFold(svc, "monitor") && strings.TrimSpace(n.IP) != "" {
				return strings.TrimSpace(n.IP)
			}
		}
	}
	if len(site.Nodes) > 0 {
		return strings.TrimSpace(site.Nodes[0].IP)
	}
	return ""
}

func exporterTarget(site *config.SiteConfig, name string) (host string, port int, pass string) {
	m := site.Middleware
	switch name {
	case "kafka":
		return strings.TrimSpace(m.Kafka.Host), portOr(m.Kafka.Port, 9092), ""
	case "redis":
		return strings.TrimSpace(m.Redis.Host), portOr(m.Redis.Port, 6377), m.Redis.Password
	case "mysql":
		return strings.TrimSpace(m.MySQL.Host), portOr(m.MySQL.Port, 3306), m.MySQL.Password
	case "pgsql":
		return strings.TrimSpace(m.PgSQL.Host), portOr(m.PgSQL.Port, 5433), m.PgSQL.Password
	default:
		return "", 0, ""
	}
}

func portOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func cleanIPs(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, ip := range in {
		ip = strings.TrimSpace(ip)
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	return out
}
