// Package state 扩展：UI 设置与交付单字段。
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/wpg/wpgctl/internal/util"
)

// UISettings 控制台偏好（实施/专家模式、操作者、投屏等）。
type UISettings struct {
	Mode        string `json:"mode"`        // implementer | expert
	Operator    string `json:"operator"`    // 操作者署名
	PrivacyMode bool   `json:"privacyMode"` // 投屏模式：隐藏敏感
	Scenario    string `json:"scenario"`    // windows=本机Docker | linux=现场交付（默认）
	// AdvancedMode 显示一键式部署入口（包中心 / 升级 / 交付单 / 验收报告）。
	AdvancedMode bool `json:"advancedMode"`
	// FieldPaths 现场向导中不属于 site.yaml 的本机路径（middleware/platform 根目录、Docker 离线包、
	// nacos 配置目录、nginx 目录等），按 key/value 持久化，重开控制台后无需重填。
	FieldPaths map[string]string `json:"fieldPaths,omitempty"`
	// Preflight 部署开始前决策（规模 / 登录方式 / 工作簿状态），现场以多机为常态。
	Preflight *PreflightSettings `json:"preflight,omitempty"`
	// UISession 前端会话（当前页、向导进度、未写入 site.yaml 的草稿索引），刷新后恢复。
	UISession map[string]any `json:"uiSession,omitempty"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// PreflightSettings 「开始前」页结论，驱动向导默认拓扑与 Init 是否可跳过。
type PreflightSettings struct {
	// Scale: dual=常用双机 | multi=三机及以上 | single=本机联调/极小规模
	Scale string `json:"scale"`
	// Login: root=可直接 root SSH | sudo=普通用户+非交互 sudo | unsure=未确认
	Login string `json:"login"`
	// Workspace: fresh=未初始化 | ready=已初始化路径清楚 | unknown=不确定
	Workspace string `json:"workspace"`
	// PackageSync: auto=缺目录自动上传 | already=各机已同路径拷好
	PackageSync string `json:"packageSync"`
	Completed   bool   `json:"completed"`
}

// SettingsPath 返回设置文件路径。
func SettingsPath() string {
	return filepath.Join(util.StateDir(), "settings.json")
}

// LoadSettings 读取 UI 设置，不存在则返回默认。
func LoadSettings() (*UISettings, error) {
	path := SettingsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			op := os.Getenv("USERNAME")
			if op == "" {
				op = os.Getenv("USER")
			}
			if op == "" {
				op = "operator"
			}
			return &UISettings{
				Mode:     "implementer",
				Operator: op,
				Scenario: "linux", // 默认现场 Linux 交付
			}, nil
		}
		return nil, err
	}
	var s UISettings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Mode == "" {
		s.Mode = "implementer"
	}
	if s.Operator == "" {
		s.Operator = "operator"
	}
	if s.Scenario != "windows" {
		s.Scenario = "linux"
	}
	return &s, nil
}

// SaveSettings 保存 UI 设置。
func SaveSettings(s *UISettings) error {
	if err := util.EnsureDir(util.StateDir()); err != nil {
		return err
	}
	s.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SettingsPath(), data, 0o644)
}
