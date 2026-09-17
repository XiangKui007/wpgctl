// Package moduledeploy 按 /waterwork 目录结构逐步部署（对齐现场 SOP）。
package moduledeploy

// Phase 部署阶段。
type Phase string

const (
	PhaseDatabase   Phase = "database"
	PhaseMiddleware Phase = "middleware"
	PhaseBusiness   Phase = "business"
	PhaseStandalone Phase = "standalone" // 市政水厂、模型服务等独立包（非 platform/middleware 子目录）
)

// ModuleSpec 模块目录名（相对 middleware / platform 根）。
type ModuleSpec struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

// Modules 返回某阶段按顺序部署的模块清单。
func Modules(phase Phase) []ModuleSpec {
	switch phase {
	case PhaseDatabase:
		return []ModuleSpec{
			{Name: "mysql", Label: "MySQL", Required: false},
			{Name: "pgsql", Label: "PostgreSQL", Required: true},
			{Name: "postgis", Label: "PostGIS", Required: false},
			{Name: "mongodb", Label: "MongoDB(GIS)", Required: false},
		}
	case PhaseMiddleware:
		return []ModuleSpec{
			{Name: "redis", Label: "Redis", Required: true},
			{Name: "kafka", Label: "Kafka+ZK", Required: true},
			{Name: "nacos", Label: "Nacos", Required: true},
			{Name: "minio", Label: "MinIO", Required: false},
			{Name: "influxdb", Label: "InfluxDB", Required: false},
			{Name: "emqx", Label: "EMQX", Required: false},
		}
	case PhaseBusiness:
		return []ModuleSpec{
			{Name: "public", Label: "平台 public", Required: true},
			{Name: "device", Label: "设备 device", Required: false},
			{Name: "alarm", Label: "报警 alarm", Required: false},
			{Name: "graph", Label: "组态 graph", Required: false},
			{Name: "gis", Label: "GIS", Required: false},
			{Name: "monitor", Label: "监控 monitor", Required: false},
			{Name: "report-center", Label: "报表 report-center", Required: false},
			{Name: "out-work", Label: "out-work", Required: false},
		}
	case PhaseStandalone:
		return []ModuleSpec{
			{Name: "waterwork", Label: "市政水厂", Required: false},
			{Name: "intelligent-model", Label: "模型服务", Required: false},
		}
	default:
		return nil
	}
}
