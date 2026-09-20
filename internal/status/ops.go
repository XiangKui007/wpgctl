package status

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	dockerx "github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/sshx"
	"github.com/wpg/wpgctl/internal/util"
)

var reSafeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// ActionOptions 状态页启停：本机直接 docker，从机 SSH 执行同样命令。
type ActionOptions struct {
	Action      string
	Name        string
	ComposeDir  string
	NodeIP      string
	Site        *config.SiteConfig
	SSHPassword string
	SSHKeyPath  string
}

// RunAction 对本机单个容器或 compose 栈执行运维动作。
func RunAction(action, name, composeDir string) error {
	return RunClusterAction(ActionOptions{Action: action, Name: name, ComposeDir: composeDir})
}

// RunClusterAction 按 NodeIP 决定本机或 SSH 从机。
func RunClusterAction(opts ActionOptions) error {
	action := strings.ToLower(strings.TrimSpace(opts.Action))
	name := strings.TrimSpace(opts.Name)
	composeDir := strings.TrimSpace(opts.ComposeDir)
	nodeIP := strings.TrimSpace(opts.NodeIP)

	if nodeIP != "" && !util.IsLocalIP(nodeIP) {
		return runRemoteAction(opts, action, name, composeDir, nodeIP)
	}

	d := dockerx.New()
	switch action {
	case "start", "stop", "restart", "down":
		if !reSafeName.MatchString(name) {
			return fmt.Errorf("非法容器名")
		}
	case "stack-down":
		if composeDir == "" && name != "" && reSafeName.MatchString(name) {
			composeDir = d.ComposeWorkingDir(name)
		}
		if composeDir == "" {
			return fmt.Errorf("无法确定 compose 目录，请对单个容器使用 Down")
		}
		if !util.DirExists(composeDir) {
			return fmt.Errorf("compose 目录不存在: %s", composeDir)
		}
		return d.ComposeDown(composeDir, "")
	default:
		return fmt.Errorf("未知操作: %s", action)
	}

	switch action {
	case "start":
		return d.StartContainer(name)
	case "stop":
		return d.StopContainer(name)
	case "restart":
		return d.RestartContainer(name)
	case "down":
		return d.RemoveContainer(name)
	default:
		return fmt.Errorf("未知操作: %s", action)
	}
}

func runRemoteAction(opts ActionOptions, action, name, composeDir, nodeIP string) error {
	node, ok := findNodeByIP(opts.Site, nodeIP)
	if !ok {
		return fmt.Errorf("site.yaml 中没有节点 %s，无法 SSH", nodeIP)
	}
	sess, err := sshx.Open(node, opts.SSHPassword, opts.SSHKeyPath, nil)
	if err != nil {
		return fmt.Errorf("SSH %s 失败: %w", nodeIP, err)
	}
	defer sess.Close()

	var cmd string
	switch action {
	case "start", "stop", "restart", "down":
		if !reSafeName.MatchString(name) {
			return fmt.Errorf("非法容器名")
		}
		switch action {
		case "start":
			cmd = "docker start " + name
		case "stop":
			cmd = "docker stop " + name
		case "restart":
			cmd = "docker restart " + name
		case "down":
			cmd = "docker rm -f " + name
		default:
			return fmt.Errorf("未知操作: %s", action)
		}
	case "stack-down":
		quoted, qerr := shellSingleQuote(composeDir)
		if qerr != nil {
			return qerr
		}
		cmd = "docker compose --project-directory " + quoted + " down"
	default:
		return fmt.Errorf("未知操作: %s", action)
	}

	if err := sess.Client.RunStream(cmd, "", nil); err == nil {
		return nil
	}
	if err := sess.RunPrivileged(cmd, nil); err != nil {
		return fmt.Errorf("%s 上执行失败: %w", nodeIP, err)
	}
	return nil
}

func shellSingleQuote(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("compose 目录必须是 Linux 绝对路径")
	}
	if strings.ContainsAny(p, "'\n;$`|&") {
		return "", fmt.Errorf("compose 目录含非法字符")
	}
	return "'" + p + "'", nil
}
