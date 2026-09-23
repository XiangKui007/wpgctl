package moduledeploy

import (
	"fmt"
	"path/filepath"
	"strings"

	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/util"
)

// NginxRuntime 本模块 compose 在本机 Docker 上的运行情况。
// 只认 nginx 目录对应的项目，不用整机 docker ps，避免把别的 nginx 当成已部署。
type NginxRuntime struct {
	ModuleDir string   `json:"moduleDir"`
	Compose   string   `json:"compose,omitempty"`
	Deployed  bool     `json:"deployed"` // 有服务且全部 running，且没有 unhealthy/starting
	Running   int      `json:"running"`
	Total     int      `json:"total"`
	Names     []string `json:"names,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Summary   string   `json:"summary"`
}

// InspectNginxRuntime 查 nginx 模块是否已经 compose up 且在跑。
// docker / compose 不可用时 Deployed 为 false，向导仍走正常部署。
func InspectNginxRuntime(moduleDir string) NginxRuntime {
	dir := strings.TrimSpace(moduleDir)
	if found := FindNginxModuleDir(dir); found != "" {
		dir = found
	}
	out := NginxRuntime{ModuleDir: dir}
	if dir == "" || !util.DirExists(dir) {
		return finishNginxRuntime(out, "模块目录不存在")
	}
	compose, err := findComposeFile(dir)
	if err != nil || compose == "" {
		return finishNginxRuntime(out, "未找到 docker-compose")
	}
	out.Compose = compose
	list, err := dockerx.New().ComposePs(filepath.Dir(compose), filepath.Base(compose))
	if err != nil {
		return finishNginxRuntime(out, "无法查询 compose 状态："+err.Error())
	}
	return JudgeNginxRuntime(out, list)
}

// JudgeNginxRuntime 根据 compose ps 结果判定是否已部署（单测用，不调 Docker）。
func JudgeNginxRuntime(base NginxRuntime, list []dockerx.ComposeService) NginxRuntime {
	base.Total = len(list)
	if len(list) == 0 {
		return finishNginxRuntime(base, "还没有容器（尚未 compose up）")
	}
	var notReady []string
	for _, s := range list {
		name := s.Service
		if name == "" {
			name = s.Name
		}
		base.Names = append(base.Names, name)
		if nginxServiceReady(s) {
			base.Running++
			continue
		}
		why := strings.TrimSpace(s.State)
		if h := strings.TrimSpace(s.Health); h != "" {
			why = why + "/" + h
		}
		if why == "" {
			why = "未运行"
		}
		notReady = append(notReady, name+"("+why+")")
	}
	if len(notReady) > 0 {
		return finishNginxRuntime(base, "未就绪："+strings.Join(notReady, "、"))
	}
	base.Deployed = true
	return finishNginxRuntime(base, "")
}

func nginxServiceReady(s dockerx.ComposeService) bool {
	st := strings.ToLower(strings.TrimSpace(s.State))
	status := strings.ToLower(s.Status)
	running := st == "running" || st == "up" || strings.HasPrefix(status, "up")
	if !running {
		return false
	}
	h := strings.ToLower(strings.TrimSpace(s.Health))
	if h == "unhealthy" || h == "starting" {
		return false
	}
	return true
}

func finishNginxRuntime(st NginxRuntime, reason string) NginxRuntime {
	st.Reason = reason
	if st.Deployed {
		st.Summary = fmt.Sprintf("Nginx 已在运行（%d/%d）", st.Running, st.Total)
	} else if st.Total > 0 {
		st.Summary = fmt.Sprintf("Nginx 未完全就绪（%d/%d）", st.Running, st.Total)
	} else if reason != "" {
		st.Summary = "Nginx 尚未部署"
	} else {
		st.Summary = "Nginx 状态未知"
	}
	return st
}
