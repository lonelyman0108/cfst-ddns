# CloudflareSpeedTest DDNS 自动更新脚本

## 功能说明

自动执行 CloudflareSpeedTest 测速，并将最优 IP 通过 Cloudflare API 更新到指定域名的 DNS 记录。

### 主要特性

- ✅ 自动测速获取最优 Cloudflare IP
- ✅ 支持多域名同时更新
- ✅ 支持 IPv4/IPv6 双栈测速和更新
- ✅ 支持多种通知方式（Bark、Telegram）
- ✅ 智能判断是否需要更新
- ✅ 详细的执行日志和结果汇总
- ✅ 支持 Docker 部署
- ✅ 自动下载 CloudflareSpeedTest
- ✅ 支持 GitHub 镜像加速

## 快速开始

### 方式一：使用 Docker Hub 镜像（最简单）

直接使用已构建好的多架构镜像，支持 `amd64`、`arm64`、`armv7` 等架构：

```bash
# 1. 创建项目目录
mkdir -p cfst-ddns && cd cfst-ddns

# 2. 下载 docker-compose.yml
curl -O https://raw.githubusercontent.com/lonelyman0108/cfst-ddns/main/docker-compose.yml

# 3. 创建配置文件
mkdir -p data
curl -o data/config.sh https://raw.githubusercontent.com/lonelyman0108/cfst-ddns/main/config.example.sh
vim data/config.sh  # 编辑配置

# 4. 启动定时任务（每6小时执行一次）
docker-compose up -d

# 5. 查看日志
docker-compose logs -f cfst-ddns
```

**可用镜像标签：**
- `lonelyman0108/cfst-ddns:latest` - 最新版本（主分支）
- `lonelyman0108/cfst-ddns:v1.0.0` - 特定版本（语义化版本）
- `lonelyman0108/cfst-ddns:main` - 主分支最新构建

### 方式二：从源码构建 Docker 镜像

```bash
# 1. 克隆项目
git clone https://github.com/lonelyman0108/cfst-ddns.git
cd cfst-ddns

# 2. 创建数据目录并配置
mkdir -p data
cp config.example.sh data/config.sh
vim data/config.sh  # 编辑配置

# 3. 启动定时任务（每6小时执行一次）
docker-compose up -d

# 4. 查看日志
docker-compose logs -f cfst-ddns
```

### 方式三：本地运行

```bash
# 1. 克隆项目
git clone https://github.com/lonelyman0108/cfst-ddns.git
cd cfst-ddns

# 2. 安装 CloudflareSpeedTest
./install.sh

# 3. 配置脚本
cp config.example.sh config.sh
vim config.sh

# 4. 运行脚本
./cfst_ddns.sh
```

## 详细使用步骤

### 1. 获取 Cloudflare API 凭证

**方式一：使用 API Token（推荐）**

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. 点击右上角头像 → My Profile → API Tokens
3. 点击 "Create Token"
4. 选择 "Edit zone DNS" 模板，或自定义权限：
   - Permissions: `Zone` - `DNS` - `Edit`
   - Zone Resources: 选择你的域名
5. 创建后复制 Token（只显示一次）

**方式二：使用 Global API Key**

1. 登录 Cloudflare Dashboard
2. My Profile → API Tokens → Global API Key
3. 点击 "View" 并复制

### 2. 获取 Zone ID

1. 进入 Cloudflare Dashboard
2. 选择你的域名
3. 右侧 "API" 区域可以看到 Zone ID

### 3. 配置脚本

**推荐方式：使用配置文件（避免敏感信息泄露）**

```bash
# 复制配置示例文件
cp config.example.sh config.sh

# 编辑配置文件
vim config.sh  # 或使用其他编辑器
```

**配置文件示例（[config.example.sh](config.example.sh)）：**

```bash
# ========== Cloudflare API 配置 ==========
# 方式一：使用 API Token（推荐）
CF_API_TOKEN="your_api_token_here"
CF_ZONE_ID="your_zone_id_here"

# 方式二：使用 Global API Key
# CF_API_EMAIL="your_email@example.com"
# CF_API_KEY="your_global_api_key"
# CF_ZONE_ID="your_zone_id_here"

# ========== DNS 记录配置 ==========
# 要更新的域名（支持多个域名，空格分隔）
CF_RECORD_NAMES="test1.example.com test2.example.com"

# DNS 记录类型（A=IPv4, AAAA=IPv6）
CF_RECORD_TYPE="A"

# ========== 测速配置 ==========
# CloudflareSpeedTest 测速参数
# -n: 测速线程数量 -t: 延迟测速次数 -sl: 下载速度下限(MB/s)
CFST_PARAMS="-n 200 -t 4 -sl 5"

# 测速模式（auto/v4/v6/both）
# auto: 根据 CF_RECORD_TYPE 自动选择
# v4: 仅测速 IPv4
# v6: 仅测速 IPv6
# both: 同时测速 IPv4 和 IPv6
CFST_TEST_MODE="auto"

# 跳过测速（使用已保存的测速结果）
# true: 跳过测速，使用上次结果
# false: 执行测速
SKIP_SPEED_TEST="false"

# ========== 通知配置 ==========
# Bark 通知（iOS）
ENABLE_BARK="false"
BARK_URL="https://api.day.app"
BARK_KEY=""

# Telegram 通知
ENABLE_TELEGRAM="false"
TG_BOT_TOKEN=""
TG_CHAT_ID=""

# ========== 其他配置 ==========
# 数据目录（测速结果保存位置）
# Docker: /app/data
# 本地: 脚本所在目录
DATA_DIR=""
```

**或者直接编辑主脚本**（不推荐，容易泄露敏感信息）

编辑 [cfst_ddns.sh](cfst_ddns.sh:29-34) 文件的默认配置区域。

### 4. 可选：调整测速参数

在 `config.sh` 中修改测速相关配置：

```bash
# 测速参数
CFST_PARAMS="-n 200 -t 4 -sl 5"

# 测速模式
CFST_TEST_MODE="auto"  # auto/v4/v6/both

# 跳过测速（使用已保存结果）
SKIP_SPEED_TEST="false"
```

**测速参数说明：**
- `-n` 测速线程数量（默认200）
- `-t` 延迟测速次数（默认4）
- `-sl` 下载速度下限，单位MB/s（如设置为5，则只保留>=5MB/s的IP）
- `-dn` 下载测速数量（默认10）

**测速模式说明：**
- `auto`：根据 `CF_RECORD_TYPE` 自动选择（默认）
- `v4`：仅测速 IPv4
- `v6`：仅测速 IPv6
- `both`：同时测速 IPv4 和 IPv6

**跳过测速说明：**

脚本会自动保存测速结果到 `result_ddns.txt.v4` 和 `result_ddns.txt.v6`。

设置 `SKIP_SPEED_TEST="true"` 可跳过测速，直接使用上次保存的结果，方便快速测试其他步骤（如 DNS 更新、通知等）。

### 5. 配置通知（可选）

脚本支持多种通知方式，可以同时启用。

**Bark 通知（iOS）**

```bash
ENABLE_BARK="true"
BARK_URL="https://api.day.app"
BARK_KEY="your_bark_key"
```

获取 Bark Key:
1. 在 iPhone 上安装 [Bark](https://apps.apple.com/cn/app/bark/id1403753865) 应用
2. 打开应用获取设备密钥
3. 填入配置文件

**Telegram 通知**

```bash
ENABLE_TELEGRAM="true"
TG_BOT_TOKEN="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
TG_CHAT_ID="123456789"
```

获取 Telegram 配置:
1. 与 [@BotFather](https://t.me/BotFather) 对话创建机器人
2. 获取 Bot Token
3. 与你的机器人对话，发送任意消息
4. 访问 `https://api.telegram.org/bot<你的Token>/getUpdates` 获取 Chat ID
5. 填入配置文件

### 6. 运行脚本

#### 本地运行

```bash
# 进入脚本目录
cd /path/to/cfst-ddns

# 首次运行：安装 CloudflareSpeedTest
./install.sh

# 执行 DDNS 脚本
./cfst_ddns.sh
```

#### 使用 Docker 运行

**方式一：定时任务模式（推荐）**

默认配置已启用定时任务模式（`ENABLE_CRON=true`），容器会持续运行并按计划执行：

```bash
# 启动定时任务容器
docker-compose up -d

# 查看运行状态
docker-compose ps

# 查看日志
docker-compose logs -f cfst-ddns

# 停止定时任务
docker-compose down
```

**方式二：单次执行模式**

如果只想手动执行一次，需要修改配置：

1. 编辑 [docker-compose.yml](docker-compose.yml)：
   ```yaml
   environment:
     - ENABLE_CRON=false  # 改为 false
   ```

2. 注释掉 `restart: unless-stopped`：
   ```yaml
   # restart: unless-stopped
   ```

3. 执行单次任务：
   ```bash
   docker-compose run --rm cfst-ddns
   ```

## 安装 CloudflareSpeedTest

### 自动安装（推荐）

运行安装脚本会自动检测系统架构并下载对应版本：

```bash
./install.sh
```

**支持的系统：**
- Linux (x86_64, ARM64, ARM v5/v6/v7, MIPS 等)
- macOS (Intel, Apple Silicon)
- Windows (x86_64, ARM64)

### 使用 GitHub 镜像加速

如果下载速度慢，可以使用 GitHub 镜像站点：

```bash
# 方式一：使用环境变量
export GITHUB_MIRROR="https://ghp.ci"
./install.sh

# 方式二：可用的镜像站点
export GITHUB_MIRROR="https://ghproxy.cc"
export GITHUB_MIRROR="https://mirror.ghproxy.com"
export GITHUB_MIRROR="https://gh-proxy.com"
export GITHUB_MIRROR="https://gh.api.99988866.xyz"
```

### 手动安装

1. 访问 [CloudflareSpeedTest Releases](https://github.com/XIU2/CloudflareSpeedTest/releases)
2. 下载对应系统的版本
3. 解压到 `./cfst/` 目录
4. 确保可执行文件有执行权限

## Docker 部署

### 使用预构建镜像（推荐）

项目提供了多架构 Docker 镜像，自动构建并发布到 Docker Hub：

```bash
# 拉取最新镜像
docker pull lonelyman0108/cfst-ddns:latest

# 或拉取特定版本
docker pull lonelyman0108/cfst-ddns:v1.0.0
```

**支持的架构：**
- `linux/amd64` - x86_64 架构（Intel/AMD）
- `linux/arm64` - ARM 64位架构（Apple Silicon、树莓派4等）
- `linux/arm/v7` - ARM v7 架构（树莓派3等）

Docker 会自动选择适合你系统的架构镜像。

### 镜像构建

如果需要自己构建镜像：

```bash
# 本地构建
docker build -t cfst-ddns:latest .

# 使用 docker-compose 构建
docker-compose build
```

### CI/CD 自动构建

项目使用 GitHub Actions 自动构建和推送 Docker 镜像到 Docker Hub。

**自动触发条件：**
- 推送代码到 `main` 或 `master` 分支 → 生成 `latest` 标签
- 推送 Git 标签（如 `v1.0.0`） → 生成版本标签（`1.0.0`、`1.0`、`1`）
- 创建 Pull Request → 仅构建不推送

**如需自己配置 CI/CD：**

查看 [GitHub Actions 设置指南](.github/SETUP.md) 了解如何配置 Docker Hub 密钥。

### 配置说明

**目录结构：**

```
cfst-ddns/
├── data/              # 配置和数据目录（挂载到容器）
│   └── config.sh      # 配置文件
├── logs/              # 日志目录（可选）
├── cfst/              # cfst 可执行文件（可选挂载，避免重复下载）
├── cfst_ddns.sh       # 主脚本
├── install.sh         # 安装脚本
├── Dockerfile         # Docker 镜像配置
└── docker-compose.yml # Docker Compose 配置
```

**环境变量：**

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `TZ` | 时区设置 | `Asia/Shanghai` |
| `GITHUB_MIRROR` | GitHub 镜像站点 | 空（使用官方） |
| `AUTO_INSTALL_CFST` | 自动安装 CloudflareSpeedTest | `true` |
| `DATA_DIR` | 数据目录（测速结果保存位置） | Docker: `/app/data`<br>本地: 脚本所在目录 |
| `CRON_SCHEDULE` | 定时任务执行频率（cron 表达式） | `0 */6 * * *`（每6小时） |

**修改定时任务间隔：**

编辑 [docker-compose.yml:23](docker-compose.yml#L23) 中的 `CRON_SCHEDULE` 环境变量：

```yaml
environment:
  # 定时任务执行频率（cron 表达式）
  - CRON_SCHEDULE=0 */6 * * *  # 默认: 每6小时
```

常用示例：
- 每天凌晨3点：`0 3 * * *`
- 每小时：`0 * * * *`
- 每12小时：`0 */12 * * *`
- 每30分钟：`*/30 * * * *`

修改后重启容器：
```bash
docker-compose restart cfst-ddns
```

## 定时执行（可选）

### macOS 使用 crontab

```bash
# 编辑 crontab
crontab -e

# 添加定时任务（每天凌晨3点执行）
0 3 * * * cd /Users/lm/Desktop/cfst-ddns && ./cfst_ddns.sh >> /tmp/cfst_ddns.log 2>&1

# 或每6小时执行一次
0 */6 * * * cd /Users/lm/Desktop/cfst-ddns && ./cfst_ddns.sh >> /tmp/cfst_ddns.log 2>&1
```

### macOS 使用 launchd（推荐）

创建配置文件 `~/Library/LaunchAgents/com.cfst.ddns.plist`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.cfst.ddns</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/lm/Desktop/cfst-ddns/cfst_ddns.sh</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/Users/lm/Desktop/cfst-ddns</string>
    <key>StartInterval</key>
    <integer>21600</integer>
    <key>StandardOutPath</key>
    <string>/tmp/cfst_ddns.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/cfst_ddns_error.log</string>
</dict>
</plist>
```

加载任务：
```bash
launchctl load ~/Library/LaunchAgents/com.cfst.ddns.plist
```

## 脚本流程

1. 检查配置是否完整
2. 执行 CloudflareSpeedTest 测速
3. 获取最优 IP
4. 查询当前 DNS 记录
5. 对比 IP 是否需要更新
6. 调用 Cloudflare API 更新/创建 DNS 记录
7. 显示执行结果

## 注意事项

1. **首次使用建议**：先手动执行一次，确认配置正确
2. **API Token 权限**：确保 Token 有 DNS 编辑权限
3. **测速环境**：关闭代理软件，否则测速结果可能不准确
4. **IP 类型匹配**：IPv4 使用 A 记录，IPv6 使用 AAAA 记录
5. **DNS 记录**：如果域名记录不存在，脚本会自动创建
6. **Proxied 设置**：脚本默认关闭 Cloudflare 代理（proxied: false），如需开启请修改脚本

## 故障排查

### 测速失败
- 检查 cfst 可执行文件路径是否正确
- 确认 ip.txt 或 ipv6.txt 文件存在
- 查看是否有代理干扰测速

### API 调用失败
- 验证 API Token/Key 是否正确
- 检查 Zone ID 是否匹配域名
- 确认 API Token 权限是否足够
- 查看返回的错误信息

### 更新后不生效
- DNS 记录有 TTL 缓存时间，需要等待
- 检查是否开启了 Cloudflare 代理（橙色云朵）
- 使用 `dig` 或 `nslookup` 验证 DNS 记录

## 参考链接

- [CloudflareSpeedTest 项目](https://github.com/XIU2/CloudflareSpeedTest)
- [Cloudflare API 文档](https://developers.cloudflare.com/api/)
