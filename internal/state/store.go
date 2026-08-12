// Package state 管理本地部署台账（~/.wpgctl/state/）。
//
// 部署记录是 upgrade 基线校验与 rollback 的唯一真相来源（方案 §6.12）。
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wpg/wpgctl/internal/util"
)

// DeploymentRecord 单次部署/升级/回滚记录。
type DeploymentRecord struct {
	ID           string    `json:"id"`
	Time         time.Time `json:"time"`
	Operator     string    `json:"operator"`
	Action       string    `json:"action"` // deploy / upgrade / rollback
	PackageKind  string    `json:"packageKind"`
	PackageVer   string    `json:"packageVersion"`
	SiteCode     string    `json:"siteCode"`
	SiteHash     string    `json:"siteHash"`
	Success      bool      `json:"success"`
	DurationSec  int64     `json:"durationSec"`
	Message      string    `json:"message,omitempty"`
	ServiceTags  map[string]string `json:"serviceTags,omitempty"` // 服务名 -> 镜像 tag
}

// Store 本地 JSON 台账存储。
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore 创建台账存储，必要时自动建目录。
func NewStore() (*Store, error) {
	dir := util.StateDir()
	if err := util.EnsureDir(dir); err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(dir, "deployments.json")}, nil
}

// List 返回全部记录（时间倒序）。
func (s *Store) List() ([]DeploymentRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readUnlocked()
}

// LatestSuccess 返回最近一次成功记录，无则返回 nil。
func (s *Store) LatestSuccess(siteCode string) (*DeploymentRecord, error) {
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	for i := len(list) - 1; i >= 0; i-- {
		r := list[i]
		if !r.Success {
			continue
		}
		if siteCode != "" && r.SiteCode != siteCode {
			continue
		}
		cp := r
		return &cp, nil
	}
	return nil, nil
}

// Append 追加一条记录。
func (s *Store) Append(rec DeploymentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.readUnlocked()
	if err != nil {
		return err
	}
	if rec.ID == "" {
		rec.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if rec.Time.IsZero() {
		rec.Time = time.Now()
	}
	list = append(list, rec)
	return s.writeUnlocked(list)
}

func (s *Store) readUnlocked() ([]DeploymentRecord, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []DeploymentRecord{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return []DeploymentRecord{}, nil
	}
	var list []DeploymentRecord
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("解析部署台账失败: %w", err)
	}
	return list, nil
}

func (s *Store) writeUnlocked(list []DeploymentRecord) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
