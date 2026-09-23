package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/wpg/wpgctl/internal/ui"
	"github.com/wpg/wpgctl/internal/util"
	"github.com/wpg/wpgctl/internal/vault"
)

func newUICmd() *cobra.Command {
	var listen string
	c := &cobra.Command{
		Use:   "ui",
		Short: "启动本地 Web 控制台（向导部署 / 状态 / 日志）",
		Long: `Linux 默认监听 0.0.0.0:9527（可用服务器 IP 访问）；Windows 默认 127.0.0.1:9527。

  wpgctl ui                 前台运行（关掉终端即退出）
  wpgctl ui start           后台运行
  wpgctl ui stop            停止后台
  wpgctl ui status          查看是否在跑
  wpgctl ui install         Linux：安装 systemd 并开机自启
  wpgctl ui uninstall       Linux：卸载 systemd 单元`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.EnsureSitePathExists(flagSitePath)
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return ui.Start(ctx, ui.Options{Listen: listen, SitePath: flagSitePath})
		},
	}
	c.PersistentFlags().StringVar(&listen, "listen", ui.DefaultListen(), "监听地址，如 0.0.0.0:9527 或 127.0.0.1:9527")
	c.AddCommand(newUIStartCmd(&listen))
	c.AddCommand(newUIStopCmd())
	c.AddCommand(newUIStatusCmd())
	c.AddCommand(newUIInstallCmd(&listen))
	c.AddCommand(newUIUninstallCmd())
	return c
}

func newUIStartCmd(listen *string) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "后台启动 Web 控制台（不占用当前终端）",
		Long:  "已 install 过 systemd 则走 systemctl；否则拉起独立进程，日志写在 ~/.wpgctl/logs/ui.log。",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.EnsureSitePathExists(flagSitePath)
			return ui.StartDaemon(ui.DaemonOptions{Listen: *listen, SitePath: flagSitePath})
		},
	}
}

func newUIStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "停止后台 Web 控制台",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ui.StopDaemon()
		},
	}
}

func newUIStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "查看后台 Web 控制台是否在运行",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ui.PrintDaemonStatus()
		},
	}
}

func newUIInstallCmd(listen *string) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Linux：安装 systemd 服务并立即启动（开机自启）",
		Long:  "写入 /etc/systemd/system/wpgctl-ui.service 并 enable --now。需要 root 或 sudo。",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.EnsureSitePathExists(flagSitePath)
			return ui.InstallDaemon(ui.DaemonOptions{Listen: *listen, SitePath: flagSitePath})
		},
	}
}

func newUIUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Linux：卸载 systemd 服务",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ui.UninstallDaemon()
		},
	}
}

func newEncryptCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "encrypt [plaintext]",
		Short: "将明文密码加密为 !vault: 标记（写入 site.yaml）",
		Long:  "口令来自环境变量 WPGCTL_VAULT_KEY。生成的 !vault:... 可直接作为 password 字段值。",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pass, err := vault.PassphraseFromEnv()
			if err != nil {
				return err
			}
			out, err := vault.Encrypt(args[0], pass)
			if err != nil {
				return err
			}
			fmt.Println(out)
			util.Infof("已生成密文，请粘贴到 site.yaml 对应 password 字段")
			return nil
		},
	}
	return c
}
