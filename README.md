<p align="center">
  <img src="docs/assets/logo.svg" width="96" alt="cfst-ddns logo">
</p>

<h1 align="center">cfst-ddns</h1>

<p align="center">
  Find the fastest Cloudflare IPs and keep your DNS records pointed at them — with a web UI
</p>

<p align="center">
  <a href="https://github.com/lonelyman0108/cfst-ddns/releases/latest"><img src="https://img.shields.io/github/v/release/lonelyman0108/cfst-ddns?color=f3680f" alt="Release"></a>
  <a href="https://hub.docker.com/r/lonelyman0108/cfst-ddns"><img src="https://img.shields.io/docker/pulls/lonelyman0108/cfst-ddns?color=f3680f" alt="Docker Pulls"></a>
  <a href="https://github.com/lonelyman0108/cfst-ddns/actions/workflows/build-and-release.yml"><img src="https://img.shields.io/github/actions/workflow/status/lonelyman0108/cfst-ddns/build-and-release.yml?branch=main" alt="Build"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/lonelyman0108/cfst-ddns" alt="Go">
</p>

<p align="center">
  <b>English</b> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.zh-TW.md">繁體中文</a> · <a href="README.ja.md">日本語</a>
</p>

cfst-ddns runs [CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) on a schedule to find the fastest Cloudflare IPs from your network, then writes them to your DNS records. It ships with a web UI as a single binary or a Docker image, and runs on x86 and ARM NAS boxes, soft routers and mini PCs.

> v2 is a complete rewrite. Upgrading from v1 (the Bash scripts)? See the [migration guide](docs/MIGRATION.md) (Chinese), or import your old `config.sh` during first-time setup.

## Features

- **Web UI**: dashboard, task management, live speed-test logs, run history and latency/speed trend charts; dark mode, mobile friendly, with brand logos for every provider and channel
- **Four languages**: the web UI is available in English, Simplified Chinese, Traditional Chinese and Japanese, detected from your browser and switchable in the header
- **Guided setup**: a setup wizard takes you from installing cfst to your first dry run; a getting-started checklist on the dashboard and a searchable help page cover the rest
- **DNS providers**: Cloudflare, DNSPod, Tencent Cloud (DNSPod API 3.0), Alibaba Cloud, Huawei Cloud, GoDaddy
- **Notifications**: Bark, Telegram, WeCom, DingTalk, Feishu/Lark, ServerChan, PushPlus, Gotify, ntfy, SMTP email and custom webhooks
- **Multiple tasks**: each task has its own cron schedule, IPv4/IPv6/dual-stack mode, full cfst options and custom IP ranges, and can update many records across accounts and domains
- **Smart updates**: skips unchanged IPs, keeps the existing record when a test finds nothing, can write the top N IPs for load balancing, and supports ISP lines, TTL and the Cloudflare proxy toggle; dry runs test speeds without touching DNS or sending notifications
- **cfst management**: automatic download or upload of an archive/binary, detection of an existing install, version switching, GitHub mirror speed test and IP range file editing
- **Operations**: webhook triggers, config backup and restore, v1 config import, automatic history cleanup, encrypted credentials and a health check

## Quick start

### Docker Compose (recommended)

```yaml
services:
  cfst-ddns:
    image: lonelyman0108/cfst-ddns:latest   # use :latest-bundled offline (cfst preinstalled)
    container_name: cfst-ddns
    restart: unless-stopped
    network_mode: host                      # speed tests must use the host network
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/app/data
```

```bash
docker compose up -d
```

Open `http://<host-ip>:8080` and create the admin account (you can also restore a backup or import a v1 config here). The setup wizard then walks you through:

1. Checking that cfst is installed
2. Adding a DNS account and testing the connection
3. Optional: adding a notification channel
4. Creating your first task from a template
5. A dry run, with the live log, to confirm everything works

### Binary

Download the archive for your platform from [Releases](https://github.com/lonelyman0108/cfst-ddns/releases). Builds are available for Linux (amd64 / arm64 / armv7 / armv6 / 386), Windows (amd64 / arm64) and macOS (amd64 / arm64). MIPS is not supported, because the embedded pure-Go SQLite does not support it.

Extract it and run:

```bash
./cfst-ddns -listen :8080 -data ./data
```

cfst is downloaded automatically on first start. If GitHub is unreachable from your network, pick a GitHub mirror in **Settings** (it can speed-test the mirrors for you) and install cfst from the **cfst** page, or upload a cfst archive there.

## Configuration

Most settings live in the web UI. Startup options:

| Flag | Environment variable | Default | Description |
|---|---|---|---|
| `-listen` | `CFST_DDNS_LISTEN` | `:8080` | Listen address |
| `-data` | `CFST_DDNS_DATA` | `./data` (`/app/data` in the container) | Data directory |
| `-log-level` | `CFST_DDNS_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `-auto-install` | `CFST_DDNS_AUTO_INSTALL` | `true` | Download cfst automatically if it is missing |
| `-bundle-dir` | `CFST_DDNS_BUNDLE_DIR` | — | Preinstalled cfst directory (used by the bundled image) |
| — | `CFST_DDNS_SECRET` | auto-generated `data/secret.key` | Credential encryption key |
| — | `ADMIN_USERNAME` / `ADMIN_PASSWORD` | — | Create the admin account on first start |

Data directory layout:

```
data/
├── cfst-ddns.db   # settings and history (SQLite)
├── secret.key     # encryption key; always back it up with the database
├── cfst/          # cfst binary and IP range files
└── tmp/           # temporary speed-test files
```

> ⚠️ If you lose `secret.key` (or change `CFST_DDNS_SECRET`), saved credentials can no longer be decrypted. When moving to another machine, copy the whole `data/` directory or use **Settings → Backup / Restore**.

### Forgot your password

```bash
docker exec cfst-ddns cfst-ddns reset-password -username admin -password NEW_PASSWORD
# binary: ./cfst-ddns reset-password -data ./data -username admin -password NEW_PASSWORD
```

### Webhook triggers

After enabling webhooks in **Settings**, you can trigger a task from outside:

```bash
curl -X POST "http://<host>:8080/api/hooks/tasks/<task-id>/run?token=<token>"
```

## Speed-test notes

- **Use host networking**: Docker's bridge network skews results. Docker Desktop (Windows/macOS) does not support host networking, so deploy on Linux or run the binary directly.
- **Turn off proxies**: TUN or transparent proxies such as Clash or Surge make latency look unrealistically low (around 1 ms). Exclude the device running the tests from your proxy.
- **Download speed is 0**: cfst's default test URL is not guaranteed to work. Set your own test URL in the task, or disable the download test and sort by latency only.
- **`-sl` with `-tl`**: if you only set a minimum download speed and not enough IPs qualify, cfst may keep testing for a long time.

## Development

Requires Go 1.26+, Node.js 22+ and pnpm.

```bash
# Frontend (the dev server proxies /api to 127.0.0.1:8080)
cd web && pnpm install && pnpm dev

# Backend
go run ./cmd/cfst-ddns -data ./data -log-level debug

# Full build (the frontend is embedded with go:embed)
cd web && pnpm build && cd .. && go build -o cfst-ddns ./cmd/cfst-ddns

# Tests
go test ./...

# Docker
docker build -t cfst-ddns .                    # standard
docker build --target bundled -t cfst-ddns:b . # cfst preinstalled
```

See [docs/PLAN.md](docs/PLAN.md) for the project layout, [docs/API.md](docs/API.md) for the API, and [CONTRIBUTING.md](CONTRIBUTING.md) for commit and release conventions (all in Chinese).

### Adding a DNS provider or notification channel

Create a file in `internal/provider/` (or `internal/notify/`), implement the interface, and call `Register` in `init()` with the field schema. The frontend builds its forms from the schema, so no frontend changes are needed.

## Acknowledgements

- [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) (GPL-3.0): cfst-ddns runs its released binary as a separate process.
