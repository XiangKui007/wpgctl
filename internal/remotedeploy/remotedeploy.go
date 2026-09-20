// Package remotedeploy 把控制台的"单模块部署 / nginx patch"动作分发到 site.yaml 中的目标机器执行。
//
// 流程（每次调用都幂等）：
//  1. SSH 连接目标机（root 直连；非 root 自动 sudo）；
//  2. 上传 site.yaml 与本机同版本 wpgctl（已有同版本则跳过）；
//  3. 目标机若无模块目录，则把主控机上的模块目录同步过去（跳过 data/logs；非 Nginx 模块还跳过 html/frontend/dist，前端只随 Nginx 到中间件机）；
//  4. 远端执行 `wpgctl module deploy|nginx ...`，输出逐行回传到控制台日志。
package remotedeploy

import (
	"fmt"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/sshx"
	"github.com/wpg/wpgctl/internal/util"
	"github.com/wpg/wpgctl/internal/version"
)

// Target 目标机器与凭据。
type Target struct {
	Node        config.Node
	SSHPassword string
	SSHKeyPath  string
	SitePath    string // 主控机 site.yaml
}

// ModuleOptions 远端模块部署参数（与 moduledeploy.Options / `wpgctl module deploy` 一致）。
type ModuleOptions struct {
	ModuleDir    string // 主控机上的模块目录
	Expand       bool
	Load         bool
	PatchEnv     bool
	ComposeUp    bool
	ComposeBuild bool
	SyncFiles    bool // 目标机缺少模块目录时是否上传；false 则直接报错提示手工拷贝
	ForceSync    bool // 即使目标机已有同路径目录也重新同步
}

// NginxOptions 远端 nginx 步骤参数。
type NginxOptions struct {
	NginxDir       string
	GatewayIP      string
	AppIP          string
	GraphIP        string
	ExpandHTML     bool
	ComposeUp      bool
	SkipProxyPatch bool
	SyncFiles      bool
	ForceSync      bool
}

// Result 远端执行摘要。
type Result struct {
	Node      string `json:"node"`
	IP        string `json:"ip"`
	RemoteDir string `json:"remoteDir"`
	Synced    bool   `json:"synced"`
	Lines     int    `json:"lines"`
}

// FindNode 按名称或 IP 在 site.yaml 中查找节点。
func FindNode(site *config.SiteConfig, nameOrIP string) (config.Node, bool) {
	key := strings.TrimSpace(nameOrIP)
	if site == nil || key == "" {
		return config.Node{}, false
	}
	for _, n := range site.Nodes {
		if n.Name == key || n.IP == key {
			return n, true
		}
	}
	return config.Node{}, false
}

// IsRemote 节点是否需要走 SSH（IP 非本机）。
func IsRemote(n config.Node) bool {
	return !util.IsLocalIP(n.IP)
}

// localVersionKey 用于比对远端 `wpgctl version` 输出的关键字。
func localVersionKey() string {
	if version.Version == "dev" && version.GitCommit == "unknown" {
		return "" // 开发构建无法区分版本：总是上传
	}
	return fmt.Sprintf("wpgctl %s (commit=%s", version.Version, version.GitCommit)
}

// RunModule 在目标机执行模块部署。log 每行日志回调（不可为空时按需忽略）。
func RunModule(t Target, opts ModuleOptions, log func(string)) (*Result, error) {
	if log == nil {
		log = func(string) {}
	}
	res := &Result{Node: t.Node.Name, IP: t.Node.IP}
	log(fmt.Sprintf("目标机器: %s (%s)，通过 SSH %s@%s:%d 执行", t.Node.Name, t.Node.IP, t.Node.SSH.User, t.Node.IP, portOrDefault(t.Node.SSH.Port)))

	sess, err := sshx.Open(t.Node, t.SSHPassword, t.SSHKeyPath, log)
	if err != nil {
		return res, err
	}
	defer sess.Close()

	if err := sess.UploadSite(t.SitePath); err != nil {
		return res, err
	}
	if err := sess.EnsureBinary(localVersionKey()); err != nil {
		return res, err
	}

	remoteDir, synced, err := ensureRemoteDir(sess, opts.ModuleDir, opts.SyncFiles, opts.ForceSync, sshx.IncludeFrontendDir(opts.ModuleDir), log)
	if err != nil {
		return res, err
	}
	res.RemoteDir, res.Synced = remoteDir, synced

	args := []string{"module", "deploy", "--site", sshx.RemoteSitePath, "--dir", remoteDir}
	if opts.ComposeBuild {
		args = append(args, "--compose-build", "--patch-env")
	} else {
		if opts.Expand {
			args = append(args, "--expand")
		}
		if opts.Load {
			args = append(args, "--load")
		}
		if opts.PatchEnv {
			args = append(args, "--patch-env")
		}
		if opts.ComposeUp {
			args = append(args, "--compose-up")
		}
	}

	err = sess.RunWpgctl(args, func(line string) {
		res.Lines++
		log("[" + t.Node.Name + "] " + line)
	})
	if err != nil {
		return res, fmt.Errorf("目标机 %s 执行失败: %w", t.Node.Name, err)
	}
	return res, nil
}

// RunNginx 在目标机执行 nginx 步骤。
func RunNginx(t Target, opts NginxOptions, log func(string)) (*Result, error) {
	if log == nil {
		log = func(string) {}
	}
	res := &Result{Node: t.Node.Name, IP: t.Node.IP}
	log(fmt.Sprintf("目标机器: %s (%s)，通过 SSH 执行 nginx 步骤", t.Node.Name, t.Node.IP))

	sess, err := sshx.Open(t.Node, t.SSHPassword, t.SSHKeyPath, log)
	if err != nil {
		return res, err
	}
	defer sess.Close()

	if err := sess.UploadSite(t.SitePath); err != nil {
		return res, err
	}
	if err := sess.EnsureBinary(localVersionKey()); err != nil {
		return res, err
	}
	remoteDir, synced, err := ensureRemoteDir(sess, opts.NginxDir, opts.SyncFiles, opts.ForceSync, true, log)
	if err != nil {
		return res, err
	}
	res.RemoteDir, res.Synced = remoteDir, synced

	args := []string{"module", "nginx", "--site", sshx.RemoteSitePath, "--dir", remoteDir}
	if opts.GatewayIP != "" {
		args = append(args, "--gateway-ip", opts.GatewayIP)
	}
	if opts.AppIP != "" {
		args = append(args, "--app-ip", opts.AppIP)
	}
	if opts.GraphIP != "" {
		args = append(args, "--graph-ip", opts.GraphIP)
	}
	if opts.ExpandHTML {
		args = append(args, "--expand-html")
	}
	if opts.ComposeUp {
		args = append(args, "--compose-up")
	}
	if opts.SkipProxyPatch {
		args = append(args, "--skip-proxy-patch")
	}
	err = sess.RunWpgctl(args, func(line string) {
		res.Lines++
		log("[" + t.Node.Name + "] " + line)
	})
	if err != nil {
		return res, fmt.Errorf("目标机 %s 执行失败: %w", t.Node.Name, err)
	}
	return res, nil
}

// ensureRemoteDir 目标机上准备模块目录：同路径已存在则直接用；否则按 syncFiles 决定上传或报错。
// includeFrontend 为 true 时同步 html/frontend/dist（仅 Nginx）；其它模块跳过前端静态。
func ensureRemoteDir(sess *sshx.Session, localDir string, syncFiles, force, includeFrontend bool, log func(string)) (string, bool, error) {
	localDir = strings.TrimSpace(localDir)
	if localDir == "" {
		return "", false, fmt.Errorf("模块目录为空")
	}
	if !util.DirExists(localDir) {
		return "", false, fmt.Errorf("主控机上模块目录不存在: %s", localDir)
	}
	// Linux 主控机上的绝对路径在目标机上原样复用（同一套交付包解压路径）；Windows 联调落到暂存目录
	remote := sshx.RemoteDirFor(localDir)
	if !force && sess.Client.Exists(remote) {
		log(fmt.Sprintf("目标机已存在 %s，直接使用（如需重新上传请勾选强制同步）", remote))
		return remote, false, nil
	}
	if !syncFiles {
		return "", false, fmt.Errorf("目标机 %s 上不存在 %s；请勾选「自动同步文件到目标机」或手工拷贝交付包后重试", sess.Node.IP, remote)
	}
	if includeFrontend {
		log("同步含前端静态（html/frontend/dist）到 Nginx 所在中间件机")
	} else {
		log("跳过前端静态目录 html/frontend/dist（前端只部署到中间件机，由 Nginx 转发）")
	}
	if err := sess.SyncDirFilter(localDir, remote, includeFrontend); err != nil {
		return "", false, err
	}
	return remote, true, nil
}

func portOrDefault(p int) int {
	if p <= 0 {
		return 22
	}
	return p
}
