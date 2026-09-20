// Package precheck 实现环境体检（方案 §6.2）。
//
// 输出红/黄/绿报告：红色阻断部署，黄色警告放行。
package precheck

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/docker"
	"github.com/wpg/wpgctl/internal/health"
	"github.com/wpg/wpgctl/internal/util"
)

// Severity 检查项严重级别。
type Severity string

const (
	// SeverityGreen 通过。
	SeverityGreen Severity = "green"
	// SeverityYellow 警告。
	SeverityYellow Severity = "yellow"
	// SeverityRed 阻断。
	SeverityRed Severity = "red"
)

// Item 单项检查结果。
type Item struct {
	Name     string   `json:"name"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"` // 可执行建议
}

// Report 完整体检报告。
type Report struct {
	Items      []Item `json:"items"`
	HasRed     bool   `json:"hasRed"`
	HasYellow  bool   `json:"hasYellow"`
	Arch       string `json:"arch"`
	OS         string `json:"os"`
}

// Options 体检选项。
type Options struct {
	Site     *config.SiteConfig
	Manifest *config.Manifest // 可选，用于端口/资源校验
}

// Run 执行全部体检项并返回报告。
func Run(opts Options) (*Report, error) {
	r := &Report{
		Arch: runtime.GOARCH,
		OS:   runtime.GOOS,
	}

	r.add(checkCPU())
	r.add(checkMemory(opts.Manifest))
	r.add(checkDisk(opts.Site, opts.Manifest))
	r.add(checkKernel())
	r.add(checkArch(opts.Manifest))
	r.add(checkDocker())
	if tip := checkWindowsComposeTips(); tip.Name != "" {
		r.add(tip)
	}
	if opts.Manifest != nil && opts.Site != nil {
		r.add(checkPorts(opts.Manifest.Ports(opts.Site.Profiles))...)
	}
	if opts.Site != nil {
		r.add(checkMiddlewareConnectivity(opts.Site)...)
	}

	for _, it := range r.Items {
		switch it.Severity {
		case SeverityRed:
			r.HasRed = true
		case SeverityYellow:
			r.HasYellow = true
		}
	}
	return r, nil
}

func (r *Report) add(items ...Item) {
	r.Items = append(r.Items, items...)
}

// Print 以人类可读格式输出报告。
func (r *Report) Print() {
	for _, it := range r.Items {
		mark := "●"
		switch it.Severity {
		case SeverityGreen:
			mark = "🟢"
		case SeverityYellow:
			mark = "🟡"
		case SeverityRed:
			mark = "🔴"
		}
		fmt.Printf("%s [%s] %s — %s\n", mark, strings.ToUpper(string(it.Severity)), it.Name, it.Message)
	}
	fmt.Println()
	if r.HasRed {
		util.Errorf("体检存在红色项，禁止继续部署")
	} else if r.HasYellow {
		util.Warnf("体检存在黄色项，可继续但请关注")
	} else {
		util.Successf("体检全部通过")
	}
}

// ToJSON 序列化报告。
func (r *Report) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ---------------------------------------------------------------------------
// 检查项实现
// ---------------------------------------------------------------------------

func checkCPU() Item {
	n := runtime.NumCPU()
	if n < 2 {
		return Item{Name: "CPU", Severity: SeverityRed, Message: fmt.Sprintf("仅 %d 核，最低建议 2 核", n)}
	}
	if n < 4 {
		return Item{Name: "CPU", Severity: SeverityYellow, Message: fmt.Sprintf("%d 核，建议 ≥4 核", n)}
	}
	return Item{Name: "CPU", Severity: SeverityGreen, Message: fmt.Sprintf("%d 核", n)}
}

func checkMemory(m *config.Manifest) Item {
	totalMB, err := memTotalMB()
	if err != nil {
		return Item{Name: "内存", Severity: SeverityYellow, Message: "无法获取内存信息: " + err.Error()}
	}
	minGB := 8
	if m != nil && m.MinMemGB > 0 {
		minGB = m.MinMemGB
	}
	gb := float64(totalMB) / 1024
	if gb < float64(minGB) {
		sev := SeverityRed
		hint := "增加内存或降低并发部署的服务组合（减少 profiles）"
		// 本机联调场景：略低于声明最低也允许黄灯放行，避免 Demo 机器被硬拦
		if gb >= float64(minGB)*0.7 {
			sev = SeverityYellow
			hint = "内存低于 manifest 建议值，可继续但建议关闭其他占内存程序"
		}
		return Item{Name: "内存", Severity: sev, Message: fmt.Sprintf("%.1f GB < 最低 %d GB", gb, minGB), Hint: hint}
	}
	return Item{Name: "内存", Severity: SeverityGreen, Message: fmt.Sprintf("%.1f GB", gb)}
}

func checkDisk(site *config.SiteConfig, m *config.Manifest) Item {
	path := "."
	if site != nil && site.Paths.Workspace != "" {
		path = site.Paths.Workspace
		_ = util.EnsureDir(path)
	}
	freeGB, err := diskFreeGB(path)
	if err != nil {
		return Item{Name: "磁盘", Severity: SeverityYellow, Message: "无法获取磁盘空间: " + err.Error()}
	}
	minGB := 50
	if m != nil && m.MinDiskGB > 0 {
		minGB = m.MinDiskGB
	}
	if freeGB < float64(minGB) {
		return Item{
			Name: "磁盘", Severity: SeverityRed,
			Message: fmt.Sprintf("%.1f GB 可用 < 最低 %d GB", freeGB, minGB),
			Hint:    "清理磁盘，或把 paths.workspace 改到更大的盘（如 D:/workspace）",
		}
	}
	return Item{Name: "磁盘", Severity: SeverityGreen, Message: fmt.Sprintf("%.1f GB 可用 (%s)", freeGB, path)}
}

func checkKernel() Item {
	if runtime.GOOS == "windows" {
		r := dockerx.New()
		if dockerx.Which("docker") && r.Available() {
			return Item{
				Name:     "平台",
				Severity: SeverityGreen,
				Message:  "Windows + Docker Desktop（Linux 容器引擎已就绪，可用于本机部署）",
			}
		}
		return Item{
			Name:     "平台",
			Severity: SeverityYellow,
			Message:  "Windows：请安装并启动 Docker Desktop 后再部署（WSL2 后端推荐）",
		}
	}
	if runtime.GOOS != "linux" {
		return Item{Name: "平台", Severity: SeverityYellow, Message: fmt.Sprintf("当前 OS=%s，请确认已安装可用的 Docker", runtime.GOOS)}
	}
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return Item{Name: "内核", Severity: SeverityYellow, Message: "无法获取内核版本"}
	}
	ver := strings.TrimSpace(string(out))
	majorMinor := strings.Split(ver, ".")
	if len(majorMinor) >= 2 {
		maj, _ := strconv.Atoi(majorMinor[0])
		min, _ := strconv.Atoi(majorMinor[1])
		if maj < 3 || (maj == 3 && min < 10) {
			return Item{Name: "内核", Severity: SeverityRed, Message: ver + " < 3.10，无法运行现代 Docker"}
		}
	}
	return Item{Name: "内核", Severity: SeverityGreen, Message: ver}
}

func checkArch(m *config.Manifest) Item {
	arch := runtime.GOARCH
	if m == nil || len(m.Arch) == 0 {
		return Item{Name: "架构", Severity: SeverityGreen, Message: arch}
	}
	for _, a := range m.Arch {
		if a == arch || (a == "amd64" && arch == "amd64") || (a == "arm64" && arch == "arm64") {
			return Item{Name: "架构", Severity: SeverityGreen, Message: arch}
		}
	}
	return Item{Name: "架构", Severity: SeverityRed, Message: fmt.Sprintf("本机 %s 不在包支持列表 %v", arch, m.Arch)}
}

func checkDocker() Item {
	r := dockerx.New()
	if !dockerx.Which("docker") {
		if runtime.GOOS == "windows" {
			return Item{
				Name: "Docker", Severity: SeverityRed,
				Message: "未检测到 docker，请安装 Docker Desktop 并勾选「启动时打开」",
				Hint:    "安装后从开始菜单启动 Docker Desktop，等待托盘图标变绿，再重新体检",
			}
		}
		return Item{Name: "Docker", Severity: SeverityYellow, Message: "未安装，可由 wpgctl init 离线安装", Hint: "向导中选择 Docker 离线目录（含 offline_install_docker.sh），或 --docker-package / --base"}
	}
	if !r.Available() {
		if runtime.GOOS == "windows" {
			return Item{
				Name: "Docker", Severity: SeverityRed,
				Message: "Docker Desktop 未运行：请从托盘启动，待引擎变绿后再试",
				Hint:    "右键托盘鲸鱼图标 → Start；或重启 Docker Desktop",
			}
		}
		return Item{Name: "Docker", Severity: SeverityYellow, Message: "docker 命令存在但 daemon 不可用", Hint: "检查 systemctl status docker"}
	}
	ver, err := r.Version()
	if err != nil {
		return Item{Name: "Docker", Severity: SeverityYellow, Message: err.Error()}
	}
	if !versionGTE(ver, "20.10") {
		return Item{Name: "Docker", Severity: SeverityRed, Message: ver + " < 20.10"}
	}
	msg := "Server " + ver
	if runtime.GOOS == "windows" {
		msg = "Docker Desktop · Server " + ver
	}
	return Item{Name: "Docker", Severity: SeverityGreen, Message: msg}
}

// checkWindowsComposeTips 提示 Windows 上 host 网络等差异（黄灯，不阻断）。
func checkWindowsComposeTips() Item {
	if runtime.GOOS != "windows" {
		return Item{}
	}
	return Item{
		Name:     "Windows 提示",
		Severity: SeverityYellow,
		Message:  "Docker Desktop 不支持 Linux 的 network_mode:host；若包内大量 host 网络，请改用 bridge+端口映射，或在 WSL2/Linux 主控机部署。paths 请用本机路径如 D:/workspace",
	}
}

func checkPorts(ports []int) []Item {
	items := make([]Item, 0, len(ports))
	for _, p := range ports {
		inUse, err := portInUse(p)
		if err != nil {
			items = append(items, Item{Name: fmt.Sprintf("端口:%d", p), Severity: SeverityYellow, Message: err.Error()})
			continue
		}
		if inUse {
			items = append(items, Item{
				Name: fmt.Sprintf("端口:%d", p), Severity: SeverityRed, Message: "已被占用",
				Hint: "用 netstat/ss 查占用进程，停掉冲突服务或改 site/manifest 端口后再部署",
			})
		} else {
			items = append(items, Item{Name: fmt.Sprintf("端口:%d", p), Severity: SeverityGreen, Message: "空闲"})
		}
	}
	return items
}

func checkMiddlewareConnectivity(site *config.SiteConfig) []Item {
	c := health.New()
	type ep struct {
		name string
		host string
		port int
	}
	eps := []ep{
		{"连通:Nacos", site.Middleware.Nacos.Host, site.Middleware.Nacos.Port},
		{"连通:Redis", site.Middleware.Redis.Host, site.Middleware.Redis.Port},
		{"连通:Kafka", site.Middleware.Kafka.Host, site.Middleware.Kafka.Port},
	}
	if !site.Middleware.MySQL.Disabled {
		eps = append(eps, ep{"连通:MySQL", site.Middleware.MySQL.Host, site.Middleware.MySQL.Port})
	}
	if site.Middleware.PgSQL.Host != "" && site.Middleware.PgSQL.Port > 0 {
		eps = append(eps, ep{"连通:PgSQL", site.Middleware.PgSQL.Host, site.Middleware.PgSQL.Port})
	}
	var items []Item
	for _, e := range eps {
		if e.host == "" || e.port == 0 {
			continue
		}
		if err := c.ProbeOnce(e.host, e.port); err != nil {
			// 首次部署时中间件可能尚未启动，记为黄色而非红色
			items = append(items, Item{Name: e.name, Severity: SeverityYellow, Message: fmt.Sprintf("%s:%d 不可达（首次部署可忽略）", e.host, e.port)})
		} else {
			items = append(items, Item{Name: e.name, Severity: SeverityGreen, Message: fmt.Sprintf("%s:%d OK", e.host, e.port)})
		}
	}
	return items
}

func versionGTE(cur, min string) bool {
	cp := parseVer(cur)
	mp := parseVer(min)
	for i := 0; i < 3; i++ {
		if cp[i] > mp[i] {
			return true
		}
		if cp[i] < mp[i] {
			return false
		}
	}
	return true
}

func parseVer(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(strings.TrimRightFunc(parts[i], func(r rune) bool {
			return r < '0' || r > '9'
		}))
		out[i] = n
	}
	return out
}

func portInUse(port int) (bool, error) {
	// 跨平台：尝试监听该端口，失败则认为占用。
	err := checkPortListen(port)
	if err != nil {
		if strings.Contains(err.Error(), "占用") {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// WriteReportFile 将 JSON 报告写入文件。
func WriteReportFile(path string, r *Report) error {
	data, err := r.ToJSON()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
