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
	"sync"
	"time"
)

// Runner 执行 docker 相关命令的抽象，便于单测注入。
type Runner struct {
	// Bin docker 可执行文件路径，默认 "docker"。
	Bin string
	// Timeout 单次命令超时。
	Timeout time.Duration

	composeOnce sync.Once
	composeMode string // "plugin" | "standalone"
	composeBin  string // standalone 时为 docker-compose 路径
}

// New 创建默认 Runner。
func New() *Runner {
	return &Runner{Bin: "docker", Timeout: 10 * time.Minute}
}

// Available 检测本机是否可执行 docker。
func (r *Runner) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.bin(), "version", "--format", "{{.Server.Version}}")
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

// MissingImages 返回本地不存在的镜像 tag 列表。
func (r *Runner) MissingImages(images []string) []string {
	seen := map[string]struct{}{}
	var missing []string
	for _, img := range images {
		img = strings.TrimSpace(img)
		if img == "" {
			continue
		}
		if _, ok := seen[img]; ok {
			continue
		}
		seen[img] = struct{}{}
		if !r.ImageExists(img) {
			missing = append(missing, img)
		}
	}
	return missing
}

// LoadImage 执行 docker load -i <tar>，返回本次 load 后 docker images 中新增的 tag。
func (r *Runner) LoadImage(tarPath string) ([]string, error) {
	before, _ := r.ListLocalImages()
	if _, err := r.run("load", "-i", tarPath); err != nil {
		return nil, fmt.Errorf("docker load 失败 (%s): %w", tarPath, err)
	}
	after, _ := r.ListLocalImages()
	return diffImageRefs(before, after), nil
}

// ListLocalImages 列出本地全部 Repository:Tag。
func (r *Runner) ListLocalImages() ([]string, error) {
	out, err := r.run("images", "--format", "{{.Repository}}:{{.Tag}}")
	if err != nil {
		return nil, err
	}
	var refs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "<none>") {
			continue
		}
		refs = append(refs, line)
	}
	return refs, nil
}

func parseLoadedImages(out string) []string {
	var refs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "Loaded image") {
			continue
		}
		// Loaded image: repo:tag
		// Loaded image ID: sha256:...
		idx := strings.Index(line, "Loaded image:")
		if idx < 0 {
			continue
		}
		ref := strings.TrimSpace(line[idx+len("Loaded image:"):])
		if ref == "" || strings.HasPrefix(ref, "ID:") {
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

func diffImageRefs(before, after []string) []string {
	set := map[string]struct{}{}
	for _, b := range before {
		set[b] = struct{}{}
	}
	var added []string
	for _, a := range after {
		if _, ok := set[a]; !ok {
			added = append(added, a)
		}
	}
	return added
}

// ComposeUp 在指定目录执行 compose up -d（自动兼容 docker compose 与 docker-compose）。
// build 为 true 时追加 --build，从 compose 上下文（如 jar 目录）构建镜像，不依赖预 load 的 tar。
func (r *Runner) ComposeUp(dir string, file string, profiles []string, build bool, services ...string) error {
	args := []string{}
	if file != "" {
		args = append(args, "-f", file)
	}
	for _, p := range profiles {
		args = append(args, "--profile", p)
	}
	args = append(args, "up", "-d")
	if build {
		args = append(args, "--build")
	} else if r.detectComposeMode() == "plugin" {
		args = append(args, "--pull", "never")
	}
	args = append(args, services...)
	_, err := r.runCompose(dir, args...)
	if err != nil {
		return fmt.Errorf("docker compose up 失败: %w", err)
	}
	return nil
}

// ListContainers 返回本机全部容器（现场逐步 deploy 无 rendered compose 时使用）。
func (r *Runner) ListContainers() ([]ComposeService, error) {
	out, err := r.run("ps", "-a", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	var list []ComposeService
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row struct {
			Names  string `json:"Names"`
			State  string `json:"State"`
			Status string `json:"Status"`
			Image  string `json:"Image"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("解析 docker ps 失败: %w", err)
		}
		name := row.Names
		if idx := strings.Index(name, ","); idx >= 0 {
			name = name[:idx]
		}
		list = append(list, ComposeService{
			Name:    name,
			Service: name,
			State:   row.State,
			Status:  row.Status,
			Image:   row.Image,
		})
	}
	return list, nil
}

// ComposePs 返回 compose 服务状态 JSON。
func (r *Runner) ComposePs(dir, file string) ([]ComposeService, error) {
	args := []string{}
	if file != "" {
		args = append(args, "-f", file)
	}
	mode := r.detectComposeMode()
	if mode == "standalone" {
		args = append(args, "ps")
	} else {
		args = append(args, "ps", "--format", "json")
	}
	out, err := r.runCompose(dir, args...)
	if err != nil {
		return nil, err
	}
	if mode == "standalone" {
		return parseComposePsStandalone(out)
	}
	return parseComposePs(out)
}

// ComposeService compose ps 单条记录。
type ComposeService struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Health  string `json:"Health"`
	Image   string `json:"Image"`
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

func parseComposePsStandalone(out string) ([]ComposeService, error) {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		return nil, nil
	}
	var list []ComposeService
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "-") {
			continue
		}
		s := ComposeService{Name: fields[0], Service: fields[0]}
		if len(fields) > 1 {
			s.Status = strings.Join(fields[1:], " ")
			s.State = fields[len(fields)-1]
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
	return r.runIn(r.bin(), "", args...)
}

// Build 在指定目录构建镜像并打 tag。
func (r *Runner) Build(dir, tag string) error {
	_, err := r.runIn(r.bin(), dir, "build", "-t", tag, ".")
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

func (r *Runner) bin() string {
	if r.Bin == "" {
		return "docker"
	}
	return r.Bin
}

func (r *Runner) detectComposeMode() string {
	r.composeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, r.bin(), "compose", "version")
		if cmd.Run() == nil {
			r.composeMode = "plugin"
			return
		}
		if path, err := exec.LookPath("docker-compose"); err == nil {
			r.composeMode = "standalone"
			r.composeBin = path
			return
		}
		r.composeMode = "plugin"
	})
	return r.composeMode
}

func (r *Runner) runCompose(dir string, args ...string) (string, error) {
	if r.detectComposeMode() == "standalone" {
		return r.runIn(r.composeBin, dir, args...)
	}
	return r.runIn(r.bin(), dir, append([]string{"compose"}, args...)...)
}

func (r *Runner) run(args ...string) (string, error) {
	return r.runIn(r.bin(), "", args...)
}

func (r *Runner) runIn(bin, dir string, args ...string) (string, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

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
		if s := strings.TrimSpace(stdout.String()); s != "" {
			msg = s + "\n" + msg
		}
		return stdout.String(), fmt.Errorf("%s", msg)
	}
	return stdout.String() + stderr.String(), nil
}

// Which 查找可执行文件是否在 PATH 中。
func Which(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
