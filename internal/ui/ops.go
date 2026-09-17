package ui

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/fetch"
	"github.com/wpg/wpgctl/internal/state"
	"github.com/wpg/wpgctl/internal/upgrade"
	"github.com/wpg/wpgctl/internal/util"
	"github.com/wpg/wpgctl/internal/version"
)

// handleHealthEx 增强健康检查：含 Docker 状态与部署友好提示。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dockerOK := false
	dockerVer := ""
	dockerMsg := ""
	dr := dockerx.New()
	if dockerx.Which("docker") {
		if dr.Available() {
			dockerOK = true
			dockerVer, _ = dr.Version()
			if runtime.GOOS == "windows" {
				dockerMsg = "Docker Desktop 已就绪"
			} else {
				dockerMsg = "Docker daemon 已就绪"
			}
		} else if runtime.GOOS == "windows" {
			dockerMsg = "请启动 Docker Desktop（托盘图标变绿）"
		} else {
			dockerMsg = "docker 已安装但 daemon 不可用"
		}
	} else if runtime.GOOS == "windows" {
		dockerMsg = "未检测到 Docker Desktop，请先安装"
	} else {
		dockerMsg = "未安装 Docker，可由 init + base 包离线安装"
	}

	hint := "可执行完整部署向导"
	if runtime.GOOS == "windows" {
		if dockerOK {
			hint = "Windows + Docker Desktop：可本机部署；注意 host 网络模式不支持"
		} else {
			hint = "Windows：请先启动 Docker Desktop 再部署"
		}
	}

	s.writeJSON(w, 200, map[string]any{
		"ok":           true,
		"version":      version.Version,
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"dockerOk":     dockerOK,
		"dockerVer":    dockerVer,
		"dockerMsg":    dockerMsg,
		"deployHint":   hint,
		"sitePath":     s.opts.SitePath,
		"packagesDir":  util.PackagesDir(),
	})
}

func (s *Server) handleHostInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	s.writeJSON(w, 200, util.DetectHostInfo())
}

func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		PatchDir string `json:"patchDir"`
		Yes      bool   `json:"yes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.PatchDir == "" {
		s.writeJSON(w, 400, map[string]string{"error": "patchDir 不能为空"})
		return
	}

	job := s.newJob("upgrade")
	go func() {
		s.appendLog(job, "开始补丁升级: "+body.PatchDir)
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		res, err := upgrade.Run(upgrade.Options{
			Site:     site,
			PatchDir: body.PatchDir,
			SitePath: s.opts.SitePath,
			Yes:      true, // UI 已确认
		})
		job.Result = res
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		s.okJob(job, "升级完成")
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		To string `json:"to"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	job := s.newJob("rollback")
	go func() {
		s.appendLog(job, "开始回滚"+(map[bool]string{true: " → " + body.To, false: "（上一成功版本）"}[body.To != ""]))
		site, err := config.LoadSite(s.opts.SitePath)
		if err != nil {
			s.failJob(job, err.Error())
			return
		}
		if err := upgrade.Rollback(site, body.To); err != nil {
			s.failJob(job, err.Error())
			return
		}
		s.okJob(job, "回滚完成")
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Name  string `json:"name"`
		From  string `json:"from"`
		Local string `json:"local"`
		Dest  string `json:"dest"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" && body.Local == "" {
		s.writeJSON(w, 400, map[string]string{"error": "请提供包名 name 或本地目录 local"})
		return
	}

	job := s.newJob("fetch")
	go func() {
		if body.Local != "" {
			s.appendLog(job, "本地导入: "+body.Local)
		} else {
			s.appendLog(job, "拉取包: "+body.Name)
		}
		res, err := fetch.Run(fetch.Options{
			Name:     body.Name,
			FromURL:  body.From,
			LocalDir: body.Local,
			DestDir:  body.Dest,
		})
		if err != nil {
			msg := err.Error()
			if strings.HasPrefix(msg, "LOCAL_READY:") {
				src := strings.TrimPrefix(msg, "LOCAL_READY:")
				s.appendLog(job, "检测到完整包目录: "+src)
				job.Result = map[string]string{"packageDir": src}
				s.okJob(job, "本地包已就绪")
				return
			}
			s.failJob(job, err.Error())
			return
		}
		job.Result = res
		s.appendLog(job, "包目录: "+res.PackageDir)
		s.okJob(job, "拉包完成")
	}()
	s.writeJSON(w, 202, job)
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Package string `json:"package"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Package == "" {
		s.writeJSON(w, 400, map[string]string{"error": "package 不能为空"})
		return
	}
	site, err := config.LoadSite(s.opts.SitePath)
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	mf, err := config.LoadManifest(filepath.Join(body.Package, "manifest.yaml"))
	if err != nil {
		s.writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	svcs := mf.EnabledServices(site.Profiles)
	type row struct {
		Name    string `json:"name"`
		Image   string `json:"image"`
		Layer   int    `json:"layer"`
		Port    int    `json:"port"`
		Profile string `json:"profile"`
	}
	list := make([]row, 0, len(svcs))
	for _, s := range svcs {
		list = append(list, row{
			Name: s.Name, Image: s.Image, Layer: s.Layer, Port: s.Port, Profile: s.Profile,
		})
	}
	s.writeJSON(w, 200, map[string]any{
		"version":  mf.Version,
		"kind":     mf.Kind,
		"profiles": site.Profiles,
		"ports":    mf.Ports(site.Profiles),
		"services": list,
		"count":    len(list),
	})
}

func (s *Server) handleLatestDeploy(w http.ResponseWriter, r *http.Request) {
	site, err := config.LoadSite(s.opts.SitePath)
	code := ""
	if err == nil {
		code = site.Site.Code
	}
	st, err := state.NewStore()
	if err != nil {
		s.writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	latest, err := st.LatestSuccess(code)
	if err != nil {
		s.writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, map[string]any{"latest": latest})
}
