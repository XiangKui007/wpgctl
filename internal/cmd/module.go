package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/moduledeploy"
	"github.com/wpg/wpgctl/internal/nacos"
	"github.com/wpg/wpgctl/internal/util"
)

// newModuleCmd 现场"按模块目录"部署命令。UI 多机分发时会在目标机上远程调用本命令，
// 因此输出必须逐行、可读（由主控机按行回传到控制台日志）。
func newModuleCmd() *cobra.Command {
	moduleCmd := &cobra.Command{
		Use:   "module",
		Short: "现场单模块目录操作（解压 / load / 改 env / compose up）",
	}
	moduleCmd.AddCommand(newModuleDeployCmd())
	moduleCmd.AddCommand(newModuleNginxCmd())
	return moduleCmd
}

func newModuleDeployCmd() *cobra.Command {
	var (
		dir          string
		expand       bool
		load         bool
		patchEnv     bool
		composeUp    bool
		composeBuild bool
		nacosZips    []string
	)
	c := &cobra.Command{
		Use:   "deploy",
		Short: "部署单个模块目录：解压 tar.zip → docker load → 改 .env → compose up",
		Long: "等价于控制台向导 ③～⑥ 步中的一个模块按钮。多机部署时主控机会把本命令分发到目标机执行。\n" +
			"未指定任何动作 flag 时默认 --expand --load --compose-up。",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				return fmt.Errorf("请指定 --dir 模块目录")
			}
			if !util.DirExists(dir) {
				return fmt.Errorf("模块目录不存在: %s", dir)
			}
			site, err := config.LoadSite(flagSitePath)
			if err != nil {
				return err
			}
			if composeBuild {
				composeUp, expand, load = true, false, false
			} else if !expand && !load && !patchEnv && !composeUp {
				expand, load, composeUp = true, true, true
			}
			util.Infof("模块部署: %s", dir)
			res, err := moduledeploy.Run(moduledeploy.Options{
				ModuleDir:    dir,
				Site:         site,
				Expand:       expand,
				Load:         load,
				PatchEnv:     patchEnv,
				ComposeUp:    composeUp,
				ComposeBuild: composeBuild,
			})
			if res != nil {
				for _, s := range res.Steps {
					util.Infof("%s", s)
				}
			}
			if err != nil {
				return err
			}
			for _, q := range moduledeploy.ListInitSQL(dir) {
				util.Infof("SQL: %s", q)
			}
			if len(nacosZips) > 0 && moduledeploy.IsNacosModule(dir) {
				util.Infof("Nacos 已启动，开始上传导入 zip（不解压）: %v", nacosZips)
				if _, err := nacos.Run(nacos.Options{Site: site, ConfigZips: nacosZips}); err != nil {
					return fmt.Errorf("Nacos 导入失败: %w", err)
				}
				util.Successf("Nacos 配置导入完成")
			}
			util.Successf("模块部署完成: %s", filepath.Base(dir))
			return nil
		},
	}
	c.Flags().StringVar(&dir, "dir", "", "模块目录（含 docker-compose.yml）")
	c.Flags().BoolVar(&expand, "expand", false, "解压 zip / tar.zip")
	c.Flags().BoolVar(&load, "load", false, "docker load 目录内 .tar")
	c.Flags().BoolVar(&patchEnv, "patch-env", false, "按 site.yaml 更新 .env / compose 内中间件地址")
	c.Flags().BoolVar(&composeUp, "compose-up", false, "docker compose up -d")
	c.Flags().BoolVar(&composeBuild, "compose-build", false, "docker compose up -d --build（市政 / 模型：先 load java8.tar，再从 jar 构建）")
	c.Flags().StringArrayVar(&nacosZips, "nacos-zip", nil, "nacos 模块启动后直接上传导入的 nacos*.zip（可重复，不解压）")
	return c
}

func newModuleNginxCmd() *cobra.Command {
	var (
		dir            string
		gatewayIP      string
		appIP          string
		graphIP        string
		expandHTML     bool
		composeUp      bool
		skipProxyPatch bool
	)
	c := &cobra.Command{
		Use:   "nginx",
		Short: "nginx 模块：load 镜像 / 解压 html / 按 IP 更新 http-web-8877.conf / compose up",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				return fmt.Errorf("请指定 --dir nginx 模块目录")
			}
			site, err := config.LoadSite(flagSitePath)
			if err != nil {
				return err
			}
			gw := gatewayIP
			if !skipProxyPatch && gw == "" && len(site.Nodes) > 0 {
				gw = site.Nodes[0].IP
			}
			layout := moduledeploy.ResolveNginxLayout(dir)
			if base := filepath.Base(dir); base == "conf" || base == "conf.d" {
				layout = moduledeploy.ResolveNginxLayout(filepath.Dir(filepath.Dir(dir)))
			}
			if skipProxyPatch {
				util.Infof("跳过 proxy_pass IP 同步，使用已保存的 conf")
			} else {
				util.Infof("更新 http-web-8877.conf，网关=%s", gw)
			}
			res, err := moduledeploy.RunNginxPatch(moduledeploy.NginxPatchOptions{
				Layout:         layout,
				ExpandArchives: expandHTML,
				SkipProxyPatch: skipProxyPatch,
				ProxyIPs:       moduledeploy.NginxProxyIPs{Gateway: gw, App: appIP, Graph: graphIP},
				ComposeUp:      composeUp,
			})
			if res != nil {
				for _, s := range res.Steps {
					util.Infof("%s", s)
				}
				for _, n := range res.ProxyNotes {
					util.Infof("proxy_pass %s", n)
				}
			}
			if err != nil {
				return err
			}
			util.Successf("Nginx 部署完成")
			return nil
		},
	}
	c.Flags().StringVar(&dir, "dir", "", "nginx 模块目录（.../middleware/nginx）")
	c.Flags().StringVar(&gatewayIP, "gateway-ip", "", "网关 IP（默认 site.yaml 首节点）")
	c.Flags().StringVar(&appIP, "app-ip", "", "应用 IP（opc-ua / msgCenter 等）")
	c.Flags().StringVar(&graphIP, "graph-ip", "", "组态编辑器 IP")
	c.Flags().BoolVar(&expandHTML, "expand-html", false, "解压 tar.zip / load 镜像 / 解压 html zip")
	c.Flags().BoolVar(&composeUp, "compose-up", false, "docker compose up -d")
	c.Flags().BoolVar(&skipProxyPatch, "skip-proxy-patch", false, "不改 conf 内 proxy_pass IP（手工编辑保存时用）")
	return c
}
