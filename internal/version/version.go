// Package version 提供构建版本信息，由 CI 通过 -ldflags 注入。
package version

// 构建期注入变量，默认值为开发态占位。
var (
	// Version 语义化版本号，例如 1.0.0。
	Version = "dev"
	// GitCommit 短 commit hash。
	GitCommit = "unknown"
	// BuildTime 构建时间（UTC）。
	BuildTime = "unknown"
)
