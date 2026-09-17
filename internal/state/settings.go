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
	UpdatedAt   time.Time `json:"updatedAt"`
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
