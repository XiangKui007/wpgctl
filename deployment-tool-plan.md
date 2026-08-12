# 水厂交付部署工具（wpgctl）实施方案

> 版本：v1.1（2026-08-11，补充实包分析结论，见附录 A）
> 状态：方案评审稿
> 目标读者：研发、实施/运维负责人

---

## 1. 背景与目标

### 1.1 现状痛点

当前水厂定制化项目交付流程（Spring Cloud 体系，20~40 个后端容器 + 前端静态包 + 数据库/中间件/监控）：

1. FinalShell / 企业微信中转传输约 20G 的 zip 整包，耗时数小时且易中断；
2. 手工安装 Docker、JDK，手工放行防火墙端口；
3. 手工 `docker load` 逐个镜像 tar 包；
4. 手工修改几十个服务目录下的 `.env` / `docker-compose.yml`（IP、端口、密码），耗时半天且极易漏改错改；
5. 手工按顺序 `docker-compose up`，启动后靠人肉验证；
6. Nacos 配置手工导入，智慧水厂 SQL 手工执行，无台账；
7. 后续升级补丁（通常仅 1~3 个服务）已经是"替换服务目录下的 jar + `docker-compose up -d --build`"，传输量不大，但**没有标准发包流程**：jar 无版本号、镜像 tag 不变、没有变更记录，回滚只能靠手工找旧 jar，出了问题说不清现场跑的到底是哪个版本。

单次交付耗时约 1 个工作日，且只有熟悉系统的开发能操作。

### 1.2 约束条件（来自需求调研）

| 维度 | 结论 |
|------|------|
| 拓扑 | 每站 2~5 台服务器，按角色分工（数据库 / 中间件 / 业务） |
| 网络 | 部分现场有 VPN 可直连；部分只能向日葵/ToDesk 远程桌面，传包靠企业微信中转（说明出站公网可用，无入站通道） |
| 操作系统 | 不可控，客户提供什么用什么（CentOS / 国产系统 / 未知均可能，含 ARM 可能性） |
| 频率 | 一年 5+ 个新项目；升级频繁，通常只动 1~3 个服务 |
| 操作者 | 现在是开发，目标下放给实施/运维人员 |
| 站点差异 | 主要是 IP/端口/密码 + 服务组合（部分项目只部署部分模块） |
| 数据库 | 平台服务 Flyway 自动建表；智慧水厂需手工 SQL；Nacos 需导入配置 |
| 前端 | 静态包放入 nginx 容器挂载的 html 目录 |
| 公司侧 | 已有拉包地址（HTTP 可下载） |

### 1.3 目标

| 指标 | 现状 | 目标 |
|------|------|------|
| 新项目首次部署 | ~1 天 | ≤ 3 小时（含传包） |
| 日常升级（1~3 服务） | 手工替换 jar + rebuild，无版本无回滚 | ≤ 30 分钟，版本化 + 台账 + 一键回滚 |
| 配置修改点 | 几十个 env/yaml 文件 | 1 个 `site.yaml` |
| 操作者门槛 | 熟悉系统的开发 | 实施人员按向导操作 |
| 部署验证 | 人肉逐个看 | 自动健康检查 + 冒烟报告 |

### 1.4 非目标（明确不做）

- 不上完整 Kubernetes / K3s（现场机器少、离线、低频变更，Compose + 自动化更务实；未来服务规模翻倍再评估）；
- 不做图形化拖拽编排设计器（服务拓扑固定）；
- 第一期不做总部侧多站点集中监控看板（列入远期）。

---

## 2. 总体架构

```
┌─────────────────── 公司侧 ───────────────────┐
│  Jenkins CI ──构建镜像──► 打包流水线(wpgctl pack) │
│                              │                 │
│                              ▼                 │
│                     拉包服务器(已有,HTTP)        │
└──────────────────────┬───────────────────────┘
                       │ HTTPS 断点续传 (wpgctl fetch)
                       │ 或 企业微信/硬盘 兜底
┌──────────────────────▼─── 现场侧 ─────────────┐
│  wpgctl (Go 单二进制, 放在主控机)               │
│    ├─ precheck   环境体检                      │
│    ├─ init       装docker/建目录/开端口          │
│    ├─ deploy     渲染配置→导镜像→导Nacos→分层启动 │
│    ├─ db apply   SQL 执行 + 台账               │
│    ├─ upgrade    补丁升级 / rollback 回滚       │
│    ├─ status/logs/diag  运维辅助               │
│    └─ ui         本地 Web 控制台(二期)          │
│                                               │
│  多机场景: 主控机通过 SSH 分发执行到其他节点       │
└───────────────────────────────────────────────┘
```

核心思想：

1. **传输问题用"分层包 + 现场主动拉取"解决**，而不是优化推送；
2. **配置问题用"单一配置源 site.yaml + 模板渲染"解决**，现场永不手改 env/compose；
3. **环境与启动问题用"幂等的单二进制 CLI"解决**，对宿主机零依赖；
4. **升级问题用"版本化补丁包 + 台账 + 回滚"解决**：沿用现有"传 jar、现场 build"的轻量做法，但把它变成有版本号、有记录、可一键回滚的标准流程。

---

## 3. 技术栈选型及理由

### 3.1 CLI 主体：Go 1.22+

**选 Go 的原因（这是全方案最重要的选型）：**

1. **静态单二进制，零运行时依赖**。现场 OS 不可控是硬约束——不能假设有 Python（版本更不可控）、bash 4+、甚至 glibc 版本。Go 交叉编译产出 `wpgctl-linux-amd64` 和 `wpgctl-linux-arm64` 两个文件，扔到任何 Linux 上直接跑，覆盖国产系统（麒麟/UOS/openEuler 本质是 Linux 内核 + 不同用户态）。
2. **并发原语适合本场景**：并行 `docker load` 多个镜像、并行健康检查、并行 SSH 分发，goroutine 写起来简单可靠。
3. **生态齐全**：SSH（golang.org/x/crypto/ssh）、MySQL 驱动、YAML、模板引擎、HTTP 断点续传全部标准库或成熟库。
4. **对比备选**：
   - Shell 脚本：依赖 bash 版本和 GNU 工具链，错误处理脆弱，无法做断点续传/并发/台账这类逻辑，排除；
   - Python：现场无解释器，PyInstaller 打包对 glibc 有版本要求且体积大、在国产系统上坑多，排除；
   - Java：需要 JRE，本末倒置，排除；
   - Rust：能力等价但团队是 Java 背景，Go 语法更接近、上手成本低得多，选 Go。

### 3.2 依赖库清单

| 用途 | 选型 | 理由 |
|------|------|------|
| CLI 框架 | `spf13/cobra` | 事实标准，子命令/flag/帮助文档生成齐全 |
| YAML 解析 | `gopkg.in/yaml.v3` | 标准选择，支持严格模式（未知字段报错，防手误） |
| 配置渲染 | Go 标准库 `text/template` + `Masterminds/sprig` | 无需引入 Jinja2 类外部依赖；sprig 补充 default/required 等实用函数 |
| SSH 分发 | `golang.org/x/crypto/ssh` + `pkg/sftp` | 多机场景主控机免装 ansible，直接内置 SSH 执行与文件分发 |
| MySQL | `go-sql-driver/mysql` | 执行智慧水厂 SQL、写台账表 |
| HTTP 下载 | 标准库 `net/http` + Range 头 | 断点续传自己实现（约 100 行），不引重型库 |
| 终端交互/进度 | `charmbracelet/bubbletea` 或简化用 `schollz/progressbar` | 传包/load 镜像的进度可视化；第一期用 progressbar 即可 |
| 本地状态存储 | JSON 文件（`~/.wpgctl/state/`） | 部署历史、包清单等数据量小，不引入 sqlite/bbolt，保持可手工查看修复 |

### 3.3 容器操作：调用 docker CLI，而非 Docker SDK

**决策：所有容器操作通过 exec `docker` / `docker compose` 命令 + `--format json` 解析输出。**

理由：

1. Compose v2 的行为（profiles、depends_on condition、变量插值）非常复杂，SDK 里没有等价物，用 CLI 保证行为与人工操作**完全一致**，现场排障时人可以直接复制同样的命令验证；
2. Docker SDK 需处理 API version 协商，面对现场新旧不一的 Docker 版本是额外的坑；
3. `docker inspect --format` / `docker compose ps --format json` 已提供结构化输出，解析足够。

### 3.4 编排：Docker Compose v2 + profiles

- 保留 Compose 而不迁移 K8s：现场 2~5 台机器、离线、变更低频，Compose 心智负担最小，且与现有 `scripts/docker-compose/` 资产延续；
- **服务组合差异用 Compose profiles 实现**：每个服务标注所属 profile（如 `platform`、`smartwater`、`monitor`），`site.yaml` 中声明启用的 profile 列表，deploy 时传给 compose；
- **启动分层不依赖 depends_on 跨文件能力**，由 wpgctl 按层顺序 up 并等健康（见 §6.7），比 compose 原生依赖更可控、日志更清晰。

### 3.5 Docker 离线安装：官方静态二进制

现场 OS 不可控，yum/apt 源不可用是常态。base 包内置 [Docker 官方 static binaries](https://download.docker.com/linux/static/stable/)（amd64 + arm64）+ compose plugin 二进制 + systemd unit 模板，`wpgctl init` 检测无 Docker 时直接解压安装、注册 systemd 服务。这比适配各发行版包管理器可靠得多。

JDK 不再装宿主机——所有 Java 服务已容器化，基础镜像内含 JDK。

### 3.6 包格式：tar.gz 分卷 + manifest + sha256

- 用 tar 而非 zip：支持流式解压（边下边解可做优化）、Linux 原生、分卷简单（`split`）；
- 每个包必带 `manifest.yaml`（内容清单、镜像列表、依赖的 base 版本、适用架构）和 `sha256sums.txt`；
- 完整性校验强制执行，杜绝"传了一半的包解出来一堆诡异错误"；
- 二期可加 minisign 签名防篡改（离线场景优先级不高）。

### 3.7 Web 控制台（二期）：Go embed + Vue 3

- `wpgctl ui` 在本机起 HTTP 服务（默认 `127.0.0.1:9527`），前端产物用 Go 1.16+ `embed` 打进二进制，**仍然是单文件交付**；
- 适配"只有向日葵/ToDesk"的现场：远程桌面上去后开浏览器即用，无需公司侧部署任何东西；
- 前端 Vue 3 + Vite + Element Plus（团队若更熟 React 可换，功能就是表单 + 状态展示 + 日志流，任何栈皆可）；
- 后端就是 wpgctl 内部函数的 HTTP 封装 + WebSocket 推日志，不新增架构成分。

### 3.8 公司侧打包：复用 Jenkins + `wpgctl pack` 子命令

打包逻辑做进同一个二进制（`wpgctl pack`），Jenkins 只负责调度：构建镜像 → `wpgctl pack release --version 4.0.2` → 产物上传拉包服务器。打包与部署共用 manifest/渲染代码，避免两套逻辑漂移。

---

## 4. 交付包体系规范

### 4.1 三层包设计

| 包 | 内容 | 典型大小 | 更新频率 | 传输方式 |
|----|------|---------|---------|---------|
| **base** 基础包 | 中间件镜像（MySQL/Redis/Kafka/Nacos/nginx/监控组件）、JDK 运行时基础镜像、Docker 离线安装二进制 | ~15G | 一年 1~2 次 | 每站仅首次传一次（硬盘/整夜拉取均可接受） |
| **release** 版本包 | 全量业务服务镜像、compose 编排、配置模板、Nacos 配置导出、全量 SQL、前端 dist | 3~5G | 新项目交付 / 大版本 | fetch 拉取或企业微信 |
| **patch** 补丁包 | 指定服务的**版本化 jar** + 前端 dist 增量 + 增量 SQL + Nacos 配置变更 | 50~300MB | 日常升级（高频） | fetch 拉取，分钟级 |

关键收益：base 与业务解耦后，首次交付不再搬运 20G 整包；补丁包把现有"传 jar 现场 build"的做法标准化为可管理、可回滚的流程。

**补丁包为什么装 jar 而不是镜像 tar：**

1. 与现行做法一致——现场本来就是替换 jar 后 `docker-compose up -d --build`，Dockerfile 只是把 jar COPY 进 JDK 基础镜像，现场构建快且确定性足够；
2. `docker save` 导出的镜像 tar 会携带完整基础层，单服务几百 MB 起步；jar 通常只有几十到一二百 MB，传输更小；
3. 现状真正缺的不是"换传输格式"，而是**版本管理**：wpgctl 在现场 build 时会打上**版本化的镜像 tag**（如 `waterwork-center:4.0.3`，而不是一直沿用同一个 tag），旧版本镜像和 jar 都归档保留——这正是回滚能做到"秒级切 tag"的前提（见 §6.10）。

### 4.2 包目录结构

```text
wpg-base-1.0.0/
  manifest.yaml
  sha256sums.txt
  docker-install/
    amd64/  (dockerd, docker, containerd..., docker-compose plugin)
    arm64/
    systemd/ (docker.service, docker.socket, containerd.service)
  images/
    mysql-8.0.tar
    redis-7.tar
    kafka-3.x.tar
    nacos-2.x.tar
    nginx-1.25.tar
    openjdk-base.tar
    prometheus.tar / grafana.tar ...

wpg-release-4.0.2/
  manifest.yaml            # 版本、依赖 base 版本、服务清单(含 profile 归属/端口/健康检查路径)
  sha256sums.txt
  images/
    waterwork-center-4.0.2.tar
    waterwork-device-4.0.2.tar
    ...(20~40 个)
  compose/
    templates/             # 带 {{ }} 占位符的 compose 模板，按层组织
      layer1-middleware.yml.tmpl
      layer2-platform.yml.tmpl
      layer3-business.yml.tmpl
      layer4-monitor.yml.tmpl
  config/
    env.tmpl               # 全局 .env 模板
    nacos/                 # 按 profile 分目录的 nacos 配置模板(yaml, 带占位符)
      platform/
      smartwater/
  sql/
    smartwater/            # 智慧水厂手工 SQL, 命名 V4.0.2_001__desc.sql
  frontend/
    dist-web.tar.gz
    dist-screen.tar.gz
  site.example.yaml
```

**包内容红线（来自现有包的教训，见附录 A）：运行时数据目录一律不进包。**
现有交付包里混入了 `nacos/data/`（raft 日志、derby 数据）、`pgsql/data/`（完整数据目录）等运行时产物，
既是 20G 体积的重要来源，也会把上一个环境的状态带到新站点。规范：交付包只带 conf、模板和 init 脚本，
数据目录由 `wpgctl init` 在现场创建；`wpgctl pack` 打包时对 `data/`、`logs/` 类目录强制排除并告警。

```text

wpg-patch-4.0.3/
  manifest.yaml            # 基于哪个 release、涉及哪些服务、jar 对应的镜像 tag
  sha256sums.txt
  jars/
    waterwork-center-4.0.3.jar     # 文件名强制带版本号
  sql/smartwater/V4.0.3_001__add_column.sql
  frontend/dist-web.tar.gz          # 可选
  nacos/                            # 可选，仅变更的配置
```

### 4.3 manifest.yaml 示例（release）

```yaml
kind: release
version: 4.0.2
requiresBase: ">=1.0.0"
arch: [amd64]
services:
  - name: waterwork-center
    image: waterwork-center:4.0.2
    profile: platform
    layer: 3
    port: 8401
    health: { type: http, path: /actuator/health, timeoutSec: 120 }
  - name: waterwork-device
    image: waterwork-device:4.0.2
    profile: smartwater
    layer: 3
    port: 8402
    health: { type: http, path: /actuator/health, timeoutSec: 120 }
  - name: mongodb
    image: mongo:4.4.11
    profile: gis
    layer: 1
    port: 27017
    composeFlags: ["--compatibility"]      # 现有包中靠 yaml 注释提醒"必须用 --compatibility 启动"
    requiredFiles: ["init-mongo.js"]       # 启动前必须就位的文件，deploy 时自动校验
    health: { type: tcp }
  # ...
frontend:
  - name: web
    archive: frontend/dist-web.tar.gz
    nginxHtmlSubdir: web
sql:
  - dir: sql/smartwater
    database: smartwater
```

manifest 是工具所有行为的驱动数据：deploy 按它导镜像、分层、健康检查；upgrade 按它算差异；status 按它列服务。**新增服务只改 manifest 和模板，工具代码不动。**

---

## 5. site.yaml：现场唯一配置源

```yaml
# ===== 站点基本信息 =====
site:
  name: xx水厂
  code: plant-xx

# ===== 机器清单（多机场景）=====
nodes:
  - name: db-node
    ip: 10.10.104.23
    ssh: { user: root, port: 22 }        # 密码运行时交互输入或用密钥
    roles: [database]
  - name: app-node
    ip: 10.10.104.22
    ssh: { user: root, port: 22 }
    roles: [middleware, platform, smartwater, monitor]

# ===== 启用的业务模块（映射 compose profiles）=====
profiles: [platform, smartwater, monitor]

# ===== 中间件连接参数（渲染进所有 env/nacos 配置）=====
middleware:
  nacos: { host: 10.10.104.22, port: 8848, namespace: plant-xx,
           username: nacos, password: "***" }
  mysql: { host: 10.10.104.23, port: 3306, user: wpg, password: "***" }
  redis: { host: 10.10.104.23, port: 6377, password: "***" }
  kafka: { host: 10.10.104.22, port: 9092 }

# ===== 站点级覆盖（可选，缺省用 manifest 默认值）=====
overrides:
  waterwork-center: { xmx: 2048M }

# ===== 路径规划 =====
paths:
  workspace: /workspace/waterwork
  logs: /workspace/waterwork-logs
  nginxHtml: /workspace/nginx/html
```

设计要点：

1. **严格校验**：yaml.v3 严格模式 + 自定义规则（IP 格式、端口范围、密码非空、profile 名合法），错一个字段直接报错并指出行号，把"手误"消灭在部署前；
2. **敏感信息**：第一期密码明文放 site.yaml（现场内网文件），二期支持 `!vault` 标记 + AES 加密（`wpgctl encrypt` 生成）；
3. **版本化**：每站的 site.yaml 收入公司 git 仓库（`sites/plant-xx/site.yaml`），部署记录里存其 hash，支持事后 diff 追溯。

### 5.1 与历史上"全局 env"方案的区别（为什么这次能行）

团队曾尝试过让所有服务共用一份全局 env，后因问题拆回了每服务一份。失败原因是结构性的：

1. **扁平命名空间放不下"同名不同值"**——`NACOS_GROUP` 在 public 是 `PLATFORM_COMMON`、在 device 是 `DEVICE_MANAGEMENT`、在 gis 是 `DEFAULT_GROUP`；`JVM_OPTS` 每服务不同；一份扁平 KV 文件只能存一个值；
2. **运行时共享导致改动无边界**——改一个值影响所有引用方，不敢单服务重启；
3. **docker-compose 只认同目录 `.env`**，全局文件靠复制/软链分发，随后必然漂移（现有包中 9 份 env 已出现 `10.10.10.10` 与 `192.168.200.66` 并存）。

site.yaml 的本质区别：**共享发生在编辑时，不在运行时**。site.yaml 是结构化的"源数据"，经 `wpgctl render` 一次性生成每个服务目录下各自独立的 `.env` 和 compose 文件——运行时布局与现在拆分后的形态完全一致，保留全部隔离性；但人只编辑 site.yaml 一处，全局值（如 Redis 密码）只存在一份，服务级差异（group、schema、xmx）放在各自名下的层级里互不冲突。类比：site.yaml 是源代码，各服务的 env/compose 是编译产物，产物永不手改。

---

## 6. CLI 功能实施细节

### 6.1 命令总览

```bash
wpgctl precheck  --site site.yaml            # 环境体检
wpgctl init      --site site.yaml            # 环境初始化（幂等）
wpgctl fetch     release-4.0.2 [--from URL]  # 拉包/校验/解压
wpgctl deploy    --site site.yaml --package wpg-release-4.0.2
wpgctl db apply  --site site.yaml            # 执行待执行 SQL
wpgctl nacos import --site site.yaml         # 导入/更新 Nacos 配置
wpgctl upgrade   wpg-patch-4.0.3 --site site.yaml
wpgctl rollback  [--to 4.0.2]
wpgctl status / logs <svc> / diag
wpgctl pack      base|release|patch ...      # 公司侧打包
wpgctl ui        [--listen 127.0.0.1:9527]   # 二期
```

### 6.2 precheck：环境体检

逐项检查并输出红/黄/绿报告（同时写 JSON 供 UI 用）：

- 硬件：CPU 核数、内存、磁盘可用空间（对比 manifest 声明的最低需求）；
- 系统：内核版本 ≥3.10、architecture、SELinux 状态、防火墙类型（firewalld/iptables/ufw/none）、时钟偏差（对比主控机）、hostname 解析；
- 端口：按 manifest + profiles 计算将占用的端口列表，检测冲突。**注意：存量服务绝大多数用 `network_mode: host`，compose 里的 `ports` 配置实际被 Docker 忽略**，真实端口由 `SERVER_PORT` 等应用配置决定，因此端口冲突检测和防火墙放行必须以 manifest 的端口清单为准，不能解析 compose 的 ports 字段；
- Docker：已装则检查版本 ≥20.10、storage driver、data-root 所在盘空间；
- 网络：节点间互 ping、到 MySQL/Redis 端口连通性（升级场景）。

**红色项阻断部署，黄色项警告放行。** 该命令预计消灭 50% 以上的现场"部署失败"（多数其实是环境问题）。

### 6.3 init：环境初始化（幂等）

1. 无 Docker → 从 base 包解压静态二进制到 `/usr/local/bin`，装 systemd unit，配置 `daemon.json`（data-root 指向大盘、日志轮转 `max-size=100m,max-file=3`）；
2. 建目录树（workspace/logs/nginxHtml，来自 site.yaml paths）；
3. 防火墙放行：探测 firewalld/iptables，按 manifest 端口清单放行（幂等，已放行跳过）；
4. 内核参数：`vm.max_map_count`（如未来引入 ES）、`net.core.somaxconn` 等按需写 sysctl.d；
5. 多机场景：主控机对每个 node 依次 SSH 执行上述步骤（wpgctl 先把自身二进制 sftp 到目标机再远程调用 `wpgctl init --local`，避免逐条 SSH 命令的脆弱性）。

所有步骤可重复执行，检测已完成则跳过——**幂等是硬要求**，现场断电重来不能留下半吊子状态。

### 6.4 fetch：拉包

- `wpgctl fetch release-4.0.2`：从配置的公司拉包地址下载分卷，HTTP Range 断点续传，失败自动重试（指数退避），下载中显示速度/进度/ETA；
- 下载完成 → 合并分卷 → 逐文件 sha256 校验 → 解压到本地包仓库 `~/.wpgctl/packages/`；
- 完全离线现场：实施人员把企业微信/硬盘拿到的分卷放进指定目录，`wpgctl fetch --local ./downloads/` 走同样的校验合并流程；
- 校验失败精确报告哪个分卷坏了，只需重传该分卷。

### 6.5 配置渲染

- 输入：release 包的 `compose/templates/*.tmpl`、`config/env.tmpl`、`config/nacos/**` + site.yaml；
- 输出：写到 `{workspace}/rendered/<version>/` 下的最终 compose 文件、.env、nacos 配置；
- 渲染引擎 text/template + sprig，模板中用 `{{ required "mysql host 必填" .middleware.mysql.host }}` 强制必填项；
- **渲染结果落盘并保留**，人可以直接查看最终生效的文件（排障关键）；每次部署的渲染快照按版本归档，支持 diff；
- 未启用的 profile 对应的服务不渲染、不导镜像。

**渲染范围必须覆盖非 env 配置（实包分析发现的隐蔽手改点，见附录 A）：**

| 文件 | 每站要改的内容 |
|------|---------------|
| `nginx/conf.d/*.conf` | proxy_pass 硬编码的网关/组态编辑器/消息中心等后端 IP；location 块按 profile 开关（不部署 GIS 就不渲染 gis 相关 location） |
| kafka compose | `KAFKA_ADVERTISED_LISTENERS` 需填本机物理 IP（来自 site.yaml 节点的 `host_ip`） |
| `redis/conf/redis.conf` | requirepass 密码在 conf 文件里而非 compose |
| prometheus `config/targets/*.yml` | 采集目标 IP（文件名本身都带 IP） |
| monitor compose | 云端监控地址、`IS_CLOUD`、认证密码等目前硬编码在 compose 内的值 |

**变量别名映射**：现存模板变量命名不统一（平台服务 `REDIS_*`/`DATASOURCE_*`，GIS 服务 `WPG_REDIS_*`/`WPG_PGSQL_*`/`WPG_MONGODB_*`）。site.yaml 只存一份真值，别名翻译放在各服务模板里完成（如 GIS 模板写 `WPG_REDIS_PSWD={{ .middleware.redis.password }}`），不要求统一改造存量服务的变量名——模板化迁移零代码改动。

### 6.6 镜像加载

- 按 manifest 中当前 profiles 需要的镜像列表，并行 `docker load`（并发度默认 3，可调）；
- load 前先 `docker image inspect` 检查是否已存在同 tag，存在则跳过（幂等 + 提速）；
- 多机场景按 node roles 分发：数据库镜像只发 db-node，业务镜像只发 app-node，减少无谓传输。

### 6.7 分层启动与健康检查

启动分四层，每层内并行、层间串行，**上一层全部健康才启动下一层**：

| 层 | 内容 | 健康判定 |
|----|------|---------|
| L1 | MySQL、Redis、Kafka、Nacos | 端口可连 + 协议级探测（mysql ping / redis PING / nacos health API） |
| L2 | 网关、注册到 Nacos 的平台基础服务 | actuator/health + Nacos 注册状态 |
| L3 | 业务服务（智慧水厂等，按 profile） | actuator/health |
| L4 | nginx（前端）、监控（Prometheus/Grafana） | HTTP 200 |

对照现有交付包（附录 A）的实际映射：L1 = middleware-new 的 mysql/pgsql/postgis/mongodb/redis/kafka+zookeeper/nacos/minio/influxdb/emqx；L2 = waterjob（xxl-job，虽在中间件包里但依赖 nacos+pgsql）+ public 模块（网关/auth/usercenter/msg/log）；L3 = device、alarm、graph、gis、report-center、out-work 各模块；L4 = nginx、monitor、prometheus、node-exporter。

- 每个服务的健康检查方式/超时来自 manifest，默认 HTTP `/actuator/health`、120s 超时、5s 间隔轮询；
- 某服务超时 → 自动抓取该容器最后 200 行日志附在错误报告里，其余同层服务继续等待，最终输出整体成败矩阵；
- L1 起来后、L2 之前自动执行：`nacos import`（见 6.9）+ Flyway 由各服务自带（无需干预）+ 提示是否执行 `db apply`；
- 全部完成后输出冒烟报告：每个服务 名称/版本/端口/健康状态/耗时，保存为部署记录。

### 6.8 db apply：SQL 执行与台账

- 扫描包内 `sql/<db>/` 目录，文件命名强制 `V{version}_{seq}__{desc}.sql`（仿 Flyway）；
- 目标库中自动建 `wpg_deploy_history` 表，记录已执行脚本的文件名、checksum、执行时间、耗时、结果；
- 执行逻辑：按版本+序号排序，跳过已执行且 checksum 一致的；**已执行但 checksum 变了 → 报错阻断**（防止有人改了历史脚本）；
- 每个脚本在事务中执行（DDL 在 MySQL 不支持事务回滚，故执行前自动 `mysqldump` 涉及表结构做轻量备份，失败时给出恢复指引）；
- `--dry-run` 模式列出将执行的脚本清单供确认。

### 6.9 nacos import：配置导入

- 通过 Nacos OpenAPI（`/nacos/v1/console/namespaces`、`/nacos/v1/cs/configs`）：不存在命名空间则创建（namespace 名来自 site.yaml）；
- 包内 `config/nacos/<profile>/` 下的配置模板经渲染后逐条发布；
- 发布前先 GET 现有配置比对：内容一致跳过；不一致则**备份旧值到本地**（`rendered/<version>/nacos-backup/`）再覆盖，并在报告中列出 diff 摘要；
- 只处理启用 profile 的配置。

### 6.10 upgrade / rollback：补丁升级与回滚

upgrade 流程（最高频场景，重点打磨）。本质是把现有"替换 jar + `docker-compose up -d --build`"标准化，关键差别只有一个：**镜像 tag 带版本号，而不是反复覆盖同一个 tag**，旧版本因此天然保留，回滚变成秒级切 tag：

1. 校验 patch 包 manifest 的基线版本与本地部署记录匹配（防止跳版本、装错站）；
2. 展示变更预览：涉及服务、版本 diff、SQL 清单、Nacos 配置 diff，需人工确认（`--yes` 跳过）；
3. 备份当前状态：涉及服务的当前 jar 归档到 `bak/<旧版本>/`，当前镜像 tag、compose、env 记入回滚点；
4. 替换 jar 并现场构建：新 jar 放入服务目录 → `docker build -t waterwork-center:4.0.3 .`（等价于现有 `up -d --build`，但产出版本化 tag）；
5. 执行增量 SQL（db apply，带台账）→ 更新 Nacos 配置 → 前端 dist 解压替换（旧目录 mv 为 `.bak-<ts>`）；
6. 更新渲染后的 compose 中涉及服务的 image tag → 逐个 `docker compose up -d <svc>` 重建容器；
7. 健康检查涉及服务，全部通过 → 记录升级成功；
8. **任一服务健康检查失败 → 自动回滚**：compose 指回旧 tag 重新 up（旧镜像还在本地，无需重新 build，秒级完成）、恢复前端目录、Nacos 配置恢复备份值（SQL 不自动回滚，报告中明确提示需人工评估）。

rollback 命令支持手动回到任意有记录的历史版本（旧版本镜像与 jar 保留最近 N 个版本，默认 2，超出自动清理防止磁盘涨满）。

### 6.11 status / logs / diag

- `status`：表格输出全部服务的 运行状态/健康/版本/端口/内存占用/重启次数（数据来自 `docker compose ps --format json` + inspect），多机自动汇总；
- `logs <svc> [-f] [--tail 200]`：免去实施人员记容器名；
- `diag`：一键打包诊断材料（所有容器状态、最近日志、渲染后的配置<敏感字段打码>、部署历史、precheck 报告、磁盘/内存快照）为一个 tar.gz，现场出问题时发回公司远程分析——适配"公司连不进现场"的网络现实。

### 6.12 部署记录（本地状态）

`~/.wpgctl/state/deployments.json`：每次 deploy/upgrade/rollback 记录时间、操作者、包版本、site.yaml hash、结果、耗时。它是 upgrade 基线校验和 rollback 的数据来源，也是"这个站现在到底是什么版本"的唯一真相。

---

## 7. 公司侧配套

### 7.1 打包流水线

1. Jenkins 现有 job 构建各服务镜像（延续现状）；
2. 新增 pack job：`wpgctl pack release --version 4.0.2 --manifest manifest.yaml`——docker save 镜像、组装目录、生成 sha256、tar 分卷（默认 2G/卷，适配企业微信兜底传输）；
3. `wpgctl pack patch --version 4.0.3 --services waterwork-center,waterwork-device --base-release 4.0.2`——从 CI 构建产物收集对应服务的 jar（重命名为带版本号）+ 增量 SQL + Nacos 变更打成补丁包；
4. 产物自动上传到已有拉包服务器，目录约定 `/packages/{base|release|patch}/<version>/`，并维护一个 `index.json` 供 `wpgctl fetch --list` 查询可用版本。

### 7.2 配置模板与站点库

- 仓库新增 `delivery/` 目录：compose 模板、env 模板、nacos 配置模板、site.example.yaml，随代码一起走版本控制和 code review；
- 新增 `sites/` 私有仓库（或本仓库私有目录）：每站一份 site.yaml，部署前拉取，形成站点配置审计链。

---

## 8. 实施计划与里程碑

### 阶段一：核心链路 MVP（第 1~2 周，1 人力）

| 任务 | 说明 | 工期 |
|------|------|------|
| 项目脚手架 | Go module、cobra 骨架、日志、交叉编译 CI | 1d |
| 包规范 + manifest 定义 | 含 site.yaml schema 与严格校验 | 1d |
| precheck | 全部检查项 + 报告输出 | 1.5d |
| init | docker 离线安装、目录、防火墙（先单机） | 1.5d |
| 配置渲染 | 模板引擎接入 + 现有 waterwork-center/device 模板化改造 | 1.5d |
| 镜像加载 + 分层启动 + 健康检查 | deploy 命令主体 | 2d |
| status / logs | 基础运维命令 | 0.5d |
| 内部演练 | 用测试服务器全流程走一遍 | 1d |

**验收标准**：在一台干净的测试机上，从裸机到全部服务健康，只执行 `precheck → init → deploy` 三条命令 + 一份 site.yaml，全程无手工改文件。

### 阶段二：传输与升级（第 3~4 周）

| 任务 | 工期 |
|------|------|
| fetch 断点续传 + 分卷校验 + `--local` 离线模式 | 2d |
| `wpgctl pack`（base/release/patch）+ Jenkins 集成 | 2d |
| db apply 台账 | 1.5d |
| nacos import（含 diff/备份） | 1.5d |
| upgrade / rollback | 2.5d |
| SSH 多机分发（init/deploy/status 跨节点） | 2d |

**验收标准**：模拟一次 1~3 服务的补丁升级，从拿到补丁包到升级完成 ≤30 分钟；人为制造一个启动失败，验证自动回滚。

### 阶段三：试点与 Web 化（第 5~7 周）

| 任务 | 工期 |
|------|------|
| **真实项目试点**（选最近一个交付项目全程使用，记录耗时与问题） | 跟随项目 |
| diag 诊断包 | 1d |
| `wpgctl ui`：向导式部署页 + 状态页 + 日志流（Vue3 embed） | 5d |
| 文档：实施人员操作手册（截图版） | 1d |
| 密码加密（vault 标记） | 1d |

**验收标准**：一名未参与开发的实施人员，仅凭操作手册 + UI 完成一次完整部署。

### 阶段四：远期备选（不排期）

- 总部侧多站点心跳看板（现场允许出站上报时）；
- license/授权管理集成；
- 包签名（minisign）；
- 服务规模显著增长后评估 K3s + Helm 迁移。

---

## 9. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| 现场 OS 过老（内核 <3.10）跑不了新 Docker | 阻断 | precheck 提前暴露；base 包备一个兼容旧内核的 Docker 版本 |
| 国产 ARM 机器 | 镜像架构不匹配 | manifest 声明 arch，precheck 校验；CI 逐步补 arm64 镜像构建 |
| 企业微信传大文件不稳定 | 传输失败 | 2G 分卷 + 每卷独立校验，坏哪卷补哪卷 |
| 模板化迁移期间新旧两套并存 | 配置漂移 | 阶段一即把模板作为唯一真相源收进 git，旧 scripts/ 目录标记 deprecated |
| 自动回滚误伤（SQL 已执行） | 数据不一致 | SQL 明确不自动回滚，升级预览中单独高亮 SQL 项要求确认 |
| 工具本身的 bug 在现场爆发 | 现场受阻 | 所有操作先渲染/预览再执行；关键步骤落盘中间产物，最坏情况人可接管手工继续 |

---

## 10. 预期收益复核

| 环节 | 现状 | 工具化后 |
|------|------|---------|
| 首次部署传包 | 20G，数小时且盯着 | base 提前一次性到位；release 3~5G 后台断点续传 |
| 环境初始化 | 手工半天 | `init` 10~20 分钟 |
| 配置修改 | 几十个文件半天、易错 | site.yaml 一个文件 5 分钟，强校验 |
| 启动验证 | 人肉顺序启动逐个看 | 分层自动化 + 健康矩阵 30~60 分钟 |
| 日常升级 | 手工替换 jar + rebuild，无版本记录，回滚靠翻旧包 | ≤30 分钟标准流程，版本化 + 台账，失败秒级回滚 |
| 操作者 | 仅限熟练开发 | 实施人员即可 |
| **单次交付合计** | **~1 天** | **≤3 小时；升级 ≤0.5 小时** |

按一年 5+ 项目、每项目首次部署 + 4~6 次升级估算，年节省 30~50 人天，开发投入约 25~30 人天，**首年即回本，次年起为净收益**，且附带交付质量与口碑提升。

---

## 附录 A：现有交付包盘点（2026-08，基于 middleware-new / platform-new 实包分析）

分析对象：`wpg-deploy-files` 下的两个真实交付包，共 23 份 docker-compose、10 份 `.env`。

### A.1 包结构

**middleware-new（数据库 + 中间件，13 个组件，每目录独立 compose）**：mysql 5.7.44、pgsql（5433，业务主库）、postgis 14（5432，GIS 库）、mongodb 4.4（GIS）、redis 6.0、kafka+zookeeper、nacos 2.2.3、minio、influxdb 2.4、emqx 4.3、nginx 1.24（前端统一入口 8877）、node-exporter、waterjob（xxl-job，实为 Java 业务服务混在中间件包）。

**platform-new（平台服务，8 个模块，17 个 Java 容器，9 份 env）**：public（网关+auth+msg+usercenter+log 共 5 服务）、device（dmc 4 服务）、alarm（2）、graph（组态 2）、gis（giscenter/gisdefault）、monitor（1 + 独立 prometheus）、report-center（1）、out-work（1）。

### A.2 关键发现与设计对应

| # | 发现 | 方案对应 |
|---|------|---------|
| 1 | 同一组 Nacos/数据库/Redis/Kafka 参数在 9 份 env 里重复 9 遍，且已漂移（3 份写 `10.10.10.10`，6 份写 `192.168.200.66`） | site.yaml 单一配置源（§5） |
| 2 | 变量命名两套并存：`REDIS_*`/`DATASOURCE_*` vs `WPG_REDIS_*`/`WPG_PGSQL_*`/`WPG_MONGODB_*` | 模板层做别名翻译，不改造存量服务（§6.5） |
| 3 | compose 内硬编码需手改的值：kafka advertised listener（本机 IP）、monitor 云端地址/`IS_CLOUD`/认证密码、alarm 的 `DATASOURCE_SCHEMA`、全部中间件密码 | 提升为模板变量（§6.5） |
| 4 | nginx `http-web-8877.conf`（285 行）内 6 处 proxy_pass 硬编码不同机器 IP，几十个前端 location 与 profile 无关联 | nginx conf 纳入渲染，location 按 profile 开关（§6.5） |
| 5 | 包内混入运行时数据：`nacos/data/`（raft/derby）、`pgsql/data/`（完整数据目录） | 数据目录不进包红线（§4.2），pack 强制排除 |
| 6 | 镜像来源 4 个：`harbor.shwpg.com:10445/wpg_release`、`10.10.102.75/ops_dev`、`10.10.102.75/wpg_release`、Docker Hub 官方 | pack 时统一收集，manifest 记录唯一来源 |
| 7 | 版本混杂无基线：v4.2.1 ~ v4.8.1（含 v4.7.5_patch.1、v4.2.5_1 等），无处记录"本次交付组合" | manifest 版本清单（§4.3） |
| 8 | 绝大多数服务 `network_mode: host`，compose 的 ports 配置无效 | 端口检测/防火墙以 manifest 为准（§6.2） |
| 9 | 人肉注意事项写在 yaml 注释里：mongodb 须 `--compatibility` 启动且必须挂 init-mongo.js、mysql init 目录按需删除 | manifest `composeFlags`/`requiredFiles` 字段（§4.3） |
| 10 | redis 密码在挂载的 redis.conf、prometheus 采集目标写在 targets/*.yml（文件名含 IP） | 中间件 conf 文件纳入渲染（§6.5） |

### A.3 端口清单（precheck / 防火墙放行基准）

- 中间件：3306（mysql）、5433（pgsql）、5432（postgis）、27017（mongodb）、6377（redis）、9092/2181（kafka/zk）、8848/9848（nacos）、9500（minio）、8086（influxdb）、1883/8081/8083/8883/8084/18083（emqx）、8877（nginx）、16120（node-exporter）、11005（waterjob）；
- 平台：18090~18095/18097/18099（public/alarm/monitor）、30015（report）、18005（out-work）、5588（组态编辑器）、11094/11095（gis）、29090（prometheus）。

### A.4 profile 划分（基于实包模块边界）

`platform`（public/report-center/out-work/graph）、`device`、`alarm`、`gis`（含 postgis、mongodb——不启用 gis 时这两个中间件也不部署）、`monitor`（monitor/prometheus/node-exporter）。middleware 基础件（mysql/pgsql/redis/kafka/nacos/nginx）不设 profile，始终部署；minio/influxdb/emqx 按依赖它们的 profile 自动带出。
