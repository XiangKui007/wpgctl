package ui

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/initenv"
)

// handleWorkspace 开始前：探测 / 创建工作簿目录（不含 Docker 安装）。
func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ws := strings.TrimSpace(r.URL.Query().Get("workspace"))
		html := strings.TrimSpace(r.URL.Query().Get("nginxHtml"))
		if ws == "" {
			if site, err := config.LoadSite(s.opts.SitePath); err == nil && site != nil {
				ws = site.Paths.Workspace
				if html == "" {
					html = site.Paths.NginxHTML
				}
			}
		}
		if ws == "" {
			s.writeJSON(w, 400, map[string]string{"error": "请填写 workspace 路径"})
			return
		}
		probe, err := initenv.ProbeWorkspace(ws, html)
		if err != nil {
			s.writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, 200, probe)
	case http.MethodPost:
		var body struct {
			Workspace string `json:"workspace"`
			NginxHTML string `json:"nginxHtml"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		ws := strings.TrimSpace(body.Workspace)
		html := strings.TrimSpace(body.NginxHTML)
		if ws == "" {
			s.writeJSON(w, 400, map[string]string{"error": "workspace 路径不能为空"})
			return
		}
		probe, created, err := initenv.InitWorkspaceDirs(ws, html)
		if err != nil {
			s.writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, 200, map[string]any{
			"ok":      true,
			"created": created,
			"probe":   probe,
		})
	default:
		http.Error(w, "method not allowed", 405)
	}
}
