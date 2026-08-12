// Package main 是 wpgctl 命令行工具的入口。
//
// wpgctl：水厂交付部署工具，负责现场环境体检、初始化、配置渲染、
// 分层部署、补丁升级与回滚等全流程自动化。
package main

import (
	"os"

	"github.com/wpg/wpgctl/internal/cmd"
)

// main 程序入口，将控制权交给 cobra 命令树。
func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
