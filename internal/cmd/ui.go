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
		Long:  "Linux 默认监听 0.0.0.0:9527（可用服务器 IP 访问）；Windows 默认 127.0.0.1:9527。",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.EnsureSitePathExists(flagSitePath)
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return ui.Start(ctx, ui.Options{Listen: listen, SitePath: flagSitePath})
		},
	}
	c.Flags().StringVar(&listen, "listen", ui.DefaultListen(), "监听地址，如 0.0.0.0:9527 或 127.0.0.1:9527")
	return c
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
