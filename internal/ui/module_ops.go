package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
	fw "github.com/wpg/wpgctl/internal/firewall"
	"github.com/wpg/wpgctl/internal/moduledeploy"
	"github.com/wpg/wpgctl/internal/nacos"
)

func (s *Server) handleModuleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	phase := r.URL.Query().Get("phase")
	s.writeJSON(w, 200, map[string]any{
		"phase":   phase,
		"modules": moduledeploy.Modules(moduledeploy.Phase(phase)),
	})
}

func (s *Server) handleModuleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		ModuleDir       string `json:"moduleDir"`
		ModuleRoot      string `json:"moduleRoot"`
		ModuleName      string `json:"moduleName"`
		Expand          bool   `json:"expand"`
		Load            bool   `json:"load"`
		PatchEnv        bool   `json:"patchEnv"`
		ComposeUp       bool   `json:"composeUp"`
		ComposeBuild    bool   `json:"composeBuild"`
		NacosConfigDir  string `json:"nacosConfigDir"`
		AutoNacosImport bool   `json:"autoNacosImport"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	moduleDir := strings.TrimSpace(body.ModuleDir)
	if body.ModuleRoot != "" && body.ModuleName != "" {
		moduleDir = moduledeploy.ModulePath(body.ModuleRoot, body.ModuleName)
	}
	if moduleDir == "" {
		s.writeJSON(w, 400, map[string]string{"error": "moduleDir 不能为空（或提供 moduleRoot + moduleName）"})
		return
	}
	if !util.DirExists(moduleDir) {
		s.writeJSON(w, 400, map[string]string{"error": fmt.Sprintf("模块目录不存在: %s", moduleDir)})
		return
	}
	if body.ComposeBuild {
		body.ComposeUp = true
		body.Expand = false
		body.Load = false
	} else if !body.Expand && !body.Load && !body.PatchEnv && !body.ComposeUp {
		body.Expand, body.Load, body.ComposeUp = true, true, true
	}

	job := s.newJob("module-deploy")
	go func() {
		s.appendLog(job, "模块部署: "+moduleDir)
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		if body.ComposeBuild {
			s.appendLog(job, "模式: compose up -d --build（从 jar/Dockerfile 构建，跳过 tar load）")
		}
		res, err := moduledeploy.Run(moduledeploy.Options{
			ModuleDir:    moduleDir,
			Site:         site,
			Expand:       body.Expand,
			Load:         body.Load,
			PatchEnv:     body.PatchEnv,
			ComposeUp:    body.ComposeUp,
			ComposeBuild: body.ComposeBuild,
		})
		job.Result = res
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		for _, step := range res.Steps {
			s.appendLog(job, step)
		}
		sqls := moduledeploy.ListInitSQL(moduleDir)
		if len(sqls) > 0 {
			s.appendLog(job, "提示: 请确认数据库 init SQL 已执行或容器 init 挂载已生效")
			for _, q := range sqls {
				s.appendLog(job, "  SQL: "+q)
			}
		}
		if body.AutoNacosImport && strings.EqualFold(body.ModuleName, "nacos") && body.NacosConfigDir != "" {
			s.appendLog(job, "Nacos 已启动，开始导入配置…")
			importRes, ierr := nacos.Run(nacos.Options{Site: site, ConfigDir: body.NacosConfigDir})
			job.Result = map[string]any{"deploy": res, "nacosImport": importRes}
			if ierr != nil {
				s.failJob(job, "Nacos 导入失败: "+ierr.Error())
				return
			}
			s.appendLog(job, "Nacos 配置导入完成")
			s.okJob(job, "模块部署 + Nacos 导入完成")
			return
		}
		s.okJob(job, "模块部署完成")
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) handleModulePatchEnv(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Root string `json:"root"` // 模块目录或 platform 根（递归 patch）
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Root == "" {
		s.writeJSON(w, 400, map[string]string{"error": "root 不能为空"})
		return
	}
	site, err := config.LoadSite(s.opts.SitePath)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	changed, err := moduledeploy.PatchEnvTree(body.Root, site)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true, "changed": changed, "count": len(changed)})
}

func (s *Server) handleNginxPatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		NginxDir   string `json:"nginxDir"`
		GatewayIP  string `json:"gatewayIp"`
		AppIP      string `json:"appIp"`
		GraphIP    string `json:"graphIp"`
		ExpandHTML bool   `json:"expandHtml"`
		ComposeUp  bool   `json:"composeUp"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.NginxDir == "" {
		s.writeJSON(w, 400, map[string]string{"error": "nginxDir 不能为空"})
		return
	}
	site, err := config.LoadSite(s.opts.SitePath)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	gw := body.GatewayIP
	if gw == "" && len(site.Nodes) > 0 {
		gw = site.Nodes[0].IP
	}
	layout := moduledeploy.ResolveNginxLayout(body.NginxDir)
	moduleDir := layout.ModuleDir
	if filepath.Base(body.NginxDir) == "conf" || filepath.Base(body.NginxDir) == "conf.d" {
		moduleDir = filepath.Dir(filepath.Dir(body.NginxDir))
		layout = moduledeploy.ResolveNginxLayout(moduleDir)
	}

	job := s.newJob("nginx-patch")
	go func() {
		if body.ExpandHTML {
			s.appendLog(job, "解压 nginx 模块 tar.zip / load 镜像 / 解压 html zip…")
		}
		s.appendLog(job, "更新 http-web-8877.conf，网关="+gw)
		patchRes, err := moduledeploy.RunNginxPatch(moduledeploy.NginxPatchOptions{
			Layout:         layout,
			ExpandArchives: body.ExpandHTML,
			ProxyIPs: moduledeploy.NginxProxyIPs{
				Gateway: gw,
				App:     body.AppIP,
				Graph:   body.GraphIP,
			},
			ComposeUp: body.ComposeUp,
		})
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		for _, step := range patchRes.Steps {
			s.appendLog(job, step)
		}
		for _, n := range patchRes.ProxyNotes {
			s.appendLog(job, "proxy_pass "+n)
		}
		if len(patchRes.ProxyNotes) == 0 {
			s.appendLog(job, "proxy_pass 无变更（可能已是目标 IP）")
		}
		job.Result = map[string]any{
			"webConf": layout.WebConf,
			"htmlDir": layout.HTMLDir,
			"changes": patchRes.ProxyNotes,
			"expand":  patchRes.ExpandHTML,
			"compose": patchRes.Compose,
		}
		s.okJob(job, "Nginx 配置已更新")
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) handleFirewallPorts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		var mf *config.Manifest
		manifest := r.URL.Query().Get("manifest")
		if manifest == "" && r.URL.Query().Get("package") != "" {
			manifest = filepath.Join(r.URL.Query().Get("package"), "manifest.yaml")
		}
		if manifest != "" {
			mf, _ = config.LoadManifest(manifest)
		}
		ports := moduledeploy.PortsForSite(site, mf)
		st := fw.Inspect()
		s.writeJSON(w, 200, map[string]any{
			"ports":           ports,
			"firewall":        st.Tool,
			"firewallRunning": st.Running,
			"firewallDetail":  st.Detail,
			"count":           len(ports),
		})
	case http.MethodPost:
		var body struct {
			Manifest  string `json:"manifest"`
			Package   string `json:"package"`
			Ports     []int  `json:"ports"`
			AutoStart bool   `json:"autoStart"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		var ports []int
		if len(body.Ports) > 0 {
			ports = body.Ports
		} else {
			var mf *config.Manifest
			manifest := body.Manifest
			if manifest == "" && body.Package != "" {
				manifest = filepath.Join(body.Package, "manifest.yaml")
			}
			if manifest != "" {
				mf, _ = config.LoadManifest(manifest)
			}
			ports = moduledeploy.PortsForSite(site, mf)
		}
		job := s.newJob("firewall-open")
		go func() {
			res, err := fw.OpenPortsWithOptions(fw.OpenOptions{Ports: ports, AutoStart: body.AutoStart})
			job.Result = res
			if res != nil {
				for _, msg := range res.Messages {
					s.appendLog(job, msg)
				}
				s.appendLog(job, fmt.Sprintf("计划 %d 个，新开 %d，已有 %d，失败 %d",
					len(res.Ports), len(res.Opened), len(res.Skipped), len(res.Failed)))
				if res.Reloaded {
					s.appendLog(job, "规则已生效（firewalld reload 完成）")
				}
			}
			if err != nil {
				s.failJob(job, err.Error())
				return
			}
			s.okJob(job, "端口放行完成")
		}()
		s.writeJSON(w, 202, job)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) handleFirewallStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	job := s.newJob("firewall-start")
	go func() {
		st, err := fw.Start()
		job.Result = st
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		s.appendLog(job, fmt.Sprintf("防火墙 %s 已启动 (%s)", st.Tool, st.Detail))
		s.okJob(job, "防火墙已启动")
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) handleNacosImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		ConfigDir string `json:"configDir"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.ConfigDir == "" {
		s.writeJSON(w, 400, map[string]string{"error": "configDir 不能为空"})
		return
	}

	job := s.newJob("nacos-import")
	go func() {
		s.appendLog(job, "导入 Nacos 配置: "+body.ConfigDir)
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		s.appendLog(job, fmt.Sprintf("Nacos 目标: http://%s:%d namespace=%s user=%s",
			site.Middleware.Nacos.Host, site.Middleware.Nacos.Port,
			site.Middleware.Nacos.Namespace, site.Middleware.Nacos.Username))
		res, err := nacos.Run(nacos.Options{Site: site, ConfigDir: body.ConfigDir})
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		job.Result = res
		s.appendLog(job, "创建/更新完成")
		s.okJob(job, "Nacos 导入完成")
	}()
	s.writeJSON(w, 202, job)
}
