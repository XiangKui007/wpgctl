// Package config 定义 site.yaml / manifest.yaml 的数据结构与严格校验。
//
// 设计原则（对齐方案 §4 / §5）：
// 1. site.yaml 是现场唯一配置源，运行时 env/compose 由其渲染生成；
// 2. manifest.yaml 驱动部署行为（镜像、分层、健康检查、profile）；
// 3. 未知字段 / 非法值一律报错，把手误消灭在部署前。
package config

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/wpg/wpgctl/internal/vault"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// site.yaml
// ---------------------------------------------------------------------------

// SiteConfig 现场站点配置（唯一配置源）。
type SiteConfig struct {
	Site       SiteInfo            `yaml:"site" json:"site"`
	Nodes      []Node              `yaml:"nodes" json:"nodes"`
	Profiles   []string            `yaml:"profiles" json:"profiles"`
	Middleware MiddlewareConfig    `yaml:"middleware" json:"middleware"`
	Overrides  map[string]Override `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	Paths      PathsConfig         `yaml:"paths" json:"paths"`
	// FetchBaseURL 公司侧拉包地址，可选。
	FetchBaseURL string `yaml:"fetchBaseUrl,omitempty" json:"fetchBaseUrl,omitempty"`
}

// SiteInfo 站点基本信息。
type SiteInfo struct {
	Name string `yaml:"name" json:"name"`
	Code string `yaml:"code" json:"code"`
}

// Node 机器节点定义。
type Node struct {
	Name  string   `yaml:"name" json:"name"`
	IP    string   `yaml:"ip" json:"ip"`
	SSH   SSHAuth  `yaml:"ssh" json:"ssh"`
	Roles []string `yaml:"roles" json:"roles"`
	// Services 记录表单分配到本机的具体服务；Roles 保留用于兼容部署编排。
	Services []string `yaml:"services,omitempty" json:"services,omitempty"`
}

// SSHAuth SSH 连接参数（密码运行时交互或密钥，不强制写入文件）。
type SSHAuth struct {
	User string `yaml:"user" json:"user"`
	Port int    `yaml:"port" json:"port"`
}

// MiddlewareConfig 中间件连接参数，渲染进全部 env / nacos 模板。
type MiddlewareConfig struct {
	Nacos NacosConn `yaml:"nacos" json:"nacos"`
	MySQL DBConn    `yaml:"mysql" json:"mysql"`
	PgSQL DBConn    `yaml:"pgsql,omitempty" json:"pgsql,omitempty"`
	Redis RedisConn `yaml:"redis" json:"redis"`
	Kafka KafkaConn `yaml:"kafka" json:"kafka"`
}

// NacosConn Nacos 连接信息。
type NacosConn struct {
	Host      string `yaml:"host" json:"host"`
	Port      int    `yaml:"port" json:"port"`
	Namespace string `yaml:"namespace" json:"namespace"`
	Username  string `yaml:"username" json:"username"`
	Password  string `yaml:"password" json:"password"`
}

// DBConn 数据库连接信息（MySQL / PostgreSQL 通用）。
type DBConn struct {
	Disabled bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"` // MySQL：true 表示不部署/不依赖
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	User     string `yaml:"user" json:"user"`
	Password string `yaml:"password" json:"password"`
	Database string `yaml:"database,omitempty" json:"database,omitempty"` // 可选；各业务 .env 自有库名，site 级一般不统一
}

// RedisConn Redis 连接信息。
type RedisConn struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Password string `yaml:"password" json:"password"`
}

// KafkaConn Kafka 连接信息。
type KafkaConn struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

// Override 服务级覆盖项（如 JVM 堆内存）。
type Override struct {
	Xmx string `yaml:"xmx,omitempty" json:"xmx,omitempty"`
}

// PathsConfig 现场路径规划。
type PathsConfig struct {
	Workspace        string `yaml:"workspace" json:"workspace"` // 挂载盘工作簿根，现场默认 /workspace
	Logs             string `yaml:"logs,omitempty" json:"logs,omitempty"` // 可选；各模块 compose 通常已挂载日志目录
	NginxHTML        string `yaml:"nginxHtml" json:"nginxHtml"`
	Waterwork        string `yaml:"waterwork,omitempty" json:"waterwork,omitempty"`               // 市政水厂包目录，如 waterwork-4.1.1
	IntelligentModel string `yaml:"intelligentModel,omitempty" json:"intelligentModel,omitempty"` // 模型服务包目录，如 wpg-intelligent-model-4.1.2
}

// ---------------------------------------------------------------------------
// manifest.yaml
// ---------------------------------------------------------------------------

// Manifest 交付包清单，驱动 deploy / upgrade / status 等全部行为。
type Manifest struct {
	Kind         string         `yaml:"kind"`
	Version      string         `yaml:"version"`
	RequiresBase string         `yaml:"requiresBase,omitempty"`
	BaseRelease  string         `yaml:"baseRelease,omitempty"` // patch 包基线 release
	Arch         []string       `yaml:"arch"`
	MinCPU       int            `yaml:"minCpu,omitempty"`
	MinMemGB     int            `yaml:"minMemGb,omitempty"`
	MinDiskGB    int            `yaml:"minDiskGb,omitempty"`
	Services     []ServiceSpec  `yaml:"services"`
	Frontend     []FrontendSpec `yaml:"frontend,omitempty"`
	SQL          []SQLSpec      `yaml:"sql,omitempty"`
	ServicesList []string       `yaml:"servicesList,omitempty"` // patch 涉及服务名
}

// ServiceSpec 单个服务规格。
type ServiceSpec struct {
	Name          string            `yaml:"name"`
	Image         string            `yaml:"image"`
	Profile       string            `yaml:"profile,omitempty"`
	Layer         int               `yaml:"layer"`
	Port          int               `yaml:"port,omitempty"`
	Health        HealthSpec        `yaml:"health,omitempty"`
	ComposeFlags  []string          `yaml:"composeFlags,omitempty"`
	RequiredFiles []string          `yaml:"requiredFiles,omitempty"`
	Roles         []string          `yaml:"roles,omitempty"` // 部署到哪些 node role
	Extra         map[string]string `yaml:"extra,omitempty"`
}

// HealthSpec 健康检查规格。
type HealthSpec struct {
	Type        string `yaml:"type"` // http / tcp / none
	Path        string `yaml:"path,omitempty"`
	TimeoutSec  int    `yaml:"timeoutSec,omitempty"`
	IntervalSec int    `yaml:"intervalSec,omitempty"`
}

// FrontendSpec 前端静态包规格。
type FrontendSpec struct {
	Name            string `yaml:"name"`
	Archive         string `yaml:"archive"`
	NginxHTMLSubdir string `yaml:"nginxHtmlSubdir"`
}

// SQLSpec SQL 目录规格。
type SQLSpec struct {
	Dir      string `yaml:"dir"`
	Database string `yaml:"database"`
	Driver   string `yaml:"driver,omitempty"` // mysql / pgsql，默认 mysql
}

// ---------------------------------------------------------------------------
// 加载与校验
// ---------------------------------------------------------------------------

var (
	reSiteCode = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{1,63}$`)
	reVersion  = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([._-][a-zA-Z0-9._-]+)?$`)
	// 合法 profile 字典（platform 子模块 + 独立业务包 profile）。
	validProfiles = map[string]struct{}{
		"platform":          {},
		"waterwork":         {}, // 市政水厂（独立包）
		"intelligent-model": {}, // 模型服务（独立包）
		"device":            {},
		"alarm":             {},
		"gis":               {},
		"monitor":           {},
		"graph":             {},
	}
)

// LoadSite 从文件加载并严格校验 site.yaml。
func LoadSite(path string) (*SiteConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 site.yaml 失败: %w", err)
	}

	var dec yaml.Node
	if err := yaml.Unmarshal(data, &dec); err != nil {
		return nil, fmt.Errorf("解析 site.yaml 失败: %w", err)
	}

	cfg := &SiteConfig{}
	decStrict := yaml.NewDecoder(strings.NewReader(string(data)))
	decStrict.KnownFields(true)
	if err := decStrict.Decode(cfg); err != nil {
		return nil, fmt.Errorf("site.yaml 字段非法或存在未知字段: %w", err)
	}
	cfg.Normalize()
	if err := cfg.ResolveSecrets(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Normalize 升级旧版 site.yaml 字段（如已废弃的 profile 名）。
func (c *SiteConfig) Normalize() {
	if c == nil {
		return
	}
	c.Profiles = normalizeProfileList(c.Profiles)
	for i := range c.Nodes {
		c.Nodes[i].Roles = normalizeRoleList(c.Nodes[i].Roles)
	}
}

func normalizeProfileList(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, p := range in {
		if p == "smartwater" {
			p = "waterwork"
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func normalizeRoleList(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, r := range in {
		if r == "smartwater" {
			r = "waterwork"
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	return out
}

// ResolveSecrets 解密 middleware 中带 !vault: 前缀的密码字段。
// 口令来自环境变量 WPGCTL_VAULT_KEY；无加密字段时不要求该变量。
func (c *SiteConfig) ResolveSecrets() error {
	needKey := vault.IsVault(c.Middleware.Nacos.Password) ||
		vault.IsVault(c.Middleware.MySQL.Password) ||
		vault.IsVault(c.Middleware.Redis.Password) ||
		vault.IsVault(c.Middleware.PgSQL.Password)
	if !needKey {
		return nil
	}
	pass, err := vault.PassphraseFromEnv()
	if err != nil {
		return err
	}
	decrypt := func(v *string) error {
		out, err := vault.Decrypt(*v, pass)
		if err != nil {
			return err
		}
		*v = out
		return nil
	}
	if err := decrypt(&c.Middleware.Nacos.Password); err != nil {
		return fmt.Errorf("nacos.password: %w", err)
	}
	if err := decrypt(&c.Middleware.MySQL.Password); err != nil {
		return fmt.Errorf("mysql.password: %w", err)
	}
	if err := decrypt(&c.Middleware.Redis.Password); err != nil {
		return fmt.Errorf("redis.password: %w", err)
	}
	if c.Middleware.PgSQL.Password != "" {
		if err := decrypt(&c.Middleware.PgSQL.Password); err != nil {
			return fmt.Errorf("pgsql.password: %w", err)
		}
	}
	return nil
}

// LoadManifest 从文件加载并严格校验 manifest.yaml。
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 manifest.yaml 失败: %w", err)
	}
	m := &Manifest{}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(m); err != nil {
		return nil, fmt.Errorf("manifest.yaml 字段非法或存在未知字段: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// Validate 校验站点配置完整性与合法性。
func (c *SiteConfig) Validate() error {
	var errs []string

	if strings.TrimSpace(c.Site.Name) == "" {
		errs = append(errs, "site.name 不能为空")
	}
	if !reSiteCode.MatchString(c.Site.Code) {
		errs = append(errs, "site.code 格式非法（字母数字开头，长度 2~64）")
	}
	if len(c.Nodes) == 0 {
		errs = append(errs, "nodes 至少需要一台机器")
	}
	for i, n := range c.Nodes {
		if n.Name == "" {
			errs = append(errs, fmt.Sprintf("nodes[%d].name 不能为空", i))
		}
		if ip := net.ParseIP(n.IP); ip == nil {
			errs = append(errs, fmt.Sprintf("nodes[%d].ip 非法: %s", i, n.IP))
		}
		if n.SSH.User == "" {
			errs = append(errs, fmt.Sprintf("nodes[%d].ssh.user 不能为空", i))
		}
		if n.SSH.Port == 0 {
			c.Nodes[i].SSH.Port = 22
		}
		if n.SSH.Port < 1 || n.SSH.Port > 65535 {
			errs = append(errs, fmt.Sprintf("nodes[%d].ssh.port 超出范围", i))
		}
		if len(n.Roles) == 0 {
			errs = append(errs, fmt.Sprintf("nodes[%d].roles 不能为空", i))
		}
	}
	if len(c.Profiles) == 0 {
		errs = append(errs, "profiles 不能为空")
	}
	for _, p := range c.Profiles {
		if _, ok := validProfiles[p]; !ok {
			errs = append(errs, fmt.Sprintf("非法 profile: %s", p))
		}
	}

	errs = append(errs, validateMiddleware(c.Middleware)...)

	if c.Paths.Workspace == "" {
		errs = append(errs, "paths.workspace 不能为空")
	}
	if c.Paths.NginxHTML == "" {
		errs = append(errs, "paths.nginxHtml 不能为空")
	}

	if len(errs) > 0 {
		return fmt.Errorf("site.yaml 校验失败:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func validateMiddleware(m MiddlewareConfig) []string {
	var errs []string
	checkHostPort := func(name, host string, port int) {
		if host == "" {
			errs = append(errs, name+".host 不能为空")
		} else if net.ParseIP(host) == nil && !isHostname(host) {
			errs = append(errs, name+".host 非法: "+host)
		}
		if port < 1 || port > 65535 {
			errs = append(errs, name+".port 超出范围")
		}
	}
	checkHostPort("middleware.nacos", m.Nacos.Host, m.Nacos.Port)
	if m.Nacos.Password == "" {
		errs = append(errs, "middleware.nacos.password 不能为空")
	}
	if m.Nacos.Namespace == "" {
		errs = append(errs, "middleware.nacos.namespace 不能为空")
	}
	if !m.MySQL.Disabled {
		checkHostPort("middleware.mysql", m.MySQL.Host, m.MySQL.Port)
		if m.MySQL.User == "" || m.MySQL.Password == "" {
			errs = append(errs, "middleware.mysql.user/password 不能为空")
		}
	} else {
		checkHostPort("middleware.pgsql", m.PgSQL.Host, m.PgSQL.Port)
		if m.PgSQL.User == "" || m.PgSQL.Password == "" {
			errs = append(errs, "middleware.pgsql.user/password 不能为空（已跳过 MySQL）")
		}
	}
	checkHostPort("middleware.redis", m.Redis.Host, m.Redis.Port)
	if m.Redis.Password == "" {
		errs = append(errs, "middleware.redis.password 不能为空")
	}
	checkHostPort("middleware.kafka", m.Kafka.Host, m.Kafka.Port)
	if !m.MySQL.Disabled && m.PgSQL.Host != "" {
		checkHostPort("middleware.pgsql", m.PgSQL.Host, m.PgSQL.Port)
	}
	return errs
}

func isHostname(s string) bool {
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if part == "" || len(part) > 63 {
			return false
		}
	}
	return true
}

// Validate 校验 manifest。
func (m *Manifest) Validate() error {
	var errs []string
	switch m.Kind {
	case "base", "release", "patch":
	default:
		errs = append(errs, "kind 必须是 base|release|patch")
	}
	if !reVersion.MatchString(m.Version) {
		errs = append(errs, "version 格式非法: "+m.Version)
	}
	if len(m.Arch) == 0 {
		errs = append(errs, "arch 不能为空")
	}
	if m.Kind == "release" || m.Kind == "base" {
		if len(m.Services) == 0 {
			errs = append(errs, "services 不能为空")
		}
		for i, s := range m.Services {
			if s.Name == "" {
				errs = append(errs, fmt.Sprintf("services[%d].name 不能为空", i))
			}
			if s.Image == "" {
				errs = append(errs, fmt.Sprintf("services[%d].image 不能为空", i))
			}
			if s.Layer < 1 || s.Layer > 4 {
				errs = append(errs, fmt.Sprintf("services[%d].layer 必须在 1~4", i))
			}
			if s.Health.Type == "" {
				m.Services[i].Health.Type = "http"
			}
			if m.Services[i].Health.TimeoutSec == 0 {
				m.Services[i].Health.TimeoutSec = 120
			}
			if m.Services[i].Health.IntervalSec == 0 {
				m.Services[i].Health.IntervalSec = 5
			}
			if m.Services[i].Health.Type == "http" && m.Services[i].Health.Path == "" {
				m.Services[i].Health.Path = "/actuator/health"
			}
		}
	}
	if m.Kind == "patch" && m.BaseRelease == "" {
		errs = append(errs, "patch 包必须声明 baseRelease")
	}
	if len(errs) > 0 {
		return fmt.Errorf("manifest.yaml 校验失败:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// EnabledServices 按 site profiles 过滤应部署的服务。
// profile 为空的服务视为中间件基础件，始终启用。
func (m *Manifest) EnabledServices(profiles []string) []ServiceSpec {
	set := make(map[string]struct{}, len(profiles))
	for _, p := range profiles {
		set[p] = struct{}{}
	}
	out := make([]ServiceSpec, 0, len(m.Services))
	for _, s := range m.Services {
		if s.Profile == "" {
			out = append(out, s)
			continue
		}
		if _, ok := set[s.Profile]; ok {
			out = append(out, s)
		}
	}
	return out
}

// Ports 汇总启用服务的端口清单（供 precheck / 防火墙使用）。
func (m *Manifest) Ports(profiles []string) []int {
	seen := map[int]struct{}{}
	var ports []int
	for _, s := range m.EnabledServices(profiles) {
		if s.Port <= 0 {
			continue
		}
		if _, ok := seen[s.Port]; ok {
			continue
		}
		seen[s.Port] = struct{}{}
		ports = append(ports, s.Port)
	}
	return ports
}
