package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/wpg/wpgctl/internal/config"
	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/sshx"
	"github.com/wpg/wpgctl/internal/util"
)

// NodeDockerStatus 一台机器上 Docker 是否已启动，给机器卡片挂在名称后面。
type NodeDockerStatus struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Local   bool   `json:"local"`
	OK      bool   `json:"ok"`
	Version string `json:"version,omitempty"`
	Message string `json:"message"`
}

const remoteDockerVersion = "docker version --format '{{.Server.Version}}' 2>/dev/null || true"

// handleDockerNodes 探测表单或 site.yaml 里各节点的 Docker 是否起来。
func (s *Server) handleDockerNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Nodes []struct {
			Name    string `json:"name"`
			IP      string `json:"ip"`
			SSHUser string `json:"sshUser"`
			SSHPort int    `json:"sshPort"`
		} `json:"nodes"`
		SSHPassword string `json:"sshPassword"`
		SSHKeyPath  string `json:"sshKeyPath"`
	}
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.writeJSON(w, 400, map[string]string{"error": "请求体无效"})
			return
		}
	}
	nodes := make([]config.Node, 0, len(body.Nodes))
	for _, n := range body.Nodes {
		port := n.SSHPort
		if port == 0 {
			port = 22
		}
		user := strings.TrimSpace(n.SSHUser)
		if user == "" {
			user = "root"
		}
		nodes = append(nodes, config.Node{
			Name: strings.TrimSpace(n.Name),
			IP:   strings.TrimSpace(n.IP),
			SSH:  config.SSHAuth{User: user, Port: port},
		})
	}
	if len(nodes) == 0 {
		if site, err := config.LoadSite(s.opts.SitePath); err == nil {
			nodes = append(nodes, site.Nodes...)
		}
	}
	s.writeJSON(w, 200, map[string]any{"nodes": probeDockerNodes(nodes, body.SSHPassword, body.SSHKeyPath)})
}

func probeDockerNodes(nodes []config.Node, password, keyPath string) []NodeDockerStatus {
	out := make([]NodeDockerStatus, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		i, n := i, n
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = probeOneDocker(n, password, keyPath)
		}()
	}
	wg.Wait()
	return out
}

func probeOneDocker(n config.Node, password, keyPath string) NodeDockerStatus {
	name := strings.TrimSpace(n.Name)
	ip := strings.TrimSpace(n.IP)
	if name == "" {
		name = ip
	}
	st := NodeDockerStatus{Name: name, IP: ip}
	if ip == "" {
		st.Message = "未填 IP"
		return st
	}
	if util.IsLocalIP(ip) {
		st.Local = true
		return probeLocalDocker(st)
	}
	return probeRemoteDocker(st, n, password, keyPath)
}

func probeLocalDocker(st NodeDockerStatus) NodeDockerStatus {
	dr := dockerx.New()
	if !dockerx.Which("docker") {
		st.Message = "本机未安装 Docker"
		return st
	}
	if !dr.Available() {
		st.Message = "本机 Docker 未启动"
		return st
	}
	ver, _ := dr.Version()
	st.OK = true
	st.Version = ver
	if ver != "" {
		st.Message = "本机 Docker 已启动 " + ver
	} else {
		st.Message = "本机 Docker 已启动"
	}
	return st
}

func probeRemoteDocker(st NodeDockerStatus, n config.Node, password, keyPath string) NodeDockerStatus {
	sess, err := sshx.Open(n, password, keyPath, nil)
	if err != nil {
		st.Message = "SSH 失败，无法查看该机 Docker"
		return st
	}
	defer sess.Close()

	ver := runRemoteDockerVersion(sess)
	if ver == "" {
		st.Message = "该机 Docker 未启动"
		return st
	}
	st.OK = true
	st.Version = ver
	st.Message = "该机 Docker 已启动 " + ver
	return st
}

func runRemoteDockerVersion(sess *sshx.Session) string {
	out, err := sess.Client.Run(remoteDockerVersion)
	ver := firstDockerVersion(out)
	if err == nil && ver != "" {
		return ver
	}
	var b strings.Builder
	_ = sess.RunPrivileged(remoteDockerVersion, func(line string) {
		b.WriteString(line)
		b.WriteByte('\n')
	})
	return firstDockerVersion(b.String())
}

func firstDockerVersion(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		v := strings.TrimSpace(line)
		if v == "" {
			continue
		}
		lower := strings.ToLower(v)
		if strings.Contains(lower, "command not found") || strings.Contains(lower, "cannot connect") ||
			strings.Contains(lower, "permission denied") || strings.Contains(lower, "error") {
			continue
		}
		return v
	}
	return ""
}
