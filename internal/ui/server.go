// Package ui 提供本地 Web 控制台（方案 §3.7）：向导部署、状态、日志流。
//
// Linux 默认监听 0.0.0.0:9527（可用服务器 IP 访问）；Windows 默认 127.0.0.1:9527。
// 前端产物通过 go:embed 打进单二进制。
package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/deploy"
	"github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/initenv"
	"github.com/wpg/wpgctl/internal/precheck"
	"github.com/wpg/wpgctl/internal/state"
	"github.com/wpg/wpgctl/internal/status"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 控制台启动选项。
type Options struct {
	Listen   string // 默认见 DefaultListen()
	SitePath string
	OpenHint bool
}

// Server HTTP + WebSocket 服务。
type Server struct {
	opts   Options
	mux    *http.ServeMux
	up     websocket.Upgrader
	mu     sync.Mutex
	jobs   map[string]*Job
	logger utilWriter
}

// Job 异步任务（部署/体检等）状态。
type Job struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"`
	Status  string    `json:"status"` // pending|running|ok|fail
	Message string    `json:"message"`
	Logs    []string  `json:"logs"`
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitempty"`
	Result  any       `json:"result,omitempty"`
}

type utilWriter struct{}

// Start 启动控制台并阻塞直到上下文取消或服务退出。
func Start(ctx context.Context, opts Options) error {
	if opts.Listen == "" {
		opts.Listen = DefaultListen()
	}
	if opts.SitePath == "" {
		opts.SitePath = "site.yaml"
	}
	s := &Server{
		opts: opts,
		mux:  http.NewServeMux(),
		up:   websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		jobs: map[string]*Job{},
	}
	s.routes()

	srv := &http.Server{Addr: opts.Listen, Handler: s.mux}
	printAccessHints(opts.Listen)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/hostinfo", s.handleHostInfo)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/site", s.handleSite)
	s.mux.HandleFunc("/api/site/form", s.handleSiteForm)
	s.mux.HandleFunc("/api/fs", s.handleFS)
	s.mux.HandleFunc("/api/fs/text", s.handleFSText)
	s.mux.HandleFunc("/api/fs/expand", s.handleFSExpand)
	s.mux.HandleFunc("/api/packages", s.handlePackages)
	s.mux.HandleFunc("/api/packages/scan", s.handlePackageScan)
	s.mux.HandleFunc("/api/confirm-summary", s.handleConfirmSummary)
	s.mux.HandleFunc("/api/report", s.handleReport)
	s.mux.HandleFunc("/api/diag", s.handleDiagDownload)
	s.mux.HandleFunc("/api/deliveries/export", s.handleDeliveryExport)
	s.mux.HandleFunc("/api/precheck", s.handlePrecheck)
	s.mux.HandleFunc("/api/init", s.handleInit)
	s.mux.HandleFunc("/api/deploy", s.handleDeploy)
	s.mux.HandleFunc("/api/module/catalog", s.handleModuleCatalog)
	s.mux.HandleFunc("/api/module/deploy", s.handleModuleDeploy)
	s.mux.HandleFunc("/api/module/patch-env", s.handleModulePatchEnv)
	s.mux.HandleFunc("/api/nginx/patch", s.handleNginxPatch)
	s.mux.HandleFunc("/api/nacos/import", s.handleNacosImport)
	s.mux.HandleFunc("/api/firewall/ports", s.handleFirewallPorts)
	s.mux.HandleFunc("/api/firewall/start", s.handleFirewallStart)
	s.mux.HandleFunc("/api/preview", s.handlePreview)
	s.mux.HandleFunc("/api/upgrade", s.handleUpgrade)
	s.mux.HandleFunc("/api/rollback", s.handleRollback)
	s.mux.HandleFunc("/api/fetch", s.handleFetch)
	s.mux.HandleFunc("/api/latest", s.handleLatestDeploy)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/deployments", s.handleDeployments)
	s.mux.HandleFunc("/api/jobs/", s.handleJob)
	s.mux.HandleFunc("/api/ws/logs", s.handleWSLogs)
	s.mux.HandleFunc("/api/ws/job", s.handleWSJob)

	sub, err := fs.Sub(staticFS, "dist")
	if err != nil {
		util.Warnf("静态资源未嵌入: %v", err)
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "前端未构建：请先在 web/ 执行 npm run build", 503)
		})
		return
	}
	fileServer := http.FileServer(http.FS(sub))
	s.mux.Handle("/", spaHandler(fileServer, sub))
}

func spaHandler(fileServer http.Handler, fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path != "/" {
			if _, err := fs.Stat(fsys, stringsTrim(path)); err != nil {
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

func stringsTrim(p string) string {
	if len(p) > 0 && p[0] == '/' {
		return p[1:]
	}
	return p
}

func (s *Server) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleSite(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getSite(w)
	case http.MethodPut, http.MethodPost:
		s.saveSite(w, r)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) getSite(w http.ResponseWriter) {
	raw, err := os.ReadFile(s.opts.SitePath)
	if err != nil {
		// 文件不存在时返回示例，方便网页新建
		example := `# site.yaml — 在网页中编辑后保存即可
site:
  name: 示例水厂
  code: plant-demo
nodes:
  - name: app-node
    ip: 127.0.0.1
    ssh: { user: root, port: 22 }
    roles: [middleware, platform]
profiles: [platform, waterwork, monitor]
middleware:
  nacos: { host: 127.0.0.1, port: 8848, namespace: intergrate, username: nacos, password: "wpg@nice#LKsalk98" }
  mysql: { host: 127.0.0.1, port: 3306, user: wpg, password: "DmJme(ZFl9txW@2P" }
  pgsql: { host: 127.0.0.1, port: 5433, user: wpg, password: "t7u!m0Wpyu7EfbVN" }
  redis: { host: 127.0.0.1, port: 6377, password: "SJ(Qu%(kfXQBjxyT" }
  kafka: { host: 127.0.0.1, port: 9092 }
paths:
  workspace: /workspace/waterwork
  nginxHtml: /workspace/middleware/nginx/html
  waterwork: /workspace/waterwork-4.1.1
  intelligentModel: /workspace/wpg-intelligent-model-4.1.2
`
		s.writeJSON(w, 200, map[string]any{
			"path":    s.opts.SitePath,
			"exists":  false,
			"yaml":    example,
			"parsed":  nil,
			"message": "site.yaml 尚不存在，已提供模板，保存后将创建",
		})
		return
	}

	cfg, err := config.LoadSite(s.opts.SitePath)
	var parsed any
	if err == nil {
		safe := *cfg
		// 编辑用 raw yaml（含真实密码）；parsed 仅作摘要，密码脱敏
		safe.Middleware.Nacos.Password = mask(safe.Middleware.Nacos.Password)
		safe.Middleware.MySQL.Password = mask(safe.Middleware.MySQL.Password)
		safe.Middleware.Redis.Password = mask(safe.Middleware.Redis.Password)
		safe.Middleware.PgSQL.Password = mask(safe.Middleware.PgSQL.Password)
		parsed = safe
	}
	s.writeJSON(w, 200, map[string]any{
		"path":   s.opts.SitePath,
		"exists": true,
		"yaml":   string(raw),
		"parsed": parsed,
		"error":  errString(err),
	})
}

func (s *Server) saveSite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		YAML string `json:"yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求体无效"})
		return
	}
	if strings.TrimSpace(body.YAML) == "" {
		s.writeJSON(w, 400, map[string]string{"error": "yaml 不能为空"})
		return
	}

	// 先写临时文件再校验，避免破坏原文件
	dir := filepath.Dir(s.opts.SitePath)
	_ = os.MkdirAll(dir, 0o755)
	tmp := s.opts.SitePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(body.YAML), 0o644); err != nil {
		s.writeJSON(w, 500, map[string]string{"error": "写入失败: " + err.Error()})
		return
	}
	if _, err := config.LoadSite(tmp); err != nil {
		_ = os.Remove(tmp)
		s.writeJSON(w, 400, map[string]string{"error": "校验失败: " + err.Error()})
		return
	}
	if err := os.Rename(tmp, s.opts.SitePath); err != nil {
		// Windows 上目标存在时 rename 可能失败，改为覆盖写
		if err2 := os.WriteFile(s.opts.SitePath, []byte(body.YAML), 0o644); err2 != nil {
			s.writeJSON(w, 500, map[string]string{"error": "保存失败: " + err2.Error()})
			return
		}
		_ = os.Remove(tmp)
	}
	cfg, _ := config.LoadSite(s.opts.SitePath)
	s.writeJSON(w, 200, map[string]any{
		"ok":   true,
		"path": s.opts.SitePath,
		"site": cfg.Site,
	})
}

func (s *Server) handleFS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	path := r.URL.Query().Get("path")
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "dir"
	}
	res, err := listFS(path, mode)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, res)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func mask(s string) string {
	if s == "" {
		return ""
	}
	return "******"
}

func (s *Server) handlePrecheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Manifest string `json:"manifest"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	job := s.newJob("precheck")
	go func() {
		s.appendLog(job, "开始环境体检…")
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		var mf *config.Manifest
		if body.Manifest != "" {
			mf, err = config.LoadManifest(body.Manifest)
			if err != nil {
				s.failJob(job, err.Error())
				return
			}
		}
		rep, err := precheck.Run(precheck.Options{Site: site, Manifest: mf})
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		job.Result = rep
		if rep.HasRed {
			s.failJob(job, "存在红色项")
			return
		}
		s.okJob(job, "体检完成")
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) handleInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Base          string `json:"base"`
		DockerPackage string `json:"dockerPackage"`
		Manifest      string `json:"manifest"`
		Local         bool   `json:"local"`
		SSHPassword   string `json:"sshPassword"`
		SSHKeyPath    string `json:"sshKeyPath"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	job := s.newJob("init")
	go func() {
		s.appendLog(job, "开始环境初始化…")
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		if len(site.Nodes) > 1 && !body.Local {
			s.appendLog(job, fmt.Sprintf("多机模式：本机 init 后将 SSH 分发到 %d 台从机（目录/防火墙）", len(site.Nodes)-1))
		}
		var mf *config.Manifest
		if body.Manifest != "" {
			mf, _ = config.LoadManifest(body.Manifest)
		}
		res, err := initenv.Run(initenv.Options{
			Site: site, Manifest: mf,
			BasePackage: body.Base, DockerPackage: body.DockerPackage,
			LocalOnly: body.Local, SitePath: s.opts.SitePath,
			SSHPassword: body.SSHPassword, SSHKeyPath: body.SSHKeyPath,
		})
		if err != nil {
			job.Result = res
			s.failJob(job, err.Error())
			return
		}
		job.Result = res
		if res != nil && res.DockerVersion != "" {
			s.appendLog(job, "Docker 版本: "+res.DockerVersion)
		}
		if res != nil && len(res.NodeResults) > 0 {
			for _, nr := range res.NodeResults {
				st := "OK"
				if !nr.OK {
					st = "FAIL"
				}
				s.appendLog(job, fmt.Sprintf("节点 %s (%s): %s — %s", nr.Name, nr.IP, st, nr.Message))
			}
		}
		s.okJob(job, "初始化完成")
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Package  string `json:"package"`
		DryRun   bool   `json:"dryRun"`
		Operator string `json:"operator"`
		Scenario string `json:"scenario"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Package == "" {
		s.writeJSON(w, 400, map[string]string{"error": "package 不能为空"})
		return
	}
	job := s.newJob("deploy")
	go func() {
		s.appendLog(job, "开始部署: "+body.Package)
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		mf, err := config.LoadManifest(filepath.Join(body.Package, "manifest.yaml"))
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		res, err := deploy.Run(deploy.Options{
			Site: site, Manifest: mf, PackageDir: body.Package,
			SitePath: s.opts.SitePath, DryRun: body.DryRun,
			Operator: body.Operator, Scenario: body.Scenario,
			OnProgress: func(layer int, message string, smoke []deploy.SmokeEntry) {
				s.appendLog(job, message)
				s.mu.Lock()
				job.Result = map[string]any{"layer": layer, "smoke": smoke}
				s.mu.Unlock()
			},
		})
		job.Result = res
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		s.okJob(job, "部署完成")
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	composeDir := ""
	if site, err := config.LoadSite(s.opts.SitePath); err == nil {
		composeDir = filepath.Join(site.Paths.Workspace, "rendered")
	}
	res, err := status.QueryServices(composeDir)
	if err != nil {
		s.writeJSON(w, 200, map[string]any{"services": []any{}, "warning": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]any{
		"services": res.Services,
		"source":   res.Source,
		"warning":  res.Warning,
	})
}

func (s *Server) handleDeployments(w http.ResponseWriter, r *http.Request) {
	st, err := state.NewStore()
	if err != nil {
		s.writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	list, err := st.List()
	if err != nil {
		s.writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, list)
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	id := stringsTrim(r.URL.Path[len("/api/jobs/"):])
	s.mu.Lock()
	job := s.jobs[id]
	s.mu.Unlock()
	if job == nil {
		s.writeJSON(w, 404, map[string]string{"error": "job not found"})
		return
	}
	s.writeJSON(w, 200, job)
}

func (s *Server) handleWSLogs(w http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	if svc == "" {
		http.Error(w, "service required", 400)
		return
	}
	conn, err := s.up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	d := dockerx.New()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			out, err := d.Logs(svc, 80, false)
			if err != nil {
				_ = conn.WriteJSON(map[string]string{"error": err.Error()})
				return
			}
			if err := conn.WriteJSON(map[string]string{"logs": out}); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleWSJob(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	conn, err := s.up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		job := s.jobs[id]
		s.mu.Unlock()
		if job == nil {
			_ = conn.WriteJSON(map[string]string{"error": "not found"})
			return
		}
		if err := conn.WriteJSON(job); err != nil {
			return
		}
		if job.Status == "ok" || job.Status == "fail" {
			return
		}
	}
}

func (s *Server) newJob(typ string) *Job {
	j := &Job{
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:    typ,
		Status:  "running",
		Started: time.Now(),
		Logs:    []string{},
	}
	s.mu.Lock()
	s.jobs[j.ID] = j
	s.mu.Unlock()
	return j
}

func (s *Server) appendLog(j *Job, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Logs = append(j.Logs, fmt.Sprintf("%s %s", time.Now().Format("15:04:05"), line))
}

func (s *Server) okJob(j *Job, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Status = "ok"
	j.Message = msg
	j.Ended = time.Now()
	j.Logs = append(j.Logs, msg)
}

func (s *Server) failJob(j *Job, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Status = "fail"
	j.Message = msg
	j.Ended = time.Now()
	j.Logs = append(j.Logs, "ERROR: "+msg)
}

// EnsureSitePathExists 启动前轻量提示。
func EnsureSitePathExists(path string) {
	if _, err := os.Stat(path); err != nil {
		util.Warnf("site 文件暂不可用: %s（控制台内仍可查看 API 错误）", path)
	}
}
