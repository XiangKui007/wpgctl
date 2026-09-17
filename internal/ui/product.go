// Package ui 产品化 API：设置、包中心、验收报告、diag 下载、确认摘要。
package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/pack"
	"github.com/wpg/wpgctl/internal/status"
	"github.com/wpg/wpgctl/internal/state"
	"github.com/wpg/wpgctl/internal/util"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st, err := state.LoadSettings()
		if err != nil {
			s.writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, 200, st)
	case http.MethodPut, http.MethodPost:
		var body state.UISettings
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if body.Mode != "implementer" && body.Mode != "expert" {
			body.Mode = "implementer"
		}
		if body.Scenario != "windows" {
			body.Scenario = "linux"
		}
		if err := state.SaveSettings(&body); err != nil {
			s.writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, 200, body)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

// PackageInfo 本地包仓库条目。
type PackageInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
	HasManifest bool `json:"hasManifest"`
}

func (s *Server) handlePackages(w http.ResponseWriter, r *http.Request) {
	root := util.PackagesDir()
	_ = util.EnsureDir(root)
	var list []PackageInfo
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || !info.IsDir() {
			return nil
		}
		mf := filepath.Join(path, "manifest.yaml")
		if !util.FileExists(mf) {
			return nil
		}
		m, err := config.LoadManifest(mf)
		if err != nil {
			list = append(list, PackageInfo{Name: filepath.Base(path), Path: path, HasManifest: true})
			return filepath.SkipDir
		}
		list = append(list, PackageInfo{
			Name: filepath.Base(path), Path: path,
			Kind: m.Kind, Version: m.Version, HasManifest: true,
		})
		return filepath.SkipDir
	})
	// 也扫描常见相对目录
	for _, extra := range []string{".", "dist", "packages"} {
		entries, _ := os.ReadDir(extra)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(extra, e.Name())
			abs, _ := filepath.Abs(p)
			mf := filepath.Join(abs, "manifest.yaml")
			if !util.FileExists(mf) {
				continue
			}
			dup := false
			for _, x := range list {
				if x.Path == abs {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			m, err := config.LoadManifest(mf)
			if err != nil {
				list = append(list, PackageInfo{Name: e.Name(), Path: abs, HasManifest: true})
				continue
			}
			list = append(list, PackageInfo{
				Name: e.Name(), Path: abs, Kind: m.Kind, Version: m.Version, HasManifest: true,
			})
		}
	}
	baseReady := false
	for _, p := range list {
		if p.Kind == "base" {
			baseReady = true
			break
		}
	}
	s.writeJSON(w, 200, map[string]any{
		"packages":    list,
		"packagesDir": root,
		"baseReady":   baseReady,
		"count":       len(list),
	})
}

func (s *Server) handlePackageScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Dir     string `json:"dir"`
		Kind    string `json:"kind"`
		Version string `json:"version"`
		Arch    string `json:"arch"`
		Write   bool   `json:"write"`
		Out     string `json:"out"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(body.Dir) == "" {
		s.writeJSON(w, 400, map[string]string{"error": "dir 不能为空"})
		return
	}
	res, err := pack.ScanDir(pack.ScanOptions{
		Dir: body.Dir, Kind: body.Kind, Version: body.Version, Arch: body.Arch,
	})
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	outPath := body.Out
	if body.Write {
		if outPath == "" {
			outPath = filepath.Join(body.Dir, "manifest.yaml")
		}
		if err := pack.WriteManifest(outPath, res.YAML); err != nil {
			s.writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
	}
	s.writeJSON(w, 200, map[string]any{
		"images":      res.Images,
		"yaml":        res.YAML,
		"warnings":    res.Warnings,
		"written":     body.Write,
		"manifestPath": outPath,
		"count":       len(res.Images),
	})
}

func (s *Server) handleConfirmSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Package string `json:"package"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	site, err := config.LoadSite(s.opts.SitePath)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	summary := map[string]any{
		"siteName": site.Site.Name,
		"siteCode": site.Site.Code,
		"nodes":    len(site.Nodes),
		"profiles": site.Profiles,
		"text":     "",
	}
	svcCount := 0
	ports := []int{}
	if body.Package != "" {
		mf, err := config.LoadManifest(filepath.Join(body.Package, "manifest.yaml"))
		if err == nil {
			svcs := mf.EnabledServices(site.Profiles)
			svcCount = len(svcs)
			ports = mf.Ports(site.Profiles)
			summary["version"] = mf.Version
			summary["kind"] = mf.Kind
		}
	}
	summary["serviceCount"] = svcCount
	summary["ports"] = ports
	summary["text"] = fmt.Sprintf(
		"将在站点「%s」(%s) 启用模块 %s，部署到 %d 台机器，预计启动 %d 个服务%s。",
		site.Site.Name, site.Site.Code,
		strings.Join(profileLabels(site.Profiles), "、"),
		len(site.Nodes), svcCount,
		func() string {
			if len(ports) == 0 {
				return ""
			}
			return fmt.Sprintf("，占用端口 %v", ports)
		}(),
	)
	s.writeJSON(w, 200, summary)
}

func profileLabels(profiles []string) []string {
	m := map[string]string{
		"platform": "平台", "waterwork": "市政水厂",
		"intelligent-model": "模型服务", "device": "设备",
		"alarm": "报警", "gis": "GIS", "monitor": "监控", "graph": "组态",
	}
	out := make([]string, 0, len(profiles))
	for _, p := range profiles {
		if l, ok := m[p]; ok {
			out = append(out, l)
		} else {
			out = append(out, p)
		}
	}
	return out
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
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
	var rec *state.DeploymentRecord
	if id != "" {
		for i := range list {
			if list[i].ID == id {
				cp := list[i]
				rec = &cp
				break
			}
		}
	} else {
		for i := len(list) - 1; i >= 0; i-- {
			if list[i].Success {
				cp := list[i]
				rec = &cp
				break
			}
		}
	}
	if rec == nil {
		s.writeJSON(w, 404, map[string]string{"error": "无可用交付记录"})
		return
	}
	if r.URL.Query().Get("format") == "json" {
		s.writeJSON(w, 200, rec)
		return
	}
	html := renderAcceptanceHTML(rec)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=wpg-report-%s.html", rec.PackageVer))
	_, _ = w.Write([]byte(html))
}

func renderAcceptanceHTML(rec *state.DeploymentRecord) string {
	status := "未通过"
	color := "#b91c1c"
	if rec.Success && (rec.AcceptanceOK || len(rec.Smoke) == 0) {
		status = "通过"
		color = "#1a7f5a"
	} else if rec.Success {
		status = "部署成功（请人工复核入口）"
		color = "#0f766e"
	}
	var rows strings.Builder
	for _, s := range rec.Smoke {
		st := "FAIL"
		if s.OK {
			st = "OK"
		}
		rows.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%s</td><td>%s</td></tr>",
			s.Name, s.Port, st, s.Message))
	}
	if rows.Len() == 0 {
		rows.WriteString("<tr><td colspan=4>无冒烟明细</td></tr>")
	}
	title := rec.Title
	if title == "" {
		title = fmt.Sprintf("%s %s %s", rec.SiteCode, rec.Action, rec.PackageVer)
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="UTF-8"/><title>%s</title>
<style>
body{font-family:system-ui,sans-serif;max-width:860px;margin:2rem auto;padding:0 1rem;color:#0b1f28}
h1{font-size:1.6rem} .ok{color:%s;font-weight:700}
table{width:100%%;border-collapse:collapse;margin-top:1rem}
th,td{border-bottom:1px solid #ddd;padding:.6rem;text-align:left;font-size:.95rem}
.meta{color:#2a4450;line-height:1.7}
@media print{button{display:none}}
</style></head><body>
<button onclick="window.print()">打印 / 另存 PDF</button>
<h1>水厂交付验收报告</h1>
<p class="ok">验收结论：%s</p>
<div class="meta">
<div>交付单：%s</div>
<div>站点：%s (%s)</div>
<div>动作：%s · 版本：%s</div>
<div>操作者：%s · 时间：%s · 耗时：%ds</div>
<div>模块：%s · 服务数：%d</div>
<div>说明：%s</div>
</div>
<h2>服务健康矩阵</h2>
<table><thead><tr><th>服务</th><th>端口</th><th>状态</th><th>说明</th></tr></thead>
<tbody>%s</tbody></table>
<p class="meta" style="margin-top:2rem">由 wpgctl 自动生成 · %s</p>
</body></html>`,
		title, color, status, rec.ID,
		rec.SiteName, rec.SiteCode, rec.Action, rec.PackageVer,
		rec.Operator, rec.Time.Format(time.RFC3339), rec.DurationSec,
		strings.Join(rec.Profiles, ", "), rec.ServiceCount, rec.Message,
		rows.String(), time.Now().Format(time.RFC3339),
	)
}

func (s *Server) handleDiagDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	site, _ := config.LoadSite(s.opts.SitePath)
	composeDir := ""
	renderDir := ""
	if site != nil {
		composeDir = filepath.Join(site.Paths.Workspace, "rendered")
		renderDir = composeDir
	}
	outName := fmt.Sprintf("wpgctl-diag-%s.tar.gz", time.Now().Format("20060102150405"))
	outPath := filepath.Join(os.TempDir(), outName)
	path, err := status.CollectDiag(status.DiagOptions{
		Site: site, ComposeDir: composeDir, RenderDir: renderDir, Output: outPath,
	})
	if err != nil {
		s.writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		s.writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename="+outName)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	_, _ = w.Write(data)
}

func (s *Server) handleDeliveryExport(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=wpg-deliveries.json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"exportedAt": time.Now().Format(time.RFC3339),
		"count":      len(list),
		"records":    list,
	})
}
