package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/db"
)

// 本文件提供现场自选 SQL 的 HTTP 入口（POST /api/db/apply），与包内 sql/ 台账执行分开。

// handleDBApply 现场自选 SQL：按顺序对 MySQL / PostgreSQL 执行，不强制文件名规范。
func (s *Server) handleDBApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Files    []string `json:"files"`
		Driver   string   `json:"driver"`
		Database string   `json:"database"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, 400, map[string]string{"error": "请求体无效"})
		return
	}
	if len(body.Files) == 0 {
		s.writeJSON(w, 400, map[string]string{"error": "请选择至少一个 .sql 文件"})
		return
	}

	job := s.newJob("db-apply")
	go func() {
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, "请先保存 site.yaml："+err.Error())
			return
		}
		s.appendLog(job, fmt.Sprintf("执行 SQL 共 %d 个 → %s", len(body.Files), strings.TrimSpace(body.Driver)))
		for _, f := range body.Files {
			s.appendLog(job, "  "+f)
		}
		res, err := db.ApplyFiles(db.FileOptions{
			Site:     site,
			Files:    body.Files,
			Driver:   body.Driver,
			Database: body.Database,
			Log:      func(line string) { s.appendLog(job, line) },
		})
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		job.Result = res
		s.okJob(job, fmt.Sprintf("SQL 执行完成：成功 %d 个", len(res.Applied)))
	}()
	s.writeJSON(w, 202, job)
}
