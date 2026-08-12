package util

import (
	"os"
	"path/filepath"
)

// HomeDir 返回 wpgctl 本地状态根目录（~/.wpgctl）。
//
// 可通过环境变量 WPGCTL_HOME 覆盖，便于测试与多实例隔离。
func HomeDir() string {
	if v := os.Getenv("WPGCTL_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".wpgctl")
	}
	return filepath.Join(home, ".wpgctl")
}

// StateDir 返回部署状态目录。
func StateDir() string {
	return filepath.Join(HomeDir(), "state")
}

// PackagesDir 返回本地包仓库目录。
func PackagesDir() string {
	return filepath.Join(HomeDir(), "packages")
}

// EnsureDir 确保目录存在（幂等）。
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// FileExists 判断路径是否存在且为文件。
func FileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// DirExists 判断路径是否存在且为目录。
func DirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
