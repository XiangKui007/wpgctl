// Package dockerx 封装对 docker / docker compose CLI 的调用。
//
// 决策（方案 §3.3）：不使用 Docker SDK，通过 exec CLI + --format json，
// 保证行为与人工排障命令完全一致。
package dockerx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner 执行 docker 相关命令的抽象，便于单测注入。
type Runner struct {
	// Bin docker 可执行文件路径，默认 "docker"。
	Bin string
	// Timeout 单次命令超时。
	Timeout time.Duration
}

// New 创建默认 Runner。
func New() *Runner {
	return &Runner{Bin: "docker", Timeout: 10 * time.Minute}
}

// Available 检测本机是否可执行 docker。
func (r *Runner) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Bin, "version", "--format", "{{.Server.Version}}")
	return cmd.Run() == nil
}

// Version 返回 Docker Server 版本号。
func (r *Runner) Version() (string, error) {
	out, err := r.run("version", "--format", "{{.Server.Version}}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ImageExists 判断本地是否已有指定镜像 tag。
func (r *Runner) ImageExists(image string) bool {
	_, err := r.run("image", "inspect", image, "--format", "{{.Id}}")
	return err == nil
}

// LoadImage 执行 docker load -i <tar>。
func (r *Runner) LoadImage(tarPath string) error {
	_, err := r.run("load", "-i", tarPath)
	if err != nil {
		return fmt.Errorf("docker load 失败 (%s): %w", tarPath, err)
	}
	return nil
}

// ComposeUp 在指定目录执行 docker compose up -d，可附加 profiles 与服务名。
func (r *Runner) ComposeUp(dir string, file string, profiles []string, services ...string) error {
	args := []string{"compose"}
	if file != "" {
		args = append(args, "-f", file)
	}
	for _, p := range profiles {
		args = append(args, "--profile", p)
	}
	args = append(args, "up", "-d")
	args = append(args, services...)
	_, err := r.runIn(dir, args...)
	if err != nil {
		return fmt.Errorf("docker compose up 失败: %w", err)
	}
	return nil
}

// ComposePs 返回 compose 服务状态 JSON。
func (r *Runner) ComposePs(dir, file string) ([]ComposeService, error) {
	args := []string{"compose"}
	if file != "" {
		args = append(args, "-f", file)
	}
	args = append(args, "ps", "--format", "json")
	out, err := r.runIn(dir, args...)
	if err != nil {
		return nil, err
	}
	return parseComposePs(out)
}

// ComposeService compose ps 单条记录。
type ComposeService struct {
	Name   string `json:"Name"`
	Service string `json:"Service"`
	State  string `json:"State"`
	Status string `json:"Status"`
	Health string `json:"Health"`
	Image  string `json:"Image"`
}

func parseComposePs(out string) ([]ComposeService, error) {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	// compose 新版本逐行 JSON；旧版本可能是数组。
	if strings.HasPrefix(out, "[") {
		var list []ComposeService
		if err := json.Unmarshal([]byte(out), &list); err != nil {
			return nil, err
		}
		return list, nil
	}
	var list []ComposeService
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var s ComposeService
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			return nil, fmt.Errorf("解析 compose ps 失败: %w", err)
		}
		list = append(list, s)
	}
	return list, nil
}

// Logs 获取容器日志。
func (r *Runner) Logs(container string, tail int, follow bool) (string, error) {
	args := []string{"logs", fmt.Sprintf("--tail=%d", tail)}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, container)
	return r.run(args...)
}

// Build 在指定目录构建镜像并打 tag。
func (r *Runner) Build(dir, tag string) error {
	_, err := r.runIn(dir, "build", "-t", tag, ".")
	if err != nil {
		return fmt.Errorf("docker build 失败 (%s): %w", tag, err)
	}
	return nil
}

// InspectMemory 返回容器内存使用（字节），失败返回 0。
func (r *Runner) InspectMemory(container string) int64 {
	out, err := r.run("stats", "--no-stream", "--format", "{{.MemUsage}}", container)
	if err != nil {
		return 0
	}
	_ = out
	return 0
}

func (r *Runner) run(args ...string) (string, error) {
	return r.runIn("", args...)
}

func (r *Runner) runIn(dir string, args ...string) (string, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	bin := r.Bin
	if bin == "" {
		bin = "docker"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

// Which 查找可执行文件是否在 PATH 中。
func Which(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
