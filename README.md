<p align="center">
  <img src="docs/assets/logo.svg" width="96" alt="cfst-ddns logo">
</p>

<h1 align="center">cfst-ddns</h1>

<p align="center">
  Cloudflare 优选 IP 自动测速 + DDNS 更新，带 Web 管理界面
</p>

<p align="center">
  <a href="https://github.com/lonelyman0108/cfst-ddns/releases/latest"><img src="https://img.shields.io/github/v/release/lonelyman0108/cfst-ddns?color=f3680f" alt="Release"></a>
  <a href="https://hub.docker.com/r/lonelyman0108/cfst-ddns"><img src="https://img.shields.io/docker/pulls/lonelyman0108/cfst-ddns?color=f3680f" alt="Docker Pulls"></a>
  <a href="https://github.com/lonelyman0108/cfst-ddns/actions/workflows/build-and-release.yml"><img src="https://img.shields.io/github/actions/workflow/status/lonelyman0108/cfst-ddns/build-and-release.yml?branch=main" alt="Build"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/lonelyman0108/cfst-ddns" alt="Go">
</p>

<p align="center">
  <b>简体中文</b> · <a href="README.en.md">English</a>
</p>

使用 [CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) 定时测出最快的 Cloudflare IP，并自动写入你的 DNS 记录。带 Web 管理界面，打包为单个二进制或 Docker 镜像，x86 / ARM 的 NAS、软路由、小主机都能运行。

> v2 是完全重写的版本。从 v1（Bash 脚本）升级请阅读 [迁移指南](docs/MIGRATION.md)。

## 功能

- **Web 界面**：仪表盘、任务管理、实时测速日志、执行历史、延迟/速度趋势图，支持暗色模式与移动端
- **多 DNS 服务商**：Cloudflare、DNSPod、腾讯云（DNSPod API 3.0）、阿里云、华为云、GoDaddy
- **多通知渠道**：Bark、Telegram、企业微信、钉钉、飞书、Server 酱、PushPlus、Gotify、ntfy、SMTP 邮件、自定义 Webhook
- **多任务**：每个任务可单独设置 cron 周期、IPv4/IPv6/双栈、cfst 全部参数、自定义 IP 段，并可同时更新跨账号、跨域名的多条记录
- **智能更新**：IP 未变时跳过；测速无结果时保留原记录；可写入前 N 个 IP 做负载均衡；支持线路、TTL，以及 Cloudflare 代理开关
- **cfst 管理**：在界面中查看版本、切换版本、配置 GitHub 镜像、编辑 IP 段文件
- **运维**：Webhook 外部触发、配置备份与恢复、历史自动清理、凭据加密存储、健康检查

## 快速开始

### Docker Compose（推荐）

```yaml
services:
  cfst-ddns:
    image: lonelyman0108/cfst-ddns:latest   # 离线环境用 :latest-bundled（预置 cfst）
    container_name: cfst-ddns
    restart: unless-stopped
    network_mode: host                      # 测速必须使用宿主机网络
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/app/data
```

```bash
docker compose up -d
```

启动后打开 `http://<主机IP>:8080`，按以下顺序完成配置：

1. 创建管理员账号
2. 在「DNS 账号」中添加服务商凭据，并测试连接
3. 可选：在「通知渠道」中添加推送方式，并发送一条测试
4. 在「任务」中新建任务，选择目标记录，然后点击「立即执行」观察实时日志

### 二进制

从 [Releases](https://github.com/lonelyman0108/cfst-ddns/releases) 下载对应平台的压缩包。提供 Linux（amd64 / arm64 / armv7 / armv6 / 386）、Windows（amd64 / arm64）、macOS（amd64 / arm64）版本；暂不支持 MIPS（内置的纯 Go SQLite 不支持该架构）。

解压后运行：

```bash
./cfst-ddns -listen :8080 -data ./data
```

首次启动时会自动下载 cfst。国内网络如果下载失败，先在「系统设置」中配置 GitHub 镜像，再到「cfst 管理」页点击安装。

## 配置

多数配置都在网页中完成。启动参数如下：

| 参数 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `-listen` | `CFST_DDNS_LISTEN` | `:8080` | 监听地址 |
| `-data` | `CFST_DDNS_DATA` | `./data`（容器内 `/app/data`） | 数据目录 |
| `-log-level` | `CFST_DDNS_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `-auto-install` | `CFST_DDNS_AUTO_INSTALL` | `true` | 未安装 cfst 时自动下载 |
| `-bundle-dir` | `CFST_DDNS_BUNDLE_DIR` | — | 预置 cfst 目录（bundled 镜像使用） |
| — | `CFST_DDNS_SECRET` | 自动生成 `data/secret.key` | 凭据加密密钥 |
| — | `ADMIN_USERNAME` / `ADMIN_PASSWORD` | — | 首次启动时自动创建管理员 |

数据目录结构：

```
data/
├── cfst-ddns.db   # 配置与历史（SQLite）
├── secret.key     # 加密密钥，务必与数据库一起备份
├── cfst/          # cfst 程序与 IP 段文件
└── tmp/           # 测速临时文件
```

> ⚠️ 丢失 `secret.key`（或更换 `CFST_DDNS_SECRET`）后，已保存的凭据将无法解密。迁移机器时，要么复制整个 `data/` 目录，要么使用「系统设置 → 备份与恢复」。

### 忘记密码

```bash
docker exec cfst-ddns cfst-ddns reset-password -username admin -password 新密码
# 二进制：./cfst-ddns reset-password -data ./data -username admin -password 新密码
```

### Webhook 触发

在「系统设置」中启用 Webhook 后，可以从外部触发任务：

```bash
curl -X POST "http://<主机>:8080/api/hooks/tasks/<任务ID>/run?token=<令牌>"
```

## 测速注意事项

- **必须使用 host 网络**：Docker 的 bridge 网络会影响测速结果。Docker Desktop（Windows/macOS）不支持 host 网络，建议在 Linux 上部署，或直接运行二进制。
- **关闭代理**：本机如果运行了 Clash/Surge 等 TUN 或透明代理，测到的延迟会异常低（约 1 ms），结果不可信。请把运行测速的设备排除在代理之外。
- **下载速度为 0**：cfst 默认的测速地址不保证可用，建议在任务中设置自建的「测速地址」，或者开启「禁用下载测速」，只按延迟排序。
- **`-sl` 搭配 `-tl`**：只设置下载速度下限时，如果凑不够满足条件的 IP，cfst 可能长时间测速。

## 开发

需要 Go 1.26+、Node.js 22+ 和 pnpm。

```bash
# 前端（开发服务器会把 /api 代理到 127.0.0.1:8080）
cd web && pnpm install && pnpm dev

# 后端
go run ./cmd/cfst-ddns -data ./data -log-level debug

# 完整构建（前端产物通过 go:embed 打入二进制）
cd web && pnpm build && cd .. && go build -o cfst-ddns ./cmd/cfst-ddns

# 测试
go test ./...

# Docker
docker build -t cfst-ddns .                    # standard
docker build --target bundled -t cfst-ddns:b . # 预置 cfst
```

项目结构见 [docs/PLAN.md](docs/PLAN.md)，接口约定见 [docs/API.md](docs/API.md)，提交与发布规范见 [CONTRIBUTING.md](CONTRIBUTING.md)。

### 新增 DNS 服务商或通知渠道

在 `internal/provider/`（或 `internal/notify/`）中新建一个文件，实现对应接口，并在 `init()` 中调用 `Register` 注册字段 Schema。前端表单会根据 Schema 自动生成，不需要修改前端代码。

## 致谢

- [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest)（GPL-3.0）：本项目以独立子进程调用其发布的程序。
