// Package verify 部署验收：汇总容器状态并探测关键服务端口连通性。
package verify

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/wpg/wpgctl/internal/config"
	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/sshx"
	"github.com/wpg/wpgctl/internal/util"
)

// Options 验收选项。
type Options struct {
	Site        *config.SiteConfig
	SSHPassword string
	SSHKeyPath  string
	DialTimeout time.Duration
}

// ServiceRow 容器/服务状态行。
type ServiceRow struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	State   string `json:"state"`
	Status  string `json:"status"`
	Health  string `json:"health,omitempty"`
	Image   string `json:"image,omitempty"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// PortRow 端口连通性行。
type PortRow struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// Report 验收报告。
type Report struct {
	Services   []ServiceRow `json:"services"`
	Ports      []PortRow    `json:"ports"`
	OK         bool         `json:"ok"`
	Summary    string       `json:"summary"`
	Warnings   []string     `json:"warnings,omitempty"`
	CheckedAt  string       `json:"checkedAt"`
	ServiceOK  int          `json:"serviceOk"`
	ServiceAll int          `json:"serviceAll"`
	PortOK     int          `json:"portOk"`
	PortAll    int          `json:"portAll"`
}

// Run 检查本机（及多机从机）容器状态，并探测 site 中关键中间件/网关端口。
func Run(opts Options) (*Report, error) {
	if opts.Site == nil {
		return nil, fmt.Errorf("site 不能为空")
	}
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = 3 * time.Second
	}
	rep := &Report{
		CheckedAt: time.Now().Format(time.RFC3339),
	}

	localList, err := dockerx.New().ListContainers()
	if err != nil {
		rep.Warnings = append(rep.Warnings, "本机 docker ps 失败: "+err.Error())
	} else {
		for _, c := range localList {
			rep.Services = append(rep.Services, rowFromContainer(c, "本机"))
		}
	}

	for _, n := range opts.Site.Nodes {
		if util.IsLocalIP(n.IP) {
			continue
		}
		remote, werr := listRemoteContainers(n, opts.SSHPassword, opts.SSHKeyPath)
		if werr != "" {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s (%s): %s", n.Name, n.IP, werr))
			continue
		}
		label := n.Name
		if label == "" {
			label = n.IP
		}
		for _, c := range remote {
			rep.Services = append(rep.Services, rowFromContainer(c, label))
		}
	}

	rep.Ports = checkPorts(opts.Site, opts.DialTimeout)

	for _, s := range rep.Services {
		rep.ServiceAll++
		if s.OK {
			rep.ServiceOK++
		}
	}
	for _, p := range rep.Ports {
		rep.PortAll++
		if p.OK {
			rep.PortOK++
		}
	}
	rep.OK = rep.ServiceAll > 0 && rep.ServiceOK == rep.ServiceAll &&
		(rep.PortAll == 0 || rep.PortOK == rep.PortAll)
	if rep.ServiceAll == 0 {
		rep.OK = false
		rep.Summary = "未发现运行中的容器，请确认前面步骤已部署"
	} else if rep.OK {
		rep.Summary = fmt.Sprintf("验收通过：容器 %d/%d，端口 %d/%d",
			rep.ServiceOK, rep.ServiceAll, rep.PortOK, rep.PortAll)
	} else {
		rep.Summary = fmt.Sprintf("验收未完全通过：容器 %d/%d 正常，端口 %d/%d 可连",
			rep.ServiceOK, rep.ServiceAll, rep.PortOK, rep.PortAll)
	}
	return rep, nil
}

func rowFromContainer(c dockerx.ComposeService, node string) ServiceRow {
	name := c.Service
	if name == "" {
		name = c.Name
	}
	state := strings.ToLower(strings.TrimSpace(c.State))
	ok := state == "running" || strings.Contains(strings.ToLower(c.Status), "up")
	msg := c.Status
	if msg == "" {
		msg = c.State
	}
	return ServiceRow{
		Name:    name,
		Node:    node,
		State:   c.State,
		Status:  c.Status,
		Health:  c.Health,
		Image:   c.Image,
		OK:      ok,
		Message: msg,
	}
}

func listRemoteContainers(n config.Node, password, keyPath string) ([]dockerx.ComposeService, string) {
	sess, err := sshx.Open(n, password, keyPath, nil)
	if err != nil {
		return nil, "SSH 失败: " + err.Error()
	}
	defer sess.Close()
	out, err := sess.Client.Run("docker ps -a --format '{{json .}}' 2>/dev/null")
	if err != nil {
		return nil, "docker ps 失败: " + err.Error()
	}
	list, perr := dockerx.ParseDockerPsJSON(out)
	if perr != nil {
		return nil, "解析 docker ps 失败: " + perr.Error()
	}
	return list, ""
}

type portTarget struct {
	Name string
	Host string
	Port int
}

func checkPorts(site *config.SiteConfig, timeout time.Duration) []PortRow {
	var targets []portTarget
	add := func(name, host string, port int) {
		host = strings.TrimSpace(host)
		if host == "" || port <= 0 {
			return
		}
		targets = append(targets, portTarget{Name: name, Host: host, Port: port})
	}

	m := site.Middleware
	if !m.MySQL.Disabled {
		p := m.MySQL.Port
		if p <= 0 {
			p = 3306
		}
		add("MySQL", m.MySQL.Host, p)
	}
	{
		p := m.PgSQL.Port
		if p <= 0 {
			p = 5433
		}
		if strings.TrimSpace(m.PgSQL.Host) != "" {
			add("PostgreSQL", m.PgSQL.Host, p)
		}
	}
	{
		p := m.Redis.Port
		if p <= 0 {
			p = 6377
		}
		if strings.TrimSpace(m.Redis.Host) != "" {
			add("Redis", m.Redis.Host, p)
		}
	}
	{
		p := m.Nacos.Port
		if p <= 0 {
			p = 8848
		}
		if strings.TrimSpace(m.Nacos.Host) != "" {
			add("Nacos", m.Nacos.Host, p)
		}
	}
	{
		p := m.Kafka.Port
		if p <= 0 {
			p = 9092
		}
		if strings.TrimSpace(m.Kafka.Host) != "" {
			add("Kafka", m.Kafka.Host, p)
		}
	}

	// Nginx 入口：优先首节点（网关常见落在主控）
	nginxHost := ""
	if len(site.Nodes) > 0 {
		nginxHost = strings.TrimSpace(site.Nodes[0].IP)
	}
	for _, n := range site.Nodes {
		for _, s := range n.Services {
			if strings.EqualFold(strings.TrimSpace(s), "nginx") {
				nginxHost = strings.TrimSpace(n.IP)
				break
			}
		}
	}
	if nginxHost != "" {
		add("Nginx", nginxHost, 8877)
	}

	var out []PortRow
	for _, t := range targets {
		out = append(out, dialPort(t, timeout))
	}
	return out
}

func dialPort(t portTarget, timeout time.Duration) PortRow {
	addr := fmt.Sprintf("%s:%d", t.Host, t.Port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return PortRow{
			Name: t.Name, Host: t.Host, Port: t.Port,
			OK: false, Message: err.Error(),
		}
	}
	_ = conn.Close()
	return PortRow{
		Name: t.Name, Host: t.Host, Port: t.Port,
		OK: true, Message: "可连通",
	}
}
