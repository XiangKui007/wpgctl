package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wpg/wpgctl/internal/util"
)

func (s *Server) handleFSText(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		s.writeJSON(w, 400, map[string]string{"error": "path 不能为空"})
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if !util.FileExists(abs) {
		s.writeJSON(w, 404, map[string]string{"error": "文件不存在: " + abs})
		return
	}
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(abs)
		if err != nil {
			s.writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, 200, map[string]any{
			"path": abs,
			"text": string(data),
			"size": len(data),
		})
	case http.MethodPut:
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		tmp := abs + ".wpgctl.tmp"
		if err := os.WriteFile(tmp, []byte(body.Text), 0o644); err != nil {
			s.writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if err := os.Rename(tmp, abs); err != nil {
			if err2 := os.WriteFile(abs, []byte(body.Text), 0o644); err2 != nil {
				s.writeJSON(w, 500, map[string]string{"error": fmt.Sprintf("写入失败: %v / %v", err, err2)})
				return
			}
			_ = os.Remove(tmp)
		}
		s.writeJSON(w, 200, map[string]any{"ok": true, "path": abs, "size": len(body.Text)})
	default:
		http.Error(w, "method not allowed", 405)
	}
}
