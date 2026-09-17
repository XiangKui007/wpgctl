// Package cmd 定义 wpgctl 全部子命令（cobra）。
//
// 注释风格遵循阿里巴巴规范：包说明、导出符号用途、参数含义清晰可检索。
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wpg/wpgctl/internal/util"
	"github.com/wpg/wpgctl/internal/version"
)

// 全局持久化 flag。
var (
	flagSitePath string
	flagVerbose  bool
)

// rootCmd 根命令。
var rootCmd = &cobra.Command{
	Use:           "wpgctl",
	Short:         "水厂交付部署工具",
	Long:          "wpgctl — 水厂定制化项目现场交付/升级 CLI（环境体检、初始化、部署、补丁升级与回滚）。",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if flagVerbose {
			util.SetLevel(util.LevelDebug)
		}
	},
}

// Execute 执行根命令，供 main 调用。
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		util.Errorf("%v", err)
		return err
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagSitePath, "site", "site.yaml", "站点配置文件路径（site.yaml）")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "输出调试日志")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newPrecheckCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newFetchCmd())
	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(newDBCmd())
	rootCmd.AddCommand(newNacosCmd())
	rootCmd.AddCommand(newFirewallCmd())
	rootCmd.AddCommand(newUpgradeCmd())
	rootCmd.AddCommand(newRollbackCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newDiagCmd())
	rootCmd.AddCommand(newPackCmd())
	rootCmd.AddCommand(newRenderCmd())
	rootCmd.AddCommand(newUICmd())
	rootCmd.AddCommand(newEncryptCmd())
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("wpgctl %s (commit=%s built=%s)\n",
				version.Version, version.GitCommit, version.BuildTime)
		},
	}
}
