package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	fw "github.com/wpg/wpgctl/internal/firewall"
	"github.com/wpg/wpgctl/internal/moduledeploy"
	"github.com/wpg/wpgctl/internal/nacos"
	"github.com/wpg/wpgctl/internal/remotedeploy"
	"github.com/wpg/wpgctl/internal/util"
)

func nacosZipsIf(enabled bool, zips []string) []string {
	if !enabled {
		return nil
	}
	var out []string
	for _, z := range zips {
		z = strings.TrimSpace(z)
		if z != "" {
			out = append(out, z)
		}
	}
	return out
}

func (s *Server) importNacosAfterDeploy(job *Job, site *config.SiteConfig, zips []string, moduleName string) bool {
	if len(zips) == 0 {
		if strings.EqualFold(moduleName, "nacos") {
			s.appendLog(job, "Nacos 已部署。请立刻选择 nacos*.zip 并点「导入配置」——未导入则后续平台、市政水厂会因拉不到配置而报错。")
		}
		return false
	}
	s.appendLog(job, fmt.Sprintf("等待 Nacos 可登录后上传导入 %d 个 zip（不解压）…", len(zips)))
	for _, z := range zips {
		s.appendLog(job, "  "+filepath.Base(z))
	}
	importRes, ierr := nacos.Run(nacos.Options{Site: site, ConfigZips: zips})
	if ierr != nil {
		s.appendLog(job, "WARN: Nacos 导入失败: "+ierr.Error())
		s.appendLog(job, "请在 Nacos 行下方点「导入配置」重试。未导入则后续平台、市政水厂会报错。")
		return false
	}
	if importRes != nil && len(importRes.Imported) > 0 {
		s.appendLog(job, "已导入: "+strings.Join(importRes.Imported, ", "))
	}
	s.appendLog(job, "Nacos 配置导入完成")
	return true
}

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

// handleModulePath 解析套层包内的模块目录（middleware/middleware/nginx 等）。
func (s *Server) handleModulePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	root := strings.TrimSpace(r.URL.Query().Get("root"))
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if root == "" || name == "" {
		s.writeJSON(w, 400, map[string]string{"error": "请提供 root 与 name"})
		return
	}
	path := moduledeploy.ModulePath(root, name)
	s.writeJSON(w, 200, map[string]any{
		"root":   root,
		"name":   name,
		"path":   path,
		"exists": util.DirExists(path),
	})
}

func (s *Server) handleModuleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		ModuleDir       string   `json:"moduleDir"`
		ModuleRoot      string   `json:"moduleRoot"`
		ModuleName      string   `json:"moduleName"`
		Expand          bool     `json:"expand"`
		Load            bool     `json:"load"`
		PatchEnv        bool     `json:"patchEnv"`
		ComposeUp       bool     `json:"composeUp"`
		ComposeBuild    bool     `json:"composeBuild"`
		NacosConfigZips []string `json:"nacosConfigZips"`
		AutoNacosImport bool     `json:"autoNacosImport"`
		// 多机：目标节点（site.yaml 中的 name 或 ip）；为空或为本机时在主控机本地执行
		Node        string `json:"node"`
		SSHPassword string `json:"sshPassword"`
		SSHKeyPath  string `json:"sshKeyPath"`
		SyncFiles   *bool  `json:"syncFiles"` // 目标机缺少模块目录时自动上传（默认 true）
		ForceSync   bool   `json:"forceSync"`
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
	syncFiles := body.SyncFiles == nil || *body.SyncFiles
	nacosZips := nacosZipsIf(body.AutoNacosImport && strings.EqualFold(body.ModuleName, "nacos"), body.NacosConfigZips)

	job := s.newJob("module-deploy")
	go func() {
		s.appendLog(job, "模块部署: "+moduleDir)
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}

		// 多机：目标为其他机器时走 SSH 分发；zip 导入在主控机对 Nacos HTTP 上传（不解压）
		if target, ok := remotedeploy.FindNode(site, body.Node); ok && remotedeploy.IsRemote(target) {
			res, rerr := remotedeploy.RunModule(
				remotedeploy.Target{Node: target, SSHPassword: body.SSHPassword, SSHKeyPath: body.SSHKeyPath, SitePath: s.opts.SitePath},
				remotedeploy.ModuleOptions{
					ModuleDir:    moduleDir,
					Expand:       body.Expand,
					Load:         body.Load,
					PatchEnv:     body.PatchEnv,
					ComposeUp:    body.ComposeUp,
					ComposeBuild: body.ComposeBuild,
					SyncFiles:    syncFiles,
					ForceSync:    body.ForceSync,
				},
				func(line string) { s.appendLog(job, line) },
			)
			job.Result = res
			if rerr != nil {
				s.failJob(job, rerr.Error())
				return
			}
			imported := s.importNacosAfterDeploy(job, site, nacosZips, body.ModuleName)
			if imported {
				s.okJob(job, fmt.Sprintf("模块已在 %s (%s) 部署并导入 Nacos 配置", target.Name, target.IP))
				return
			}
			s.okJob(job, fmt.Sprintf("模块已在 %s (%s) 部署完成", target.Name, target.IP))
			return
		}
		if body.Node != "" {
			s.appendLog(job, "目标机器为本机，直接在主控机执行")
		}
		if body.ComposeBuild {
			s.appendLog(job, "模式: compose up -d --build（先 load java8.tar 等基础镜像，再从 jar/Dockerfile 构建）")
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
		imported := s.importNacosAfterDeploy(job, site, nacosZips, body.ModuleName)
		if imported {
			s.okJob(job, "模块部署 + Nacos zip 导入完成")
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
		NginxDir       string `json:"nginxDir"`
		GatewayIP      string `json:"gatewayIp"`
		AppIP          string `json:"appIp"`
		GraphIP        string `json:"graphIp"`
		ExpandHTML     bool   `json:"expandHtml"`
		ComposeUp      bool   `json:"composeUp"`
		SkipProxyPatch bool   `json:"skipProxyPatch"`
		// 多机：目标节点与 SSH 凭据
		Node        string `json:"node"`
		SSHPassword string `json:"sshPassword"`
		SSHKeyPath  string `json:"sshKeyPath"`
		SyncFiles   *bool  `json:"syncFiles"`
		ForceSync   bool   `json:"forceSync"`
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
	if !body.SkipProxyPatch && gw == "" && len(site.Nodes) > 0 {
		gw = site.Nodes[0].IP
	}
	layout := moduledeploy.ResolveNginxLayout(body.NginxDir)
	moduleDir := layout.ModuleDir
	if filepath.Base(body.NginxDir) == "conf" || filepath.Base(body.NginxDir) == "conf.d" {
		moduleDir = filepath.Dir(filepath.Dir(body.NginxDir))
		layout = moduledeploy.ResolveNginxLayout(moduleDir)
	}
	syncFiles := body.SyncFiles == nil || *body.SyncFiles

	job := s.newJob("nginx-patch")
	go func() {
		if target, ok := remotedeploy.FindNode(site, body.Node); ok && remotedeploy.IsRemote(target) {
			res, rerr := remotedeploy.RunNginx(
				remotedeploy.Target{Node: target, SSHPassword: body.SSHPassword, SSHKeyPath: body.SSHKeyPath, SitePath: s.opts.SitePath},
				remotedeploy.NginxOptions{
					NginxDir:       moduleDir,
					GatewayIP:      gw,
					AppIP:          body.AppIP,
					GraphIP:        body.GraphIP,
					ExpandHTML:     body.ExpandHTML,
					ComposeUp:      body.ComposeUp,
					SkipProxyPatch: body.SkipProxyPatch,
					SyncFiles:      syncFiles,
					ForceSync:      body.ForceSync,
				},
				func(line string) { s.appendLog(job, line) },
			)
			job.Result = res
			if rerr != nil {
				s.failJob(job, rerr.Error())
				return
			}
			s.okJob(job, fmt.Sprintf("Nginx 已在 %s (%s) 更新完成", target.Name, target.IP))
			return
		}
		s.appendLog(job, "nginx 模块目录: "+moduleDir)
		if body.ExpandHTML {
			s.appendLog(job, "解压 nginx 模块 tar.zip / load 镜像 / 解压 html zip…")
		}
		if body.SkipProxyPatch {
			s.appendLog(job, "跳过 proxy_pass IP 同步，使用已保存的 conf")
		} else {
			s.appendLog(job, "更新 http-web-8877.conf，网关="+gw)
		}
		patchRes, err := moduledeploy.RunNginxPatch(moduledeploy.NginxPatchOptions{
			Layout:         layout,
			ExpandArchives: body.ExpandHTML,
			SkipProxyPatch: body.SkipProxyPatch,
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
		if !body.SkipProxyPatch && len(patchRes.ProxyNotes) == 0 {
			s.appendLog(job, "proxy_pass 无变更（可能已是目标 IP）")
		}
		job.Result = map[string]any{
			"webConf": layout.WebConf,
			"htmlDir": layout.HTMLDir,
			"changes": patchRes.ProxyNotes,
			"expand":  patchRes.ExpandHTML,
			"compose": patchRes.Compose,
		}
		s.okJob(job, "Nginx 部署完成")
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
		ConfigZips []string `json:"configZips"`
		ConfigDir  string   `json:"configDir"` // 兼容旧目录导入
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	zips := nacosZipsIf(true, body.ConfigZips)
	if len(zips) == 0 && strings.TrimSpace(body.ConfigDir) == "" {
		s.writeJSON(w, 400, map[string]string{"error": "请选择至少一个 nacos*.zip（或提供 configDir）"})
		return
	}

	job := s.newJob("nacos-import")
	go func() {
		if len(zips) > 0 {
			s.appendLog(job, fmt.Sprintf("上传导入 Nacos zip（不解压）共 %d 个", len(zips)))
			for _, z := range zips {
				s.appendLog(job, "  "+z)
			}
		}
		if body.ConfigDir != "" {
			s.appendLog(job, "导入 Nacos 配置目录: "+body.ConfigDir)
		}
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		s.appendLog(job, fmt.Sprintf("Nacos 目标: http://%s:%d namespace=%s user=%s",
			site.Middleware.Nacos.Host, site.Middleware.Nacos.Port,
			site.Middleware.Nacos.Namespace, site.Middleware.Nacos.Username))
		s.appendLog(job, "等待 Nacos 控制台可登录后导入…")
		res, err := nacos.Run(nacos.Options{Site: site, ConfigZips: zips, ConfigDir: body.ConfigDir})
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
