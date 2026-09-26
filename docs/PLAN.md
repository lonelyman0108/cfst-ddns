# cfst-ddns v2 重构计划

> 分支：`refactor/v2`　｜　创建：2026-09-24　｜　本文件为全生命周期唯一计划文档，持续更新阶段状态、勾选、验证结果与阻塞项。

## 0. 决策记录

| # | 决策 | 结论 | 原因 |
|---|------|------|------|
| D1 | 后端语言 | **Go 1.26** | homelab 常驻服务，单二进制、镜像 <40MB、内存 <30MB；路由器/NAS/ARM 可跑 |
| D2 | dev 分支 | 一并吸收 | dev 仅多 CHANGELOG / generate-changelog.sh / CI 标签规则；因本机无 git identity，采用 `git checkout origin/dev -- <files>` 吸收内容（内容与 dev 完全一致） |
| D3 | 兼容旧配置 | **不兼容**，提供 `docs/MIGRATION.md` | 旧配置为 shell 代码，无法可靠映射到多任务模型 |
| D4 | 用户模型 | 单用户 | 自用工具 |
| D5 | cfst 集成方式 | **子进程** + 解析 CSV | 上游 go.mod 模块名为 `CloudflareSpeedTest`，无法 import；依赖包级全局变量；GPL-3.0 许可隔离；支持 UI 切换版本 |
| D6 | 存储 | SQLite（GORM + `glebarez/sqlite`，纯 Go 无 CGO） | 零运维、交叉编译方便 |
| D7 | 前端 | Vue 3 + Vite + TypeScript + **shadcn-vue**（Tailwind v4 + reka-ui）+ Pinia + lucide 图标 | 2026-09-24 用户要求由 Element Plus 改为 shadcn-vue，现代风格；`go:embed` 打入二进制；字体本地打包（内网/离线可用） |
| D8 | 实时推送 | SSE | 单向日志流足够，比 WebSocket 简单 |
| D9 | 表单扩展 | 服务商/通知渠道由后端下发字段 Schema，前端动态渲染 | 新增服务商只改后端 |
| D10 | 密钥存储 | AES-256-GCM 加密后存库，主密钥来自 `CFST_DDNS_SECRET` 或 `data/secret.key` | 数据库泄露不直接泄露凭据 |

## 1. 目录结构

```
cmd/cfst-ddns/            # 入口：serve（默认）、reset-password、version
internal/
  app/                    # 启动装配、启动参数（env/flag）
  store/                  # GORM 模型、迁移、仓储
  secret/                 # AES-GCM 加解密
  auth/                   # bcrypt + JWT、中间件
  provider/               # DNSProvider 接口、注册表、Schema
    cloudflare.go dnspod.go tencentcloud.go alidns.go huaweicloud.go ...
  notify/                 # Notifier 接口、注册表、Schema
    bark.go telegram.go wecom.go dingtalk.go feishu.go serverchan.go pushplus.go gotify.go ntfy.go webhook.go smtp.go
  cfst/                   # 二进制管理（版本/下载/镜像/IP 文件）+ 运行器（子进程、日志流、CSV 解析）
  engine/                 # 任务执行：测速 → 选 IP → 同步 DNS → 通知 → 写历史
  scheduler/              # robfig/cron 调度
  logbus/                 # 运行日志发布订阅 + 应用日志环形缓冲
  api/                    # Gin 路由、处理器、SSE
web/                      # Vue 3 前端（构建产物 web/dist 由 go:embed 嵌入）
docs/                     # PLAN / API / MIGRATION
legacy/                   # 旧 Bash 版本（归档，v2 发布后删除）
```

数据目录（默认 `./data`，容器内 `/app/data`）：

```
data/cfst-ddns.db   data/secret.key   data/cfst/{cfst[.exe], ip.txt, ipv6.txt, *.default.txt, VERSION}   data/tmp/
```

## 2. 功能清单

- **认证**：首次启动初始化管理员（UI 或 `ADMIN_USERNAME/ADMIN_PASSWORD` 环境变量）；JWT；修改密码；CLI `reset-password`。
- **DNS 服务商**：Cloudflare（Token / Global Key）、DNSPod（旧版 Token）、腾讯云 DNSPod API 3.0、阿里云 DNS、华为云 DNS、GoDaddy。连通性测试、域名列表、记录列表。
- **通知渠道**：Bark、Telegram、企业微信机器人、钉钉机器人（加签）、飞书机器人（加签）、Server 酱、PushPlus、Gotify、ntfy、自定义 Webhook、SMTP 邮件。触发条件：成功 / 失败 / 仅 IP 变化时。测试发送。
- **任务**：多任务；cron（预设 + 下次执行预览）；v4 / v6 / both；cfst 全参数结构化配置 + 附加参数；自定义 IP 段；多目标记录（跨账号、跨域名）；Top-N 多 IP 负载均衡；IP 未变跳过；无合格 IP 保留原记录；Cloudflare proxied / TTL / 线路；克隆；启停；立即执行；取消执行。
- **执行**：全局串行队列（避免多测速互相干扰带宽）；SSE 实时日志 + 进度；结果入库。
- **历史**：执行列表（筛选/分页）、详情（测速结果表、DNS 变更、完整日志）、删除、按保留天数自动清理。
- **仪表盘**：统计卡片、各记录当前 IP、延迟/速度趋势图、下次执行时间、运行中任务。
- **cfst 管理**：当前版本、GitHub Releases 列表、安装/切换版本、镜像站、编辑 ip.txt / ipv6.txt、恢复默认。
- **系统设置**：GitHub 镜像、历史保留天数、Webhook 触发令牌（外部 `curl` 触发任务）、配置备份导出 / 导入恢复、应用日志查看、系统信息。
- **运维**：`/healthz`；多阶段 Dockerfile（standard / bundled 两个 target）；HEALTHCHECK；多架构 CI。

## 3. 阶段与状态

图例：⬜ 未开始　🟡 进行中　✅ 完成　⛔ 阻塞

### P0 准备 — ✅
- [x] 盘点现有项目与分支
- [x] 新建 `refactor/v2`，吸收 dev 分支内容
- [x] 验证 cfst 能否作为库引入（结论：不能，见 D5）
- [x] 编写本计划与 `docs/API.md`

### P1 后端骨架 — ✅
- [x] go.mod、入口、启动参数、数据目录初始化（另加 `healthcheck` 子命令）
- [x] store：模型 + 自动迁移
- [x] secret：AES-GCM
- [x] auth：初始化 / 登录（IP 失败限流）/ 改密（TokenVersion 使旧令牌失效）/ JWT 中间件 / reset-password
- [x] logbus + 应用日志
- 验证：`go build ./...`、`go vet ./...`；启动后 `curl /healthz`、setup/login 流程

### P2 服务商与通知抽象 — ✅
- [x] provider 接口、注册表、Schema；Cloudflare（参考实现）
- [x] DNSPod（旧版）、腾讯云 v3、阿里云、华为云、GoDaddy
- [x] notify 接口、注册表、Schema；Bark、Telegram（JSON 请求体 + HTML 转义，修复旧版截断）
- [x] 企业微信、钉钉、飞书、Server 酱、PushPlus、Gotify、ntfy、Webhook、SMTP
- 验证：签名算法单元测试（官方文档示例向量）；httptest 模拟服务端测试请求格式

### P3 cfst 管理与运行器 — ✅
- [x] 平台 → asset 名映射（复刻 install.sh 规则）；镜像拼接；Releases 列表；下载解压（zip / tar.gz）
- [x] 子进程运行：参数构建（强制 `-p 0` 防止 Windows 等待回车）、日志流（`\r` 进度 / `\n` 日志分流、去 ANSI）、取消
- [x] CSV 解析（兼容 6/7 列、BOM）
- 验证：单元测试（参数构建、CSV 解析）；本机真实下载 + 小规模测速（`-ip` 指定少量 IP）

### P4 任务引擎与调度 — ✅
- [x] 执行流程与全局串行队列（同任务去重、排队/运行中可取消、重启后遗留执行标记失败）
- [x] DNS 同步算法（保留已匹配 → 复用多余记录更新 → 新建 → 删除多余；未变跳过；无结果保留；按线路隔离）
- [x] 通知组装与分发（纯文本 + Markdown 两种正文，渠道并发发送）
- [x] cron 调度（增删改任务时热更新）、每日 04:10 历史清理
- 验证：同步算法单元测试（fake provider）；端到端执行一次

### P5 REST API — ✅
- [x] 按 `docs/API.md` 实现全部接口 + SSE（含 20s 心跳）
- [x] Webhook 触发（常量时间比较令牌）、备份导出/导入（事务、类型校验）
- [x] 静态资源嵌入与 SPA fallback
- 验证：curl 脚本冒烟

### P6 前端 — ✅（shadcn-vue）
- [x] Element Plus 首版（11 页面、API 层、SSE、路由守卫）——作为迁移基础保留 api/stores/router
- [x] shadcn-vue 重构：Tailwind v4、Sidebar 布局、⌘K 命令面板、亮/暗/跟随系统主题、Sonner、Skeleton、AlertDialog
- [x] 字体与字号规范：Inter（latin 子集）+ 系统中文回退（苹方/雅黑，不打包中文字体）、JetBrains Mono 等宽；基准 14px，页面标题 22px，统计数字 28px tabular-nums；中文字重不超过 600、不用负字距、最小 12px；移动端输入框 16px
- [x] 工程脚手架、布局、暗色模式、路由守卫
- [x] 登录 / 初始化
- [x] 仪表盘、任务、执行详情（实时日志）、历史、DNS 账号、通知、cfst、设置、日志
- 验证：`pnpm build`、`vue-tsc` 类型检查；浏览器实测主流程

### P7 部署与 CI — 🟡（已编写，待 CI 验证）
- [x] 多阶段 Dockerfile（node 构建 → go 交叉编译 → alpine 运行），standard / bundled target，HEALTHCHECK
- [x] docker-compose.yml
- [x] GitHub Actions：前端构建 + Go 测试 + 多架构镜像 + Release 附带 9 个平台二进制（2.0.1 起移除 MIPS：纯 Go SQLite 不支持）
- [x] 旧 Bash 文件移入 `legacy/`，删除 Dockerfile.bundled
- 验证：本机无 Docker，仅能 CI 验证（⚠ 交付时标注"镜像未本地验证"）

### P8 文档 — ✅
- [x] README 重写
- [x] `docs/MIGRATION.md`：旧环境变量 → 新 UI 位置对照
- [x] CHANGELOG 更新（Unreleased）；.github/SETUP.md 同步
- [x] 接入 release-please 自动生成 CHANGELOG 与发版；v2 变化一览移入 `docs/MIGRATION.md`；删除 `scripts/generate-changelog.sh`
- [x] Git 规范：`CONTRIBUTING.md`、`AGENTS.md`、PR 标题检查（`pr-title.yml`）、本地 `commit-msg` 钩子（`.githooks/`）

### P9 引导、图标与 cfst 导入 — ✅（分支 `feat/onboarding-and-icons`，2026-09-26 用户确认范围）
- [x] 首次引导向导 `/welcome`：cfst → DNS 账号 → 通知（可跳过）→ 首个任务（模板）→ 试跑
- [x] 初始化页支持从备份恢复；仪表盘入门清单（可关闭，状态存 `onboardingDismissed`）
- [x] 帮助页（快速上手 / 参数说明 / 常见问题 / Webhook / 备份迁移），⌘K 可搜索
- [x] 品牌图标：6 个服务商 + 11 个渠道全部为本地 logo（simple-icons / Iconify 开源图标集 / 官网 favicon，来源见 `web/src/assets/brands`）
- [x] cfst 页：操作列统一；最新版提示；上传导入（压缩包 / 裸二进制，校验文件头平台）；自动识别已有 cfst
- [x] GitHub 镜像测速与一键选用
- [x] 任务试运行（只测速，不写 DNS、不通知）
- [x] v1 配置导入（预览 → 确认创建）
- [x] 空状态补操作入口；移动端逐页检查
- [x] 系统设置页分节导航（与帮助页共用 `SectionNav`）；正文行内图标链接统一为 `InlineLink`；侧栏用户菜单重排
- 接口契约：`docs/API.md`（P9 新增部分）
- 验证：`go vet ./... && go test ./...`；`pnpm build`；浏览器走通引导全流程、上传导入、试运行、v1 导入

## 4. 验证记录

| 日期 | 阶段 | 内容 | 结果 |
|------|------|------|------|
| 2026-09-24 | P0 | `go get github.com/XIU2/CloudflareSpeedTest` | 失败：module declares its path as `CloudflareSpeedTest` → 采用子进程 |
| 2026-09-24 | P1–P5 | `go build ./...`、`go vet`、`go test ./internal/engine ./internal/cfst ./internal/schema` | 通过 |
| 2026-09-24 | P2 | `go vet ./...`、`go test ./...`（provider 23 项、notify 24 项，含 TC3/阿里云官方向量、华为云签名重算） | 通过 |
| 2026-09-24 | P3 | 修复：cfst 使用的 pb/v3 在非终端输出下不加 `
`、进度条首尾相接 → 改为分块读取并截取最后一条进度条；新增单测 `TestPumpConcatenatedBars`；真实测速（4 个 /20）确认推送了 progress 事件，完整日志只保留最后一条进度 | 通过 |
| 2026-09-24 | P5 | `/accounts/test` 与 `/notifiers/test` 新增可选 `id`，用已保存的配置补齐掩码密钥（前端报告的第 2 点） | 通过 `go vet`、`go test` |
| 2026-09-24 | P6 | `pnpm build`（vue-tsc 零错误）；grep 不到 element-plus；首屏 gzip 约 400KB → 约 185KB，dist 2.1MB → 1.5MB；浏览器走通初始化 → 账号 → 任务 → 执行（进度行实时刷新）→ 取消 → 历史 → 设置 → ⌘K；亮色和暗色截图；test 接口带 id 已抓包确认 | 通过 |
| 2026-09-24 | 集成 | 前端嵌入后二进制 28MB（`-s -w`）；8080 实例仪表盘渲染正常、控制台无错误；6 个服务商、11 个渠道全部注册 | 通过 |
| 2026-09-24 | P3–P5 | 本机冒烟：自动下载 cfst v2.3.5（Windows）→ 初始化/登录 → 建账号/任务 → 真实测速（自定义 3 个 /24、`-dd`）→ SSE 实时日志 → DNS 失败被正确记录为 failed → 重复触发 409 → Webhook 触发 + 取消 → 备份导出 | 通过；本机有 TUN 代理，延迟约 1 ms 不可信（已写入 README 注意事项） |
| 2026-09-25 | P7 | CI 失败原因：`pnpm/action-setup` 未指定版本 → `packageManager: pnpm@10.34.5`（本地 `--frozen-lockfile` 通过）；触发收紧为 main/PR/tag/手动；接入 release-please；`actionlint` 通过 | 通过（待 CI 实测） |
| 2026-09-26 | P9 | `go vet`、`go test ./...`（新增 importer / legacy / dryrun 测试）；`pnpm build`；浏览器（隔离实例 + 全新数据目录）：初始化 → `/welcome`（cfst 就绪、服务商/渠道图标）→ v1 导入预览与应用（假凭据下任务按设计停用并提示）→ 任务列表试运行（抽屉显示试运行徽标、实时日志、取消后 `dryRun=true`）→ 仪表盘入门清单 4/5 → `/help#faq` 锚点；390px 宽度下 15 个页面无横向滚动；亮色模式图标检查 | 通过；DNS 账号连通性与真实写入未测（无真实凭据） |

## 5. 阻塞与风险

- ⚠ 以下 API 细节凭记忆实现、未经文档核实，需真实账号冒烟：DNSPod 每页 3000 条上限与 error_on_empty 参数；腾讯云 Limit 3000 / Type=ALL / NoDataOfDomain；阿里云分页上限 / DomainRecordDuplicate；华为云 line 字段与 error_code 错误格式；GoDaddy 分页、404、空数组 PUT 与 DELETE 接口；Server 酱 sctp 地址格式、企业微信字节上限、PushPlus code 200、Gotify 错误字段。
- ⚠ GoDaddy API 仅对拥有 10 个以上域名（或特定套餐）的账号开放。
- ⚠ SMTP 的 ssl/starttls 路径无单测。

- ⚠ 本机无 Docker：镜像构建只能依赖 CI。
- ⚠ P9 品牌图标：DNSPod、Bark、Server 酱、PushPlus 为官网 favicon 位图（非矢量）；Gotify 图标为 CC BY 4.0（selfh.st），已在文件内注明。
- ⚠ `network_mode: host` 在 Docker Desktop（Windows/macOS）上无效，测速结果会失真，文档需注明。
- ⚠ 各云厂商 API 未用真实凭据验证时，仅保证签名与请求格式正确（单测），需用户用真实账号冒烟。
