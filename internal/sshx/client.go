// Package sshx 提供多机场景下的 SSH 分发能力（方案 §6.3 / 阶段二）。
//
// 设计要点：
// 1. 主控机将 wpgctl 自身二进制 sftp 到目标机后远程执行 `wpgctl xxx --local`，
//    避免逐条 SSH 命令的脆弱性；
// 2. 密码支持交互输入或环境变量，密钥优先；
// 3. 依赖 golang.org/x/crypto/ssh + github.com/pkg/sftp。
package sshx

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"github.com/wpg/wpgctl/internal/config"
	"golang.org/x/crypto/ssh"
)

// Client SSH 会话客户端接口。
type Client interface {
	// Run 在远端执行命令，返回合并输出。
	Run(command string) (string, error)
	// RunStream 在远端执行命令，stdout/stderr 按行回调；stdin 非空时写入命令标准输入（用于 sudo -S）。
	RunStream(command, stdin string, onLine func(line string)) error
	// Upload 上传本地文件到远端路径。
	Upload(localPath, remotePath string) error
	// UploadDir 递归上传目录；skip 返回 true 的相对路径（/ 分隔）跳过；progress 每个文件回调一次。
	UploadDir(localDir, remoteDir string, skip func(rel string, isDir bool) bool, progress func(msg string)) error
	// Exists 远端路径是否存在（文件或目录）。
	Exists(remotePath string) bool
	// Close 关闭连接。
	Close() error
}

// Options 连接选项。
type Options struct {
	Host        string
	Port        int
	User        string
	Password    string
	PrivateKey  string
	DialTimeout time.Duration
}

// sshClient 真实 SSH + SFTP 实现。
type sshClient struct {
	client *ssh.Client
}

// Dial 建立 SSH 连接。
func Dial(opts Options) (Client, error) {
	if opts.Host == "" || opts.User == "" {
		return nil, fmt.Errorf("ssh: host/user 不能为空")
	}
	if opts.Port == 0 {
		opts.Port = 22
	}
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = 15 * time.Second
	}

	auth, err := buildAuth(opts)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 现场内网；二期可改为 known_hosts
		Timeout:         opts.DialTimeout,
	}
	addr := net.JoinHostPort(opts.Host, fmt.Sprintf("%d", opts.Port))
	cli, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh 连接失败 %s: %w", addr, err)
	}
	return &sshClient{client: cli}, nil
}

// DialNode 根据 site.yaml 节点信息建立连接。
func DialNode(node config.Node, password, privateKey string) (Client, error) {
	return Dial(Options{
		Host:        node.IP,
		Port:        node.SSH.Port,
		User:        node.SSH.User,
		Password:    password,
		PrivateKey:  privateKey,
		DialTimeout: 15 * time.Second,
	})
}

func buildAuth(opts Options) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if opts.PrivateKey != "" {
		keyData, err := os.ReadFile(opts.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("读取私钥失败: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if opts.Password != "" {
		methods = append(methods, ssh.Password(opts.Password))
	}
	if len(methods) == 0 {
		if env := os.Getenv("WPGCTL_SSH_PASSWORD"); env != "" {
			methods = append(methods, ssh.Password(env))
		} else if key := os.Getenv("WPGCTL_SSH_KEY"); key != "" {
			return buildAuth(Options{PrivateKey: key, Password: opts.Password})
		}
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("未提供 SSH 密码或私钥（可用环境变量 WPGCTL_SSH_PASSWORD / WPGCTL_SSH_KEY）")
	}
	return methods, nil
}

func (c *sshClient) Run(command string) (string, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(command)
	return string(out), err
}

func (c *sshClient) RunStream(command, stdin string, onLine func(string)) error {
	sess, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	if stdin != "" {
		sess.Stdin = strings.NewReader(stdin)
	}
	pr, pw := io.Pipe()
	sess.Stdout = pw
	sess.Stderr = pw
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			if onLine != nil {
				onLine(sc.Text())
			}
		}
	}()
	runErr := sess.Run(command)
	_ = pw.Close()
	<-done
	return runErr
}

func (c *sshClient) Upload(localPath, remotePath string) error {
	sc, err := sftp.NewClient(c.client)
	if err != nil {
		return fmt.Errorf("sftp 会话失败: %w", err)
	}
	defer sc.Close()
	return uploadFile(sc, localPath, remotePath, 0o755)
}

func uploadFile(sc *sftp.Client, localPath, remotePath string, mode os.FileMode) error {
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	_ = sc.MkdirAll(path.Dir(remotePath))
	dst, err := sc.Create(remotePath)
	if err != nil {
		return fmt.Errorf("创建远端文件失败 %s: %w", remotePath, err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return sc.Chmod(remotePath, mode)
}

func (c *sshClient) UploadDir(localDir, remoteDir string, skip func(rel string, isDir bool) bool, progress func(string)) error {
	sc, err := sftp.NewClient(c.client)
	if err != nil {
		return fmt.Errorf("sftp 会话失败: %w", err)
	}
	defer sc.Close()

	localDir = filepath.Clean(localDir)
	if err := sc.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("创建远端目录失败 %s: %w", remoteDir, err)
	}
	var count int
	var total int64
	err = filepath.Walk(localDir, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		rel, _ := filepath.Rel(localDir, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if skip != nil && skip(rel, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		remote := path.Join(remoteDir, rel)
		if info.IsDir() {
			return sc.MkdirAll(remote)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		// 已存在且大小一致：跳过（幂等，重跑不重传大 tar）
		if st, err := sc.Stat(remote); err == nil && st.Size() == info.Size() && !st.IsDir() {
			if progress != nil {
				progress(fmt.Sprintf("跳过（已存在）%s", rel))
			}
			return nil
		}
		if progress != nil {
			progress(fmt.Sprintf("上传 %s (%s)", rel, humanSize(info.Size())))
		}
		if err := uploadFile(sc, p, remote, info.Mode().Perm()|0o600); err != nil {
			return err
		}
		count++
		total += info.Size()
		return nil
	})
	if err != nil {
		return err
	}
	if progress != nil {
		progress(fmt.Sprintf("目录上传完成：%d 个文件，共 %s → %s", count, humanSize(total), remoteDir))
	}
	return nil
}

func (c *sshClient) Exists(remotePath string) bool {
	sc, err := sftp.NewClient(c.client)
	if err != nil {
		return false
	}
	defer sc.Close()
	_, err = sc.Stat(remotePath)
	return err == nil
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (c *sshClient) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// NodeResult 单节点执行结果。
type NodeResult struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

func shellQuote(s string) string {
	if s == "" {
		return `""`
	}
	need := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '"' || c == '\'' || c == '\\' || c == '$' {
			need = true
			break
		}
	}
	if !need {
		return s
	}
	return `"` + s + `"`
}
