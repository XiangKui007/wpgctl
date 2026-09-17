# wpgctl 完整实施计划（v2）

> 日期：2026-09-09  
> 原则：CLI 能力已齐 → 补齐 Web 闭环 → 降低实施门槛 → 打磨体验  
> 部署目标：Linux 现场主控机 **或** Windows + Docker Desktop

---

## 0. 现状盘点

### 已具备（CLI）
- precheck / init / fetch / render / deploy
- db apply / nacos import / upgrade / rollback
- status / logs / diag / pack / encrypt / ui
- SSH 多机分发骨架、site.yaml 校验、vault 加密

### 已具备（Web）
- 四步部署向导、YAML 高亮编辑、本机目录浏览
- 状态 / 日志 / 台账、Windows Desktop 体检适配

### 缺口（按使用频率）
1. 网页无升级/回滚、无拉包
2. site 只有 YAML，实施难改
3. 部署过程缺进度与健康矩阵
4. 部署前无预览确认
5. 路径选择/报错/密码展示偏糙

---

## 1. 完整功能地图

```
公司侧                 传输                    现场（Linux 或 Win+Desktop）
─────────            ─────                   ─────────────────────────────
pack base/release    fetch / 企微 / 硬盘      precheck → init
pack patch           断点续传+校验            render → deploy（分层+健康）
Jenkins 调度                                  upgrade / rollback
                                              db apply / nacos import
                                              status / logs / diag
                                              UI 向导（本机 127.0.0.1:9527）
```

---

## 2. 里程碑与任务清单

### P0 · Web 运维闭环（本周优先）— 让实施几乎不用记命令

| ID | 任务 | 验收标准 |
|----|------|----------|
| P0-1 | 环境横幅：OS / Docker 状态 / 路径提示 | 顶栏可见；Win 明确“可用 Desktop 部署” |
| P0-2 | Web「升级」页：选 patch 目录 → 预览 → 执行 → 日志 | 等价 `wpgctl upgrade` |
| P0-3 | Web「回滚」页：选历史版本一键回滚 | 等价 `wpgctl rollback --to` |
| P0-4 | Web「拉包」页：URL 拉取 / 本地目录导入 + 校验结果 | 等价 `wpgctl fetch` / `--local` |

### P1 · 降低门槛与可观测性

| ID | 任务 | 验收标准 |
|----|------|----------|
| P1-1 | site 表单编辑（IP/密码/profiles/paths）+ YAML 高级切换 | 表单保存通过同一套校验 |
| P1-2 | 部署任务进度：当前层、服务矩阵、失败自动带日志 | Job WebSocket 推结构化进度 |
| P1-3 | 部署前预览：启用服务、端口、profiles、dry-run 渲染 | 确认后再真正 deploy |
| P1-4 | init/deploy 在 Windows 上对 host 网络给出明确提示与跳过策略 | 不静默失败 |

### P2 · 体验打磨

| ID | 任务 | 验收标准 |
|----|------|----------|
| P2-1 | 路径选择器：最近路径、搜索、检测 manifest.yaml | 选错目录提前红字提示 |
| P2-2 | 报错可执行化（端口占用/Docker 未启等） | 每条红项带「怎么处理」 |
| P2-3 | 网页保存后密码回显脱敏；改密单独字段 | 投屏不裸奔明文 |
| P2-4 | diag 一键从 UI 下载 | 出问题可打包发回公司 |

### P3 · 远期（排期但不阻塞当前）

| ID | 任务 |
|----|------|
| P3-1 | SSH 远程节点目录浏览 |
| P3-2 | 总部多站点心跳看板 |
| P3-3 | 包签名（minisign） |
| P3-4 | K3s 评估（服务规模翻倍后再做） |

---

## 3. 本轮实现顺序

1. **P0-1** 环境横幅完善  
2. **P0-2 / P0-3** 升级 + 回滚 Web API 与页面  
3. **P0-4** 拉包 / 本地导入 Web  
4. **P1-1** site 表单 + YAML 切换  
5. **P1-2 / P1-3** 部署进度与预览  
6. **P2** 体验项按剩余时间推进  

---

## 4. 技术约定

- 后端：现有 `internal/ui` 增 API，任务仍走 Job + WebSocket
- 前端：继续 Vue 3 单页，不引入重型 UI 库
- Windows：Docker Desktop 为正式目标；`network_mode:host` 黄灯提示
- 每完成一项：编译嵌入前端、本地 `wpgctl ui` 可验证

---

## 6. 进度跟踪（2026-09-09）

| ID | 状态 |
|----|------|
| P0-1 环境横幅 + Docker 状态 | ✅ |
| P0-2 Web 升级 | ✅ |
| P0-3 Web 回滚 | ✅ |
| P0-4 Web 拉包/导入 | ✅ |
| P1-1 site 表单 + YAML 切换 | ✅ |
| P1-2 部署进度/健康矩阵 | ✅ |
| P1-3 部署前预览 | ✅ |
| P1-4 Windows host 网络提示 | ✅（体检黄灯） |
| P2-1 路径选择器标 [manifest] | ✅ |
| P2-2 报错 Hint | ✅（端口/Docker/磁盘/内存） |
| P2-3 表单密码脱敏+空值保留 | ✅ |
| P2-4 UI 下载 diag | ⏳ 未做 |
| P3-* 远期 | ⏳ 未排期 |

**浏览器验收路径**：http://127.0.0.1:9527  
顶栏 → 部署向导 / 拉包 / 升级 / 状态 / 台账
