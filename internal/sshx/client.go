// Package sshx 提供多机场景下的 SSH 分发能力（方案 §6.3 / 阶段二）。
//
// 设计要点：
// 1. 主控机将 wpgctl 自身二进制 sftp 到目标机后远程执行 `wpgctl xxx --local`，
//    避免逐条 SSH 命令的脆弱性；
// 2. 密码支持交互输入或环境变量，密钥优先；
// 3. 依赖 golang.org/x/crypto/ssh + github.com/pkg/sftp。
package sshx

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"github.com/wpg/wpgctl/internal/config"
	"golang.org/x/crypto/ssh"
)

// Client SSH 会话客户端接口。
type Client interface {
	// Run 在远端执行命令，返回合并输出。
	Run(command string) (string, error)
	// Upload 上传本地文件到远端路径。
	Upload(localPath, remotePath string) error
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

func (c *sshClient) Upload(localPath, remotePath string) error {
	sc, err := sftp.NewClient(c.client)
	if err != nil {
		return fmt.Errorf("sftp 会话失败: %w", err)
	}
	defer sc.Close()

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	_ = sc.MkdirAll(filepath.ToSlash(filepath.Dir(remotePath)))
	dst, err := sc.Create(remotePath)
	if err != nil {
		return fmt.Errorf("创建远端文件失败: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return sc.Chmod(remotePath, 0o755)
}

func (c *sshClient) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}

// DistributeSelf 将本机 wpgctl 二进制分发到各节点并远程执行命令。
//
// remoteArgs 例如: []string{"init", "--site", "/tmp/site.yaml", "--local"}
func DistributeSelf(nodes []config.Node, password, privateKey, localBin, remoteBin string, remoteArgs []string) ([]NodeResult, error) {
	if remoteBin == "" {
		remoteBin = "/usr/local/bin/wpgctl"
	}
	results := make([]NodeResult, 0, len(nodes))
	for _, n := range nodes {
		r := NodeResult{Name: n.Name, IP: n.IP}
		cli, err := DialNode(n, password, privateKey)
		if err != nil {
			r.OK = false
			r.Message = err.Error()
			results = append(results, r)
			continue
		}
		if err := cli.Upload(localBin, remoteBin); err != nil {
			_ = cli.Close()
			r.OK = false
			r.Message = "上传失败: " + err.Error()
			results = append(results, r)
			continue
		}
		cmd := remoteBin
		for _, a := range remoteArgs {
			cmd += " " + shellQuote(a)
		}
		out, err := cli.Run(cmd)
		_ = cli.Close()
		r.Output = out
		if err != nil {
			r.OK = false
			r.Message = err.Error()
		} else {
			r.OK = true
			r.Message = "ok"
		}
		results = append(results, r)
	}
	return results, nil
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
