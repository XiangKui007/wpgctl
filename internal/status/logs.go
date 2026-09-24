package status

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/wpg/wpgctl/internal/config"
	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/sshx"
	"github.com/wpg/wpgctl/internal/util"
)

// LogsOptions 页面日志流参数：空 NodeIP 或本机地址走本地 Docker，否则 SSH 到从机。
type LogsOptions struct {
	Site        *config.SiteConfig
	Service     string // 容器名，须通过安全名校验
	NodeIP      string
	SSHPassword string
	SSHKeyPath  string
	Tail        int // 跟随开始前先吐出的末尾行数；≤0 时用 200
}

// LogPuller 复用本机或一条 SSH 连接，执行 docker logs -f。
type LogPuller struct {
	local     *dockerx.Runner
	remote    *sshx.Session
	service   string
	tail      int
	nodeIP    string
	closeOnce sync.Once
}

// NewLogPuller 按节点打开日志跟随器；从机失败返回可展示给现场的中文错误。
func NewLogPuller(opts LogsOptions) (*LogPuller, error) {
	name := strings.TrimSpace(opts.Service)
	if !reSafeName.MatchString(name) {
		return nil, fmt.Errorf("非法容器名")
	}
	tail := opts.Tail
	if tail <= 0 {
		tail = 200
	}
	nodeIP := strings.TrimSpace(opts.NodeIP)
	p := &LogPuller{service: name, tail: tail, nodeIP: nodeIP}
	if nodeIP == "" || util.IsLocalIP(nodeIP) {
		p.local = dockerx.New()
		return p, nil
	}
	node, ok := findNodeByIP(opts.Site, nodeIP)
	if !ok {
		return nil, fmt.Errorf("site.yaml 中没有节点 %s，无法 SSH 拉日志", nodeIP)
	}
	sess, err := sshx.Open(node, opts.SSHPassword, opts.SSHKeyPath, nil)
	if err != nil {
		return nil, fmt.Errorf("SSH %s 失败: %w", nodeIP, err)
	}
	p.remote = sess
	return p, nil
}

// Follow 跟随 docker logs -f，每行回调一次（已去掉 ANSI）；ctx 取消时结束。
func (p *LogPuller) Follow(ctx context.Context, onLine func(string)) error {
	if p == nil {
		return fmt.Errorf("日志拉取器未初始化")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	wrap := func(line string) {
		if onLine != nil {
			onLine(util.StripANSI(line))
		}
	}
	if p.local != nil {
		return p.local.LogsFollow(ctx, p.service, p.tail, wrap)
	}
	if p.remote == nil {
		return fmt.Errorf("日志拉取器未初始化")
	}
	go func() {
		<-ctx.Done()
		p.Close()
	}()
	cmd := dockerLogsCmd(p.service, p.tail, true)
	err := p.remote.Client.RunStream(cmd, "", wrap)
	if ctx.Err() != nil {
		return nil
	}
	if err == nil {
		return nil
	}
	full, stdin := p.remote.Privileged(cmd)
	err = p.remote.Client.RunStream(full, stdin, wrap)
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s 上 docker logs 失败: %v", p.nodeIP, err)
	}
	return nil
}

// Close 关闭从机 SSH；本机跟随由 ctx 取消即可。
func (p *LogPuller) Close() {
	if p == nil {
		return
	}
	p.closeOnce.Do(func() {
		if p.remote != nil {
			p.remote.Close()
			p.remote = nil
		}
	})
}

func dockerLogsCmd(name string, tail int, follow bool) string {
	if follow {
		return fmt.Sprintf("docker logs --tail=%d -f %s", tail, name)
	}
	return fmt.Sprintf("docker logs --tail=%d %s", tail, name)
}
