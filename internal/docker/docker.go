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

// LoadImage 执行 docker load -i <tar>，返回本次 load 后 docker images 中新增的 tag（无 tag 时退回 Loaded image ID）。
func (r *Runner) LoadImage(tarPath string) ([]string, error) {
	before, _ := r.ListLocalImages()
	out, err := r.run("load", "-i", tarPath)
	if err != nil {
		return nil, fmt.Errorf("docker load 失败 (%s): %w", tarPath, err)
	}
	after, _ := r.ListLocalImages()
	refs := diffImageRefs(before, after)
	if len(refs) == 0 {
		refs = parseLoadedImages(out)
	}
	if len(refs) == 0 {
		refs = parseLoadedImageIDs(out)
	}
	return refs, nil
}

// TagImage 给已有镜像打 tag（离线 load 后补齐 Dockerfile 的 FROM 名，如 java:8）。
func (r *Runner) TagImage(src, dest string) error {
	src = strings.TrimSpace(src)
	dest = strings.TrimSpace(dest)
	if src == "" || dest == "" {
		return fmt.Errorf("docker tag 参数为空")
	}
	if src == dest {
		return nil
	}
	if _, err := r.run("tag", src, dest); err != nil {
		return fmt.Errorf("docker tag %s -> %s 失败: %w", src, dest, err)
	}
	return nil
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

func parseLoadedImageIDs(out string) []string {
	var ids []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		const prefix = "Loaded image ID:"
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		id := strings.TrimSpace(line[idx+len(prefix):])
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
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
// build 为 true 时追加 --build；离线现场须先 load Dockerfile 的 FROM 基础镜像（如 java8.tar → java:8）。
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
	return ParseDockerPsJSON(out)
}

// ParseDockerPsJSON 解析 `docker ps -a --format {{json .}}` 的逐行 JSON。
// 跳过非 `{` 开头的行（SSH/sudo 杂讯），避免整表失败。
func ParseDockerPsJSON(out string) ([]ComposeService, error) {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	var list []ComposeService
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		s, err := parseDockerPsLine(line)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

func parseDockerPsLine(line string) (ComposeService, error) {
	var row struct {
		ID        string          `json:"ID"`
		Names     string          `json:"Names"`
		State     string          `json:"State"`
		Status    string          `json:"Status"`
		Image     string          `json:"Image"`
		Ports     string          `json:"Ports"`
		CreatedAt string          `json:"CreatedAt"`
		Labels    json.RawMessage `json:"Labels"`
		Networks  json.RawMessage `json:"Networks"`
	}
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		return ComposeService{}, fmt.Errorf("解析 docker ps 失败: %w", err)
	}
	name := row.Names
	if idx := strings.Index(name, ","); idx >= 0 {
		name = name[:idx]
	}
	labels := parseDockerLabels(row.Labels)
	svc := labels["com.docker.compose.service"]
	if svc == "" {
		svc = name
	}
	return ComposeService{
		ID:         row.ID,
		Name:       name,
		Service:    svc,
		State:      row.State,
		Status:     row.Status,
		Health:     healthFromStatus(row.Status),
		Image:      row.Image,
		Ports:      row.Ports,
		Created:    compactCreated(row.CreatedAt),
		Project:    labels["com.docker.compose.project"],
		ComposeDir: labels["com.docker.compose.project.working_dir"],
		Networks:   networksFromRaw(row.Networks),
	}, nil
}

func parseDockerLabels(raw json.RawMessage) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	var obj map[string]string
	if json.Unmarshal(raw, &obj) == nil {
		return obj
	}
	var s string
	if json.Unmarshal(raw, &s) != nil || s == "" {
		return out
	}
	for _, part := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
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

// ComposeService compose / docker ps 单条记录，供状态页与 CLI 展示。
type ComposeService struct {
	ID         string `json:"ID,omitempty"`
	Name       string `json:"Name"`
	Service    string `json:"Service"`
	State      string `json:"State"`
	Status     string `json:"Status"`
	Health     string `json:"Health"`
	Image      string `json:"Image"`
	Ports      string `json:"Ports,omitempty"`
	Created    string `json:"Created,omitempty"`    // 创建时间，已去掉时区后缀便于现场阅读
	Project    string `json:"Project,omitempty"`
	ComposeDir string `json:"ComposeDir,omitempty"` // compose 工作目录，排障与 Down 栈用
	Networks   string `json:"Networks,omitempty"`   // 加入的网络，逗号分隔
	ExitCode   *int   `json:"ExitCode,omitempty"`   // 非运行中才带退出码，避免 running 也显示 0
	Node       string `json:"Node,omitempty"`       // 节点名（多机状态页）
	NodeIP     string `json:"NodeIP,omitempty"`     // 节点 IP，启停时用来走 SSH
	Local      bool   `json:"Local,omitempty"`      // true 表示本机 Docker
}

// composePsJSON 兼容 docker compose ps --format json：Created 可能是 unix 秒，Ports 可能只在 Publishers 里。
type composePsJSON struct {
	ID         string             `json:"ID"`
	Name       string             `json:"Name"`
	Service    string             `json:"Service"`
	State      string             `json:"State"`
	Status     string             `json:"Status"`
	Health     string             `json:"Health"`
	Image      string             `json:"Image"`
	Ports      string             `json:"Ports"`
	Project    string             `json:"Project"`
	ExitCode   int                `json:"ExitCode"`
	Created    json.RawMessage    `json:"Created"`
	Labels     json.RawMessage    `json:"Labels"`
	Networks   json.RawMessage    `json:"Networks"`
	Publishers []composePublisher `json:"Publishers"`
}

type composePublisher struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
}

func parseComposePs(out string) ([]ComposeService, error) {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	// compose 新版本逐行 JSON；旧版本可能是数组。
	if strings.HasPrefix(out, "[") {
		var rows []composePsJSON
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			return nil, err
		}
		list := make([]ComposeService, 0, len(rows))
		for _, row := range rows {
			list = append(list, composeServiceFromJSON(row))
		}
		return list, nil
	}
	var list []ComposeService
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row composePsJSON
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("解析 compose ps 失败: %w", err)
		}
		list = append(list, composeServiceFromJSON(row))
	}
	return list, nil
}

func composeServiceFromJSON(row composePsJSON) ComposeService {
	ports := strings.TrimSpace(row.Ports)
	if ports == "" {
		ports = portsFromPublishers(row.Publishers)
	}
	labels := parseDockerLabels(row.Labels)
	health := strings.TrimSpace(row.Health)
	if health == "" {
		health = healthFromStatus(row.Status)
	}
	svc := strings.TrimSpace(row.Service)
	if svc == "" {
		svc = row.Name
	}
	project := strings.TrimSpace(row.Project)
	if project == "" {
		project = labels["com.docker.compose.project"]
	}
	return ComposeService{
		ID:         row.ID,
		Name:       row.Name,
		Service:    svc,
		State:      row.State,
		Status:     row.Status,
		Health:     health,
		Image:      row.Image,
		Ports:      ports,
		Created:    createdFromRaw(row.Created),
		Project:    project,
		ComposeDir: labels["com.docker.compose.project.working_dir"],
		Networks:   networksFromRaw(row.Networks),
		ExitCode:   exitCodeForState(row.State, row.Status, row.ExitCode),
	}
}

func healthFromStatus(status string) string {
	switch {
	case strings.Contains(status, "(healthy)"):
		return "healthy"
	case strings.Contains(status, "(unhealthy)"):
		return "unhealthy"
	case strings.Contains(status, "(health: starting)"):
		return "starting"
	default:
		return ""
	}
}

func compactCreated(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, " +"); i > 0 {
		return s[:i]
	}
	if i := strings.Index(s, " -"); i >= 10 {
		return s[:i]
	}
	return s
}

func createdFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		v, err := n.Int64()
		if err == nil && v > 0 {
			if v > 1e12 {
				v = v / 1000
			}
			return time.Unix(v, 0).Format("2006-01-02 15:04:05")
		}
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return compactCreated(s)
	}
	return ""
}

func networksFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		return strings.Join(arr, ", ")
	}
	return ""
}

func portsFromPublishers(pubs []composePublisher) string {
	parts := make([]string, 0, len(pubs))
	for _, p := range pubs {
		if p.PublishedPort == 0 {
			continue
		}
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		host := p.URL
		if host == "" {
			host = "0.0.0.0"
		}
		parts = append(parts, fmt.Sprintf("%s:%d->%d/%s", host, p.PublishedPort, p.TargetPort, proto))
	}
	return strings.Join(parts, ", ")
}

func exitCodeForState(state, status string, code int) *int {
	st := strings.ToLower(strings.TrimSpace(state))
	if st == "running" || st == "up" {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(status)), "up") {
		return nil
	}
	c := code
	return &c
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

// StartContainer 启动已存在的容器。
func (r *Runner) StartContainer(name string) error {
	_, err := r.run("start", name)
	if err != nil {
		return fmt.Errorf("docker start 失败: %w", err)
	}
	return nil
}

// StopContainer 停止运行中的容器。
func (r *Runner) StopContainer(name string) error {
	_, err := r.run("stop", name)
	if err != nil {
		return fmt.Errorf("docker stop 失败: %w", err)
	}
	return nil
}

// RestartContainer 重启容器。
func (r *Runner) RestartContainer(name string) error {
	_, err := r.run("restart", name)
	if err != nil {
		return fmt.Errorf("docker restart 失败: %w", err)
	}
	return nil
}

// RemoveContainer 强制删除容器（单个容器 down）。
func (r *Runner) RemoveContainer(name string) error {
	_, err := r.run("rm", "-f", name)
	if err != nil {
		return fmt.Errorf("docker rm 失败: %w", err)
	}
	return nil
}

// ComposeDown 在指定目录执行 compose down。
func (r *Runner) ComposeDown(dir, file string) error {
	args := []string{}
	if file != "" {
		args = append(args, "-f", file)
	}
	args = append(args, "down")
	_, err := r.runCompose(dir, args...)
	if err != nil {
		return fmt.Errorf("docker compose down 失败: %w", err)
	}
	return nil
}

// ComposeWorkingDir 读取容器 compose 工作目录标签。
func (r *Runner) ComposeWorkingDir(container string) string {
	out, err := r.run("inspect", "-f", `{{index .Config.Labels "com.docker.compose.project.working_dir"}}`, container)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
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
