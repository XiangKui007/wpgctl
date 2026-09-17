package moduledeploy

import (
	"sort"

	"github.com/wpg/wpgctl/internal/config"
)

// 现场常用端口（附录 A.3，host 网络下以 manifest / 实际监听为准）。
var (
	middlewarePorts = []int{
		3306, 5433, 5432, 27017, 6377, 9092, 2181, 8848, 9848,
		9500, 8086, 1883, 8081, 8083, 8883, 8084, 18083, 8877, 16120, 11005,
	}
	platformPorts = []int{
		18090, 18091, 18092, 18093, 18094, 18095, 18097, 18099,
		30015, 18005, 5588, 11094, 11095,
	}
	monitorPorts     = []int{29090, 3000, 9100}
	waterworkPorts   = []int{11094, 11095, 8088} // 市政水厂常见端口
	intelligentModelPorts = []int{8090, 8091}    // 模型服务（可按现场调整）
)

// PortsForSite 汇总应放行的端口：manifest 优先，否则按 profiles 默认清单 + middleware 配置端口。
func PortsForSite(site *config.SiteConfig, mf *config.Manifest) []int {
	if mf != nil && site != nil {
		return mf.Ports(site.Profiles)
	}
	set := map[int]struct{}{}
	add := func(list []int) {
		for _, p := range list {
			set[p] = struct{}{}
		}
	}
	add(middlewarePorts)
	if site != nil {
		m := site.Middleware
		if !m.MySQL.Disabled && m.MySQL.Port > 0 {
			set[m.MySQL.Port] = struct{}{}
		}
		if m.PgSQL.Port > 0 {
			set[m.PgSQL.Port] = struct{}{}
		}
		if m.Redis.Port > 0 {
			set[m.Redis.Port] = struct{}{}
		}
		if m.Nacos.Port > 0 {
			set[m.Nacos.Port] = struct{}{}
			set[m.Nacos.Port+1000] = struct{}{} // 9848 gRPC
		}
		if m.Kafka.Port > 0 {
			set[m.Kafka.Port] = struct{}{}
		}
		pe := map[string]bool{}
		for _, p := range site.Profiles {
			pe[p] = true
		}
		if pe["platform"] {
			add(platformPorts)
		}
		if pe["waterwork"] {
			add(waterworkPorts)
		}
		if pe["intelligent-model"] {
			add(intelligentModelPorts)
		}
		if pe["monitor"] {
			add(monitorPorts)
		}
	} else {
		add(platformPorts)
		add(monitorPorts)
	}
	out := make([]int, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}
