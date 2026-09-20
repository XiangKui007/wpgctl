package sshx

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
)

// 远端约定路径。
const (
	RemoteSitePath  = "/tmp/wpgctl-site.yaml"
	RemoteStageRoot = "/opt/wpgctl-stage" // 主控机路径不可复用（如 Windows 联调）时的落盘根目录
	remoteBinRoot   = "/usr/local/bin/wpgctl"
	remoteBinUser   = "/tmp/wpgctl-bin/wpgctl"
)

// Session 一台目标机上的分发会话：封装连接、sudo 提权与 wpgctl 自身分发。
type Session struct {
	Node     config.Node
	Client   Client
	password string
	// RemoteBin 目标机上的 wpgctl 路径。
	RemoteBin string
	Log       func(string)
}

// Open 连接节点并返回会话；Log 为空时静默。
func Open(node config.Node, password, privateKey string, log func(string)) (*Session, error) {
	cli, err := DialNode(node, password, privateKey)
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = func(string) {}
	}
	s := &Session{Node: node, Client: cli, password: password, Log: log}
	if s.IsRoot() {
		s.RemoteBin = remoteBinRoot
	} else {
		s.RemoteBin = remoteBinUser
	}
	return s, nil
}

// Close 关闭连接。
func (s *Session) Close() {
	if s != nil && s.Client != nil {
		_ = s.Client.Close()
	}
}

// IsRoot SSH 用户是否 root。
func (s *Session) IsRoot() bool {
	return strings.TrimSpace(s.Node.SSH.User) == "" || s.Node.SSH.User == "root"
}

// Privileged 为非 root 用户加 sudo 前缀（有密码走 sudo -S 从 stdin 读取，无密码走 sudo -n）。
// 返回命令与需要写入 stdin 的内容。
func (s *Session) Privileged(cmd string) (string, string) {
	if s.IsRoot() {
		return cmd, ""
	}
	if s.password != "" {
		return "sudo -S -p '' " + cmd, s.password + "\n"
	}
	return "sudo -n " + cmd, ""
}

// RunPrivileged 以 root 权限流式执行命令。
func (s *Session) RunPrivileged(cmd string, onLine func(string)) error {
	full, stdin := s.Privileged(cmd)
	return s.Client.RunStream(full, stdin, onLine)
}

// UploadSite 上传 site.yaml 到远端约定路径。
func (s *Session) UploadSite(localSitePath string) error {
	if strings.TrimSpace(localSitePath) == "" {
		return fmt.Errorf("site.yaml 路径为空")
	}
	if err := s.Client.Upload(localSitePath, RemoteSitePath); err != nil {
		return fmt.Errorf("上传 site.yaml: %w", err)
	}
	s.Log("已上传 site.yaml → " + RemoteSitePath)
	return nil
}

// EnsureBinary 确保目标机存在与本机同版本的 wpgctl；版本一致则跳过上传。
//
// localVersion 为本机 version 输出的关键字（如 "wpgctl dev (commit=abc"）；为空则总是上传。
func (s *Session) EnsureBinary(localVersion string) error {
	localBin, err := LocalLinuxBinary()
	if err != nil {
		return err
	}
	if localVersion != "" {
		if out, err := s.Client.Run(s.RemoteBin + " version"); err == nil && strings.Contains(out, localVersion) {
			s.Log(fmt.Sprintf("目标机已有同版本 wpgctl（%s），跳过上传", s.RemoteBin))
			return nil
		}
	}
	s.Log(fmt.Sprintf("上传 wpgctl → %s:%s", s.Node.IP, s.RemoteBin))
	if s.IsRoot() {
		if err := s.Client.Upload(localBin, s.RemoteBin); err != nil {
			return fmt.Errorf("上传 wpgctl: %w", err)
		}
		return nil
	}
	// 非 root：先传到可写目录，再 sudo 挪到 /usr/local/bin（失败则留在用户目录直接执行）
	if err := s.Client.Upload(localBin, remoteBinUser); err != nil {
		return fmt.Errorf("上传 wpgctl: %w", err)
	}
	mv := fmt.Sprintf("sh -c 'cp %s %s && chmod 755 %s'", remoteBinUser, remoteBinRoot, remoteBinRoot)
	if err := s.RunPrivileged(mv, nil); err == nil {
		s.RemoteBin = remoteBinRoot
	} else {
		s.Log("sudo 安装到 /usr/local/bin 失败，将直接使用 " + remoteBinUser)
		s.RemoteBin = remoteBinUser
	}
	return nil
}

// RunWpgctl 以 root 权限在远端执行 `wpgctl <args>`，输出按行回调。
func (s *Session) RunWpgctl(args []string, onLine func(string)) error {
	cmd := s.RemoteBin
	for _, a := range args {
		cmd += " " + shellQuote(a)
	}
	s.Log("远端执行: " + cmd)
	return s.RunPrivileged(cmd, onLine)
}

// IncludeFrontendDir 判断模块目录是否应同步前端静态。仅目录名为 nginx 时为 true（中间件机 Nginx 步骤）。
func IncludeFrontendDir(localDir string) bool {
	p := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(localDir), "\\", "/"))
	p = strings.Trim(p, "/")
	return path.Base(p) == "nginx"
}

// SkipSyncRel 同步目录时跳过的相对路径：运行时 data/logs，以及非 Nginx 机上的前端静态。
func SkipSyncRel(rel string, isDir, includeFrontend bool) bool {
	if !isDir {
		return false
	}
	base := strings.ToLower(path.Base(rel))
	if base == "data" || base == "logs" || base == "log" || base == ".git" {
		return true
	}
	if includeFrontend {
		return false
	}
	return base == "html" || base == "frontend" || base == "dist"
}

// SyncDir 将本机目录同步到远端 remoteDir（跳过 data/logs；已存在且大小一致的文件不重传）。
// 前端静态（html/frontend/dist）默认不同步——只应随 Nginx 部署到中间件机。
func (s *Session) SyncDir(localDir, remoteDir string) error {
	return s.SyncDirFilter(localDir, remoteDir, false)
}

// SyncDirFilter includeFrontend 为 true 时把 html/frontend/dist 一并上传（Nginx 模块）。
func (s *Session) SyncDirFilter(localDir, remoteDir string, includeFrontend bool) error {
	remoteDir = strings.TrimSpace(remoteDir)
	if remoteDir == "" {
		return fmt.Errorf("远端目录为空")
	}
	skip := func(rel string, isDir bool) bool {
		return SkipSyncRel(rel, isDir, includeFrontend)
	}
	if s.IsRoot() {
		s.Log(fmt.Sprintf("同步目录 %s → %s:%s", localDir, s.Node.IP, remoteDir))
		if err := s.Client.UploadDir(localDir, remoteDir, skip, s.Log); err != nil {
			return fmt.Errorf("同步目录失败: %w", err)
		}
		return nil
	}
	stage := path.Join("/tmp/wpgctl-stage", path.Base(remoteDir))
	s.Log(fmt.Sprintf("同步目录 %s → %s:%s（非 root，先暂存再 sudo 移动到 %s）", localDir, s.Node.IP, stage, remoteDir))
	if err := s.Client.UploadDir(localDir, stage, skip, s.Log); err != nil {
		return fmt.Errorf("同步目录失败: %w", err)
	}
	mv := fmt.Sprintf("sh -c 'mkdir -p %s && cp -a %s/. %s/ && rm -rf %s'",
		shellQuote(remoteDir), shellQuote(stage), shellQuote(remoteDir), shellQuote(stage))
	if err := s.RunPrivileged(mv, s.Log); err != nil {
		return fmt.Errorf("sudo 移动到 %s 失败: %w", remoteDir, err)
	}
	return nil
}

// RemoteDirFor 计算本机目录在目标机上的对应路径：Linux 绝对路径原样复用；否则放到暂存根目录下。
func RemoteDirFor(localDir string) string {
	p := strings.ReplaceAll(strings.TrimSpace(localDir), "\\", "/")
	if strings.HasPrefix(p, "/") {
		return path.Clean(p)
	}
	return path.Join(RemoteStageRoot, path.Base(p))
}

// LocalLinuxBinary 返回可上传到 Linux 目标机的 wpgctl 二进制路径。
//
// 主控机为 Linux 时即为自身；其他平台（开发机联调）需通过环境变量 WPGCTL_LINUX_BIN 指定。
func LocalLinuxBinary() (string, error) {
	if p := strings.TrimSpace(os.Getenv("WPGCTL_LINUX_BIN")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("WPGCTL_LINUX_BIN 指定的文件不存在: %s", p)
		}
		return p, nil
	}
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("多机 SSH 分发需在 Linux 主控机上运行 wpgctl（当前 %s）；开发机联调可设置 WPGCTL_LINUX_BIN 指向 Linux 版二进制", runtime.GOOS)
	}
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("定位本机二进制失败: %w", err)
	}
	return self, nil
}
