package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/precheck"
	"github.com/wpg/wpgctl/internal/util"
)

func newPrecheckCmd() *cobra.Command {
	var (
		manifestPath string
		jsonOut      string
	)
	c := &cobra.Command{
		Use:   "precheck",
		Short: "环境体检（红/黄/绿报告）",
		Long:  "检查硬件、内核、架构、Docker、端口冲突及中间件连通性。红色项阻断部署。",
		RunE: func(cmd *cobra.Command, args []string) error {
			site, err := config.LoadSite(flagSitePath)
			if err != nil {
				return err
			}
			var mf *config.Manifest
			if manifestPath != "" {
				mf, err = config.LoadManifest(manifestPath)
				if err != nil {
					return err
				}
			}
			rep, err := precheck.Run(precheck.Options{Site: site, Manifest: mf})
			if err != nil {
				return err
			}
			rep.Print()
			if jsonOut != "" {
				if err := precheck.WriteReportFile(jsonOut, rep); err != nil {
					return err
				}
				util.Infof("JSON 报告已写入: %s", jsonOut)
			}
			if rep.HasRed {
				return fmt.Errorf("体检未通过")
			}
			return nil
		},
	}
	c.Flags().StringVar(&manifestPath, "manifest", "", "manifest.yaml 路径（用于端口/资源校验）")
	c.Flags().StringVar(&jsonOut, "json", "", "将报告写入 JSON 文件")
	return c
}

func newInitCmd() *cobra.Command {
	var (
		basePkg      string
		manifestPath string
		localOnly    bool
	)
	c := &cobra.Command{
		Use:   "init",
		Short: "环境初始化（幂等）",
		Long:  "安装 Docker 静态二进制、创建目录、防火墙放行、调整内核参数。可重复执行。",
		RunE: func(cmd *cobra.Command, args []string) error {
			site, err := config.LoadSite(flagSitePath)
			if err != nil {
				return err
			}
			var mf *config.Manifest
			if manifestPath != "" {
				mf, err = config.LoadManifest(manifestPath)
				if err != nil {
					return err
				}
			}
			res, err := runInit(site, mf, basePkg, localOnly)
			if err != nil {
				return err
			}
			util.Infof("新建目录 %d 个，放行端口 %v，跳过 %d 项",
				len(res.DirsCreated), res.PortsOpened, len(res.Skipped))
			return nil
		},
	}
	c.Flags().StringVar(&basePkg, "base", "", "base 包解压目录（含 docker-install/）")
	c.Flags().StringVar(&manifestPath, "manifest", "", "manifest.yaml 路径")
	c.Flags().BoolVar(&localOnly, "local", false, "仅本机初始化，不做 SSH 分发")
	return c
}

func newFetchCmd() *cobra.Command {
	var (
		from  string
		local string
		dest  string
	)
	c := &cobra.Command{
		Use:   "fetch [name]",
		Short: "拉包 / 校验 / 解压",
		Long:  "从公司拉包地址断点续传，或 --local 导入离线分卷，完成后 sha256 校验并解压到本地包仓库。",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return runFetch(name, from, local, dest)
		},
	}
	c.Flags().StringVar(&from, "from", "", "拉包基址 URL")
	c.Flags().StringVar(&local, "local", "", "离线分卷/包目录")
	c.Flags().StringVar(&dest, "dest", "", "解压目标目录")
	return c
}

func newDeployCmd() *cobra.Command {
	var (
		packageDir string
		dryRun     bool
		concurrency int
	)
	c := &cobra.Command{
		Use:   "deploy",
		Short: "渲染配置 → 导镜像 → 分层启动 → 健康检查",
		RunE: func(cmd *cobra.Command, args []string) error {
			if packageDir == "" {
				return fmt.Errorf("请指定 --package 包目录")
			}
			return runDeploy(flagSitePath, packageDir, dryRun, concurrency)
		},
	}
	c.Flags().StringVar(&packageDir, "package", "", "release 包目录")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "仅渲染配置，不加载镜像/启动")
	c.Flags().IntVar(&concurrency, "concurrency", 3, "docker load 并发度")
	return c
}

func newRenderCmd() *cobra.Command {
	var packageDir string
	c := &cobra.Command{
		Use:   "render",
		Short: "仅渲染配置（不部署）",
		RunE: func(cmd *cobra.Command, args []string) error {
			if packageDir == "" {
				return fmt.Errorf("请指定 --package")
			}
			return runRender(flagSitePath, packageDir)
		},
	}
	c.Flags().StringVar(&packageDir, "package", "", "含 templates 的包目录")
	return c
}

func newDBCmd() *cobra.Command {
	dbCmd := &cobra.Command{Use: "db", Short: "数据库相关操作"}
	var (
		packageDir string
		dryRun     bool
	)
	apply := &cobra.Command{
		Use:   "apply",
		Short: "执行待执行 SQL 并写入台账",
		RunE: func(cmd *cobra.Command, args []string) error {
			if packageDir == "" {
				return fmt.Errorf("请指定 --package")
			}
			return runDBApply(flagSitePath, packageDir, dryRun)
		},
	}
	apply.Flags().StringVar(&packageDir, "package", "", "含 sql/ 的包目录")
	apply.Flags().BoolVar(&dryRun, "dry-run", false, "仅列出将执行的脚本")
	dbCmd.AddCommand(apply)
	return dbCmd
}

func newNacosCmd() *cobra.Command {
	nacosCmd := &cobra.Command{Use: "nacos", Short: "Nacos 配置操作"}
	var configDir string
	imp := &cobra.Command{
		Use:   "import",
		Short: "导入/更新 Nacos 配置（含备份与 diff）",
		RunE: func(cmd *cobra.Command, args []string) error {
			if configDir == "" {
				return fmt.Errorf("请指定 --config 已渲染的 nacos 目录")
			}
			return runNacosImport(flagSitePath, configDir)
		},
	}
	imp.Flags().StringVar(&configDir, "config", "", "渲染后的 nacos 配置目录")
	nacosCmd.AddCommand(imp)
	return nacosCmd
}

func newUpgradeCmd() *cobra.Command {
	var (
		yes bool
	)
	c := &cobra.Command{
		Use:   "upgrade [patch-dir]",
		Short: "补丁升级（版本化 jar + 台账 + 失败自动回滚）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(flagSitePath, args[0], yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "跳过确认")
	return c
}

func newRollbackCmd() *cobra.Command {
	var to string
	c := &cobra.Command{
		Use:   "rollback",
		Short: "回滚到历史版本",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRollback(flagSitePath, to)
		},
	}
	c.Flags().StringVar(&to, "to", "", "目标版本（默认上一成功版本）")
	return c
}

func newStatusCmd() *cobra.Command {
	var composeDir string
	c := &cobra.Command{
		Use:   "status",
		Short: "查看服务运行状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			site, err := config.LoadSite(flagSitePath)
			if err != nil {
				util.Warnf("加载 site.yaml 失败，继续仅展示 compose 状态: %v", err)
			}
			if composeDir == "" && site != nil {
				composeDir = filepath.Join(site.Paths.Workspace, "rendered")
			}
			return runStatus(site, composeDir)
		},
	}
	c.Flags().StringVar(&composeDir, "compose-dir", "", "compose 文件目录")
	return c
}

func newLogsCmd() *cobra.Command {
	var (
		tail   int
		follow bool
	)
	c := &cobra.Command{
		Use:   "logs <service>",
		Short: "查看服务日志",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(args[0], tail, follow)
		},
	}
	c.Flags().IntVar(&tail, "tail", 200, "输出末尾行数")
	c.Flags().BoolVarP(&follow, "follow", "f", false, "持续跟踪")
	return c
}

func newDiagCmd() *cobra.Command {
	var (
		composeDir string
		renderDir  string
		output     string
		manifest   string
	)
	c := &cobra.Command{
		Use:   "diag",
		Short: "打包诊断材料（发回公司远程分析）",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiag(flagSitePath, manifest, composeDir, renderDir, output)
		},
	}
	c.Flags().StringVar(&composeDir, "compose-dir", "", "compose 目录")
	c.Flags().StringVar(&renderDir, "render-dir", "", "渲染配置目录")
	c.Flags().StringVar(&output, "output", "", "输出 tar.gz 路径")
	c.Flags().StringVar(&manifest, "manifest", "", "manifest.yaml")
	return c
}

func newPackCmd() *cobra.Command {
	var (
		kind         string
		version      string
		manifest     string
		output       string
		services     []string
		baseRelease  string
		workDir      string
		volumeSizeGB int
	)
	c := &cobra.Command{
		Use:   "pack",
		Short: "公司侧打包（base|release|patch）",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && kind == "" {
				kind = args[0]
			}
			if kind == "" {
				return fmt.Errorf("请指定 kind：base|release|patch")
			}
			return runPack(kind, version, manifest, output, services, baseRelease, workDir, volumeSizeGB)
		},
	}
	c.Flags().StringVar(&kind, "kind", "", "base|release|patch")
	c.Flags().StringVar(&version, "version", "", "版本号")
	c.Flags().StringVar(&manifest, "manifest", "", "manifest.yaml 路径")
	c.Flags().StringVar(&output, "output", "./dist", "输出目录")
	c.Flags().StringSliceVar(&services, "services", nil, "patch 涉及服务")
	c.Flags().StringVar(&baseRelease, "base-release", "", "patch 基线 release 版本")
	c.Flags().StringVar(&workDir, "work-dir", "", "待打包内容目录")
	c.Flags().IntVar(&volumeSizeGB, "volume-size-gb", 2, "分卷大小（GB）")
	return c
}
