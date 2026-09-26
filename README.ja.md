<p align="center">
  <img src="docs/assets/logo.svg" width="96" alt="cfst-ddns logo">
</p>

<h1 align="center">cfst-ddns</h1>

<p align="center">
  最速の Cloudflare IP を自動で測定し、DNS レコードを更新し続けるツール（Web 管理画面付き）
</p>

<p align="center">
  <a href="https://github.com/lonelyman0108/cfst-ddns/releases/latest"><img src="https://img.shields.io/github/v/release/lonelyman0108/cfst-ddns?color=f3680f" alt="Release"></a>
  <a href="https://hub.docker.com/r/lonelyman0108/cfst-ddns"><img src="https://img.shields.io/docker/pulls/lonelyman0108/cfst-ddns?color=f3680f" alt="Docker Pulls"></a>
  <a href="https://github.com/lonelyman0108/cfst-ddns/actions/workflows/build-and-release.yml"><img src="https://img.shields.io/github/actions/workflow/status/lonelyman0108/cfst-ddns/build-and-release.yml?branch=main" alt="Build"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/lonelyman0108/cfst-ddns" alt="Go">
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.zh-TW.md">繁體中文</a> · <b>日本語</b>
</p>

cfst-ddns は [CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) を定期的に実行して、お使いのネットワークから最も速い Cloudflare IP を見つけ、DNS レコードに書き込みます。Web 管理画面を備え、単一バイナリまたは Docker イメージとして提供されており、x86 / ARM の NAS、ソフトウェアルーター、小型 PC で動作します。

> v2 は完全に書き直されたバージョンです。v1（Bash スクリプト）からアップグレードする場合は [移行ガイド](docs/MIGRATION.md)（中国語）を参照するか、初回セットアップ時に旧 `config.sh` をインポートしてください。

## 機能

- **Web 画面**：ダッシュボード、タスク管理、リアルタイムの速度テストログ、実行履歴、レイテンシ/速度の推移グラフ。ダークモードとモバイルに対応し、各プロバイダーと通知チャネルにはブランドアイコンを表示
- **4 言語対応**：Web 画面は English、简体中文、繁體中文、日本語に対応。ブラウザーの言語から自動で選ばれ、ヘッダーで切り替え可能
- **セットアップガイド**：初回セットアップウィザードで cfst のインストールから最初のドライランまで案内。ダッシュボードの「はじめに」チェックリストと検索可能なヘルプページも用意
- **DNS プロバイダー**：Cloudflare、DNSPod、Tencent Cloud（DNSPod API 3.0）、Alibaba Cloud、Huawei Cloud、GoDaddy
- **通知チャネル**：Bark、Telegram、WeCom、DingTalk、Feishu/Lark、ServerChan、PushPlus、Gotify、ntfy、SMTP メール、カスタム Webhook
- **複数タスク**：タスクごとに cron スケジュール、IPv4/IPv6/デュアルスタック、cfst の全パラメーター、カスタム IP 範囲を設定でき、複数のアカウント・ドメインにまたがる複数レコードをまとめて更新
- **スマートな更新**：IP が変わらなければスキップ、測定結果がなければ既存レコードを保持、上位 N 個の IP を書き込んで負荷分散、回線・TTL・Cloudflare プロキシの切り替えに対応。ドライランでは速度テストのみ行い、DNS の書き込みや通知はしません
- **cfst 管理**：自動ダウンロードまたはアーカイブ/バイナリのアップロード、既存 cfst の自動検出、バージョン切り替え、GitHub ミラーの速度テスト、IP 範囲ファイルの編集
- **運用**：Webhook による外部トリガー、設定のバックアップ / 復元、v1 設定のインポート、履歴の自動削除、認証情報の暗号化保存、ヘルスチェック

## クイックスタート

### Docker Compose（推奨）

```yaml
services:
  cfst-ddns:
    image: lonelyman0108/cfst-ddns:latest   # オフライン環境では :latest-bundled（cfst 同梱）
    container_name: cfst-ddns
    restart: unless-stopped
    network_mode: host                      # 速度テストにはホストネットワークが必須
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/app/data
```

```bash
docker compose up -d
```

`http://<ホストIP>:8080` を開き、管理者アカウントを作成します（ここでバックアップからの復元や v1 設定のインポートも可能です）。その後、セットアップウィザードが次の手順を案内します。

1. cfst がインストールされていることを確認
2. DNS アカウントを追加して接続をテスト
3. 任意：通知チャネルを追加
4. テンプレートから最初のタスクを作成
5. ドライランを実行し、リアルタイムログで動作を確認

### バイナリ

[Releases](https://github.com/lonelyman0108/cfst-ddns/releases) からお使いのプラットフォーム用のアーカイブをダウンロードします。Linux（amd64 / arm64 / armv7 / armv6 / 386）、Windows（amd64 / arm64）、macOS（amd64 / arm64）版を提供しています。組み込みの純 Go 製 SQLite が対応していないため、MIPS はサポートしていません。

展開して実行します。

```bash
./cfst-ddns -listen :8080 -data ./data
```

cfst は初回起動時に自動でダウンロードされます。ネットワークから GitHub に接続できない場合は、**設定** で GitHub ミラーを選び（ミラーの速度テストも可能）、**cfst 管理** ページからインストールするか、同ページで cfst のアーカイブをアップロードしてください。

## 設定

ほとんどの設定は Web 画面で行います。起動オプションは次のとおりです。

| オプション | 環境変数 | デフォルト | 説明 |
|---|---|---|---|
| `-listen` | `CFST_DDNS_LISTEN` | `:8080` | 待ち受けアドレス |
| `-data` | `CFST_DDNS_DATA` | `./data`（コンテナ内は `/app/data`） | データディレクトリ |
| `-log-level` | `CFST_DDNS_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `-auto-install` | `CFST_DDNS_AUTO_INSTALL` | `true` | cfst が未インストールなら自動でダウンロード |
| `-bundle-dir` | `CFST_DDNS_BUNDLE_DIR` | — | 同梱 cfst のディレクトリ（bundled イメージで使用） |
| — | `CFST_DDNS_SECRET` | 自動生成される `data/secret.key` | 認証情報の暗号化キー |
| — | `ADMIN_USERNAME` / `ADMIN_PASSWORD` | — | 初回起動時に管理者を自動作成 |

データディレクトリの構成：

```
data/
├── cfst-ddns.db   # 設定と履歴（SQLite）
├── secret.key     # 暗号化キー。必ずデータベースと一緒にバックアップ
├── cfst/          # cfst 本体と IP 範囲ファイル
└── tmp/           # 速度テストの一時ファイル
```

> ⚠️ `secret.key` を失う（または `CFST_DDNS_SECRET` を変更する）と、保存済みの認証情報は復号できなくなります。別のマシンに移行するときは、`data/` ディレクトリごとコピーするか、**設定 → バックアップ / 復元** を使ってください。

### パスワードを忘れた場合

```bash
docker exec cfst-ddns cfst-ddns reset-password -username admin -password 新しいパスワード
# バイナリ：./cfst-ddns reset-password -data ./data -username admin -password 新しいパスワード
```

### Webhook トリガー

**設定** で Webhook を有効にすると、外部からタスクを実行できます。

```bash
curl -X POST "http://<ホスト>:8080/api/hooks/tasks/<タスクID>/run?token=<トークン>"
```

## 速度テストの注意点

- **ホストネットワークを使う**：Docker の bridge ネットワークでは測定結果が歪みます。Docker Desktop（Windows/macOS）はホストネットワークに対応していないため、Linux にデプロイするか、バイナリを直接実行してください。
- **プロキシをオフにする**：Clash や Surge などの TUN / 透過プロキシが動いていると、レイテンシが不自然に低く（約 1 ms）なり、結果を信頼できません。テストを実行する端末はプロキシの対象外にしてください。
- **ダウンロード速度が 0**：cfst のデフォルトのテスト URL は常に使えるとは限りません。タスクで独自のテスト URL を設定するか、ダウンロードテストを無効にしてレイテンシのみで並べ替えてください。
- **`-sl` と `-tl` の併用**：ダウンロード速度の下限だけを設定し、条件を満たす IP が足りない場合、cfst が長時間テストを続けることがあります。

## 開発

Go 1.26+、Node.js 22+、pnpm が必要です。

```bash
# フロントエンド（開発サーバーは /api を 127.0.0.1:8080 にプロキシ）
cd web && pnpm install && pnpm dev

# バックエンド
go run ./cmd/cfst-ddns -data ./data -log-level debug

# フルビルド（フロントエンドは go:embed でバイナリに埋め込み）
cd web && pnpm build && cd .. && go build -o cfst-ddns ./cmd/cfst-ddns

# テスト
go test ./...

# Docker
docker build -t cfst-ddns .                    # standard
docker build --target bundled -t cfst-ddns:b . # cfst 同梱
```

プロジェクト構成は [docs/PLAN.md](docs/PLAN.md)、API は [docs/API.md](docs/API.md)、コミットとリリースの規約は [CONTRIBUTING.md](CONTRIBUTING.md) を参照してください（いずれも中国語）。

### DNS プロバイダーや通知チャネルの追加

`internal/provider/`（または `internal/notify/`）にファイルを作成してインターフェースを実装し、`init()` で `Register` を呼び出してフィールドのスキーマを登録します。フロントエンドのフォームはスキーマから自動生成されるため、フロントエンドの変更は不要です。

## 謝辞

- [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest)（GPL-3.0）：cfst-ddns はリリース版のバイナリを独立したサブプロセスとして実行しています。
