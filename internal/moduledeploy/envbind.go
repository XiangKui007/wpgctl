package moduledeploy

import (
	"strconv"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
)

// envBinder 把「节点上勾的服务」绑到业务 .env 键族。
//
// 拓展新中间件：在 envBinders 加一行（Service 与目录名 / site.yaml services id 相同）。
// 不要在 patchedEnvValue 里再写 if strings.Contains。
// Nacos / Redis / Kafka / 业务库仍走 site.yaml middleware 表单，不放这里。
// GIS 的 WPG_PGSQL_* / WPG_MONGODB_* 在 gis_env.go 按目录处理，不要把 WPG_PGSQL 配进 PostGIS（会误伤业务库）。
type envBinder struct {
	Service   string     // nodes[].services 中的 id，也是 middleware 包目录名
	Role      string     // 无 services 列表时按角色猜 IP；空则 middleware
	Port      int        // 组件默认端口（site.yaml 尚未为这些组件建独立表单）
	Fallback  string     // 找不到节点时：nacos / pgsql
	AuthFrom  string     // pgsql：账号密码沿用业务 PgSQL（PostGIS 同标准库）
	Token     string     // 固定访问令牌（如 XXL-JOB）；空则不改 token 键
	Match     []string   // 键名（大写）包含其一即视为该组件
	MatchAll  [][]string // 键名须同时包含一组子串（如 GIS + DATASOURCE）
	HostKeys  []string
	PortKeys  []string
	UserKeys  []string
	PassKeys  []string
	TokenKeys []string // 与 Token 对应的键（XXLJOB_TOKEN 等，不是密码后缀）
}

// envBinders 按节点规划填充的可选中间件。顺序即匹配优先级（先写更具体的）。
var envBinders = []envBinder{
	{
		// 容器内仍是 9000；compose 映射 9500:9000，业务从宿主机连的是 9500。
		Service:  "minio",
		Port:     9500,
		Fallback: "nacos",
		Match:    []string{"MINIO"},
		HostKeys: []string{"MINIO_HOST", "MINIO_IP", "MINIO_HOSTNAME", "MINIO_ADDR"},
		PortKeys: []string{"MINIO_PORT"},
	},
	{
		Service:  "influxdb",
		Port:     8086,
		Fallback: "nacos",
		Match:    []string{"INFLUX"},
		HostKeys: []string{"INFLUX_HOST", "INFLUXDB_HOST", "INFLUX_IP", "INFLUXDB_IP", "INFLUX_HOSTNAME"},
		PortKeys: []string{"INFLUX_PORT", "INFLUXDB_PORT"},
	},
	{
		Service:  "emqx",
		Port:     1883,
		Fallback: "nacos",
		Match:    []string{"EMQX", "MQTT"},
		HostKeys: []string{"EMQX_HOST", "EMQX_IP", "MQTT_HOST", "MQTT_IP", "MQTT_BROKER", "MQTT_BROKER_HOST", "MQTT_SERVER"},
		PortKeys: []string{"EMQX_PORT", "MQTT_PORT", "MQTT_BROKER_PORT"},
	},
	{
		Service:  "postgis",
		Role:     "database",
		Port:     5432,
		Fallback: "pgsql",
		AuthFrom: "pgsql",
		Match:    []string{"POSTGIS", "GIS_PG", "GIS_POSTGRES", "GIS_POSTGIS", "GIS_DB"},
		MatchAll: [][]string{{"GIS", "DATASOURCE"}},
		HostKeys: []string{"POSTGIS_HOST", "POSTGIS_ADDR", "POSTGIS_IP", "POSTGIS_HOSTNAME", "GIS_PGSQL_HOST", "GIS_PG_HOST", "GIS_POSTGRES_HOST", "GIS_DB_HOST"},
		PortKeys: []string{"POSTGIS_PORT", "GIS_PGSQL_PORT", "GIS_PG_PORT", "GIS_POSTGRES_PORT", "GIS_DB_PORT"},
		UserKeys: []string{"POSTGIS_USER", "POSTGIS_USERNAME", "GIS_PGSQL_USER", "GIS_PG_USER", "GIS_POSTGRES_USER", "GIS_DB_USER"},
		PassKeys: []string{"POSTGIS_PASSWORD", "POSTGIS_PASSWD", "GIS_PGSQL_PASSWORD", "GIS_PG_PASSWORD", "GIS_POSTGRES_PASSWORD", "GIS_DB_PASSWORD"},
	},
	{
		// WaterJob / XXL-JOB：对外 11005（device 的 XXLJOB_PORT）；TOKEN 为产品默认，不走 site.yaml 表单。
		Service:   "waterjob",
		Port:      11005,
		Fallback:  "nacos",
		Token:     "wpg@2020",
		Match:     []string{"XXLJOB", "XXL_JOB", "WATERJOB", "WATER_JOB"},
		HostKeys:  []string{"XXLJOB_HOST", "XXL_JOB_HOST", "XXLJOB_IP", "XXL_JOB_IP", "XXLJOB_ADMIN_HOST", "XXL_JOB_ADMIN_HOST"},
		PortKeys:  []string{"XXLJOB_PORT", "XXL_JOB_PORT", "XXLJOB_ADMIN_PORT", "XXL_JOB_ADMIN_PORT"},
		TokenKeys: []string{"XXLJOB_TOKEN", "XXL_JOB_TOKEN", "XXLJOB_ACCESS_TOKEN", "XXL_JOB_ACCESS_TOKEN"},
	},
}

func putBinderKeys(repl map[string]string, site *config.SiteConfig) {
	for i := range envBinders {
		b := &envBinders[i]
		c := connFromBinder(site, b)
		putKeys(repl, c.host, b.HostKeys...)
		if c.port > 0 {
			putKeys(repl, strconv.Itoa(c.port), b.PortKeys...)
		}
		putKeys(repl, c.user, b.UserKeys...)
		putKeys(repl, c.pass, b.PassKeys...)
		putKeys(repl, b.Token, b.TokenKeys...)
	}
}

func binderForKey(key string) *envBinder {
	k := strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
	for i := range envBinders {
		b := &envBinders[i]
		for _, m := range b.Match {
			if m != "" && strings.Contains(k, m) {
				return b
			}
		}
		for _, group := range b.MatchAll {
			ok := len(group) > 0
			for _, p := range group {
				if !strings.Contains(k, p) {
					ok = false
					break
				}
			}
			if ok {
				return b
			}
		}
	}
	return nil
}

func connFromBinder(site *config.SiteConfig, b *envBinder) svcConn {
	if site == nil || b == nil {
		return svcConn{}
	}
	c := connOf(site, b.Service, fallbackHost(site, b.Fallback), b.Port, b.Role == "database")
	if b.AuthFrom == "pgsql" {
		c.user = strings.TrimSpace(site.Middleware.PgSQL.User)
		c.pass = site.Middleware.PgSQL.Password
	}
	return c
}

func fallbackHost(site *config.SiteConfig, kind string) string {
	if site == nil {
		return ""
	}
	switch kind {
	case "nacos":
		return strings.TrimSpace(site.Middleware.Nacos.Host)
	case "pgsql":
		return strings.TrimSpace(site.Middleware.PgSQL.Host)
	default:
		return ""
	}
}
