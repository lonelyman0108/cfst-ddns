# 从 v1（Bash 脚本版）迁移到 v2

v2 用 Go + Web 界面重写了项目，**不再读取** v1 的 `config.sh` 或环境变量。所有配置都改为在网页中完成，保存在 `data/cfst-ddns.db`，其中的凭据会加密存储。迁移大约需要 5 分钟。

## v2 变化一览

#### 变更
- **v2 完全重写**：Go 后端 + Vue 3 Web 界面，单二进制 / Docker 部署；旧版 Bash 脚本移至 `legacy/`
- 配置改为网页管理并加密存入 SQLite，不再读取 `config.sh` 与旧环境变量（见 `docs/MIGRATION.md`）
- 镜像合并为同一个多阶段 Dockerfile（`standard` / `bundled` 两个构建目标），新增 HEALTHCHECK
- CI 新增前端构建、Go 测试与 13 个平台的二进制发布

#### 新增
- 多任务：独立的 cron、IP 类型、cfst 全参数、自定义 IP 段、跨账号多目标记录
- DNS 服务商：腾讯云 DNSPod API 3.0、阿里云、华为云、GoDaddy（原有 Cloudflare、DNSPod）
- 通知渠道：企业微信、钉钉、飞书、Server 酱、PushPlus、Gotify、ntfy、SMTP、自定义 Webhook（原有 Bark、Telegram）
- 更新策略：IP 未变跳过、无结果保留原记录、Top-N 多记录负载均衡、线路 / TTL / Cloudflare 代理
- 实时测速日志（SSE）、执行历史、仪表盘趋势图、cfst 版本管理与 IP 段编辑
- Webhook 外部触发、配置备份与恢复、历史自动清理、`reset-password` 命令

#### 修复
- Telegram 消息未编码导致含 `&` 等字符时被截断
- 测速无可用 IP 时可能写入空记录

## 1. 记录旧配置

打开旧的 `config.sh` 或 `docker-compose.yml`，把下列值抄下来备用：

- DNS 凭据：`CF_API_TOKEN`（或 `CF_EMAIL` + `CF_API_KEY`），或 `DNSPOD_TOKEN`
- 域名：`DNS_RECORD_NAMES`
- 测速：`CFST_PARAMS`、`CFST_TEST_MODE`
- 定时：`CRON_SCHEDULE`
- 通知：Bark、Telegram 相关配置
- 镜像：`GITHUB_MIRROR`

## 2. 替换容器

```bash
docker compose down
# 可选：旧版数据目录里只有测速结果和日志，v2 用不到，可以直接删除
mv data data.v1.bak
```

用仓库中新的 [`docker-compose.yml`](../docker-compose.yml) 覆盖旧文件，然后启动：

```bash
docker compose up -d
```

需要注意：

- 旧版的 `ENABLE_CRON`、`CRON_SCHEDULE`、`AUTO_INSTALL_CFST`、`LOG_MAX_SIZE` 等环境变量都已**移除**，定时规则改为按任务配置。
- `./cfst` 和 `./logs` 挂载也不再需要。cfst 安装在 `data/cfst/`，日志可以在网页的「系统日志」页查看，或用 `docker logs` 查看。
- 打开 `http://<主机IP>:8080` 创建管理员账号。

## 3. 配置对照表

| v1 配置 | v2 位置 | 说明 |
|---|---|---|
| `DNS_PROVIDER=cloudflare` + `CF_API_TOKEN` | DNS 账号 → 新建 → Cloudflare → 认证方式选「API Token」 | Token 需要 `Zone:Read` + `DNS:Edit` 权限 |
| `CF_EMAIL` + `CF_API_KEY` | 同上，认证方式选「Global API Key」 | |
| `CF_ZONE_ID` | Cloudflare 账号的「Zone ID（可选）」 | 只有 Token 缺少 `Zone:Read` 权限时才需要填写 |
| `DNS_PROVIDER=dnspod` + `DNSPOD_TOKEN` | DNS 账号 → 新建 → DNSPod | 格式不变，仍为 `ID,Token` |
| `DNS_RECORD_NAMES="a.example.com b.example.com"` | 任务 → 目标记录（每个域名一行） | 拆分成「主域名 `example.com`」+「主机记录 `a`」；根域名的主机记录填 `@` |
| `CFST_TEST_MODE=v4/v6/both` | 任务 → 基本 → IP 类型 | 含义不变 |
| `CFST_PARAMS="-n 200 -t 4 -sl 5 -tl 200"` | 任务 → 测速参数 | 每个参数都有独立输入框；表单里没有的参数填到「附加参数」 |
| `CRON_SCHEDULE="0 */6 * * *"` | 任务 → 基本 → 执行周期 | 同样是 5 段 cron 表达式；留空表示只能手动执行 |
| `ENABLE_CRON=false`（单次执行） | 任务 → 立即执行，或通过 Webhook 触发 | 见下方「外部触发」 |
| `SKIP_SPEED_TEST` | 已移除 | 每次执行都会重新测速 |
| `ENABLE_BARK` / `BARK_URL` / `BARK_KEY` | 通知渠道 → 新建 → Bark | 服务器地址对应 `BARK_URL`，设备密钥对应 `BARK_KEY` |
| `ENABLE_TELEGRAM` / `TG_BOT_TOKEN` / `TG_CHAT_ID` | 通知渠道 → 新建 → Telegram | 修复了旧版消息含 `&` 等字符时被截断的问题 |
| `GITHUB_MIRROR` | 系统设置 → GitHub 镜像 | 支持两种写法：`https://mirror.example`（替换 `github.com`），或 `https://proxy.example/{url}` |
| `CFST_VERSION` | cfst 管理 → 选择版本安装 | |
| `DATA_DIR` / `RESULT_FILE` | 已移除 | 测速结果改存数据库，可在执行历史中查看 |
| `TZ` | 仍为环境变量 `TZ` | |

新建任务后需要在任务里勾选通知渠道。每个渠道可以单独设置「成功时 / 失败时 / 仅 IP 变化时」是否通知。

## 4. 行为差异

- **IP 未变化时不再调用更新接口。** 如需每次都强制更新，在任务中关闭「IP 未变化时跳过」。
- **测速没有得到合格 IP 时保留原记录。** 旧版在这种情况下可能写入空值或直接失败。
- **支持多条同名记录。** 「每种类型写入 IP 数」设为 N 时，会维护 N 条同名记录，用于简单的负载均衡；数量调小后，多余的记录会被删除。
- **只处理同名、同类型、同线路的记录。** 其他记录不会被改动。
- **多个任务串行执行。** 这样可以避免同时测速互相抢占带宽。

## 5. 外部触发（替代单次执行模式）

在「系统设置 → Webhook 触发」中启用并复制令牌：

```bash
curl -X POST "http://<主机>:8080/api/hooks/tasks/<任务ID>/run?token=<令牌>"
```

## 6. 不使用 Docker

从 [Releases](https://github.com/lonelyman0108/cfst-ddns/releases) 下载对应平台的二进制：

```bash
./cfst-ddns -listen :8080 -data ./data
```

旧版 Bash 脚本保留在 [`legacy/`](../legacy/) 目录，仅供参考，后续版本会删除。
