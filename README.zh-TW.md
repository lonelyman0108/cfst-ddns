<p align="center">
  <img src="docs/assets/logo.svg" width="96" alt="cfst-ddns logo">
</p>

<h1 align="center">cfst-ddns</h1>

<p align="center">
  Cloudflare 優選 IP 自動測速 + DDNS 更新，附 Web 管理介面
</p>

<p align="center">
  <a href="https://github.com/lonelyman0108/cfst-ddns/releases/latest"><img src="https://img.shields.io/github/v/release/lonelyman0108/cfst-ddns?color=f3680f" alt="Release"></a>
  <a href="https://hub.docker.com/r/lonelyman0108/cfst-ddns"><img src="https://img.shields.io/docker/pulls/lonelyman0108/cfst-ddns?color=f3680f" alt="Docker Pulls"></a>
  <a href="https://github.com/lonelyman0108/cfst-ddns/actions/workflows/build-and-release.yml"><img src="https://img.shields.io/github/actions/workflow/status/lonelyman0108/cfst-ddns/build-and-release.yml?branch=main" alt="Build"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/lonelyman0108/cfst-ddns" alt="Go">
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <b>繁體中文</b> · <a href="README.ja.md">日本語</a>
</p>

使用 [CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) 定時測出最快的 Cloudflare IP，並自動寫入你的 DNS 紀錄。附 Web 管理介面，封裝為單一執行檔或 Docker 映像檔，x86 / ARM 的 NAS、軟路由、小主機都能執行。

> v2 是完全重寫的版本。從 v1（Bash 腳本）升級請參閱 [遷移指南](docs/MIGRATION.md)（簡體中文），也可以在首次初始化時直接匯入舊的 `config.sh`。

## 功能

- **Web 介面**：儀表板、任務管理、即時測速日誌、執行紀錄、延遲/速度趨勢圖，支援深色模式與行動裝置，服務商與管道皆有品牌圖示
- **四種介面語言**：English、简体中文、繁體中文、日本語，依瀏覽器語言自動選擇，也可在頂端列切換
- **上手引導**：首次引導精靈從安裝 cfst 一路帶到第一次試運行；儀表板提供入門清單，另有可搜尋的說明頁
- **多 DNS 服務商**：Cloudflare、DNSPod、騰訊雲（DNSPod API 3.0）、阿里雲、華為雲、GoDaddy
- **多通知管道**：Bark、Telegram、企業微信、釘釘、飛書、Server 醬、PushPlus、Gotify、ntfy、SMTP 郵件、自訂 Webhook
- **多任務**：每個任務可單獨設定 cron 週期、IPv4/IPv6/雙堆疊、cfst 全部參數、自訂 IP 段，並可同時更新跨帳號、跨網域的多筆紀錄
- **智慧更新**：IP 未變時略過；測速無結果時保留原紀錄；可寫入前 N 個 IP 做負載平衡；支援線路、TTL，以及 Cloudflare 代理開關；試運行只測速，不寫入 DNS、不發送通知
- **cfst 管理**：自動下載或上傳壓縮檔/執行檔匯入，自動偵測已有的 cfst，切換版本，GitHub 鏡像測速，編輯 IP 段檔案
- **維運**：Webhook 外部觸發、設定備份與還原、v1 設定匯入、歷史自動清理、憑證加密儲存、健康檢查

## 快速開始

### Docker Compose（建議）

```yaml
services:
  cfst-ddns:
    image: lonelyman0108/cfst-ddns:latest   # 離線環境用 :latest-bundled（預載 cfst）
    container_name: cfst-ddns
    restart: unless-stopped
    network_mode: host                      # 測速必須使用主機網路
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/app/data
```

```bash
docker compose up -d
```

啟動後開啟 `http://<主機IP>:8080`，建立管理員帳號（也可以在這裡從備份還原，或匯入 v1 設定）。接著引導精靈會帶你完成：

1. 確認 cfst 已安裝
2. 新增 DNS 帳號並測試連線
3. 選用：新增通知管道
4. 用範本建立第一個任務
5. 試運行一次，查看即時日誌確認一切正常

### 執行檔

從 [Releases](https://github.com/lonelyman0108/cfst-ddns/releases) 下載對應平台的壓縮檔。提供 Linux（amd64 / arm64 / armv7 / armv6 / 386）、Windows（amd64 / arm64）、macOS（amd64 / arm64）版本；暫不支援 MIPS（內建的純 Go SQLite 不支援該架構）。

解壓縮後執行：

```bash
./cfst-ddns -listen :8080 -data ./data
```

首次啟動時會自動下載 cfst。若網路無法連上 GitHub，請先在「系統設定」中選擇 GitHub 鏡像（可一鍵測速），再到「cfst 管理」頁安裝；也可以在該頁直接上傳 cfst 壓縮檔。

## 設定

大多數設定都在網頁中完成。啟動參數如下：

| 參數 | 環境變數 | 預設值 | 說明 |
|---|---|---|---|
| `-listen` | `CFST_DDNS_LISTEN` | `:8080` | 監聽位址 |
| `-data` | `CFST_DDNS_DATA` | `./data`（容器內 `/app/data`） | 資料目錄 |
| `-log-level` | `CFST_DDNS_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `-auto-install` | `CFST_DDNS_AUTO_INSTALL` | `true` | 未安裝 cfst 時自動下載 |
| `-bundle-dir` | `CFST_DDNS_BUNDLE_DIR` | — | 預載 cfst 目錄（bundled 映像檔使用） |
| — | `CFST_DDNS_SECRET` | 自動產生 `data/secret.key` | 憑證加密金鑰 |
| — | `ADMIN_USERNAME` / `ADMIN_PASSWORD` | — | 首次啟動時自動建立管理員 |

資料目錄結構：

```
data/
├── cfst-ddns.db   # 設定與歷史（SQLite）
├── secret.key     # 加密金鑰，務必與資料庫一起備份
├── cfst/          # cfst 程式與 IP 段檔案
└── tmp/           # 測速暫存檔案
```

> ⚠️ 遺失 `secret.key`（或更換 `CFST_DDNS_SECRET`）後，已儲存的憑證將無法解密。搬移到其他機器時，請複製整個 `data/` 目錄，或使用「系統設定 → 備份與還原」。

### 忘記密碼

```bash
docker exec cfst-ddns cfst-ddns reset-password -username admin -password 新密碼
# 執行檔：./cfst-ddns reset-password -data ./data -username admin -password 新密碼
```

### Webhook 觸發

在「系統設定」中啟用 Webhook 後，可以從外部觸發任務：

```bash
curl -X POST "http://<主機>:8080/api/hooks/tasks/<任務ID>/run?token=<權杖>"
```

## 測速注意事項

- **必須使用 host 網路**：Docker 的 bridge 網路會影響測速結果。Docker Desktop（Windows/macOS）不支援 host 網路，建議部署在 Linux 上，或直接執行執行檔。
- **關閉代理**：本機若執行了 Clash/Surge 等 TUN 或透明代理，測得的延遲會異常偏低（約 1 ms），結果不可信。請將執行測速的裝置排除在代理之外。
- **下載速度為 0**：cfst 預設的測速位址不保證可用，建議在任務中設定自架的「測速位址」，或開啟「停用下載測速」，只依延遲排序。
- **`-sl` 搭配 `-tl`**：只設定下載速度下限時，若湊不滿符合條件的 IP，cfst 可能長時間測速。

## 開發

需要 Go 1.26+、Node.js 22+ 和 pnpm。

```bash
# 前端（開發伺服器會把 /api 代理到 127.0.0.1:8080）
cd web && pnpm install && pnpm dev

# 後端
go run ./cmd/cfst-ddns -data ./data -log-level debug

# 完整建置（前端產物透過 go:embed 打包進執行檔）
cd web && pnpm build && cd .. && go build -o cfst-ddns ./cmd/cfst-ddns

# 測試
go test ./...

# Docker
docker build -t cfst-ddns .                    # standard
docker build --target bundled -t cfst-ddns:b . # 預載 cfst
```

專案結構見 [docs/PLAN.md](docs/PLAN.md)，API 約定見 [docs/API.md](docs/API.md)，提交與發佈規範見 [CONTRIBUTING.md](CONTRIBUTING.md)（皆為簡體中文）。

### 新增 DNS 服務商或通知管道

在 `internal/provider/`（或 `internal/notify/`）中新增一個檔案，實作對應介面，並在 `init()` 中呼叫 `Register` 註冊欄位 Schema。前端表單會依 Schema 自動產生，不需要修改前端程式碼。

## 致謝

- [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest)（GPL-3.0）：本專案以獨立子程序呼叫其發佈的程式。
