#!/usr/bin/env bash

# =========================
# CloudflareSpeedTest DDNS 配置文件
# =========================
# 使用说明：
# 1. 复制此文件为 config.sh: cp config.example.sh config.sh
# 2. 编辑 config.sh 填入你的实际配置
# 3. config.sh 会被 .gitignore 忽略，不会泄露敏感信息

# =========================
# DNS 提供商选择
# =========================
# 支持的提供商：cloudflare, dnspod
DNS_PROVIDER="cloudflare"

# =========================
# 要更新的域名列表（空格分隔，支持多个域名）
# =========================
# Cloudflare 示例：DNS_RECORD_NAMES="test1.example.com test2.example.com"
# DNSPod 示例：DNS_RECORD_NAMES="test1.example.com test2.example.com example.com"
# 注意：对于DNSPod，会自动解析主域名和子域名
DNS_RECORD_NAMES="test.example.com"

# =========================
# Cloudflare API 配置
# =========================

# 方式一：使用 API Token（推荐）
# 获取方式：Cloudflare Dashboard → My Profile → API Tokens → Create Token
# 权限：Zone - DNS - Edit
CF_API_TOKEN=""

# 方式二：使用 Global API Key（如使用 Token 则这两项留空）
# 获取方式：Cloudflare Dashboard → My Profile → API Tokens → Global API Key
CF_API_KEY=""
CF_EMAIL=""

# Zone ID 配置
# Zone ID 获取：Cloudflare Dashboard → 选择域名 → 右侧 API 区域
CF_ZONE_ID="your_zone_id_here"

# =========================
# DNSPod API 配置
# =========================

# DNSPod API Token
# 获取方式：DNSPod 控制台 → 用户中心 → 安全设置 → API Token
# https://console.dnspod.cn/account/token/token
# 格式：ID,Token (例如: 12345,1234567890abcdef1234567890abcdef)
DNSPOD_TOKEN=""

# =========================
# 测速配置
# =========================

# cfst 可执行文件路径（一般不需要修改）
# CFST_BIN="./cfst/cfst"

# 数据目录（测速结果文件保存位置）
# 本地运行：默认为脚本所在目录
# Docker 运行：自动设置为 /app/data
# DATA_DIR=""

# cfst 测速参数
# -n: 测速线程数（默认200，可提高至500）
# -t: 延迟测速次数（默认4次）
# -sl: 下载速度下限，单位MB/s（只保留>=该速度的IP）
# -dn: 下载测速数量（默认10个）
# -dt: 下载测速时间，单位秒（默认10秒）
CFST_PARAMS="-n 200 -t 4"

# 测速模式（同时决定 DNS 记录类型）
# v4: 仅测速 IPv4,更新 A 记录（默认）
# v6: 仅测速 IPv6,更新 AAAA 记录
# both: 同时测速 IPv4 和 IPv6,同时更新 A 和 AAAA 记录
CFST_TEST_MODE="v4"

# 是否跳过测速（使用已有测速结果）
# true: 跳过测速，使用上次保存的结果（快速测试其他步骤）
# false: 执行完整测速（默认）
SKIP_SPEED_TEST="false"

# 测速结果文件名（一般不需要修改）
# 实际会生成 result_ddns.txt.v4 和 result_ddns.txt.v6
# RESULT_FILE="result_ddns.txt"

# =========================
# 下载配置
# =========================

# GitHub 镜像站点（用于加速下载 CloudflareSpeedTest）
# 留空则使用官方 GitHub，国内服务器建议配置镜像站点
# 注意：由于镜像站点可用性会变化，请根据实际情况选择
# 镜像站点示例：
# GITHUB_MIRROR="https://你的镜像站点"
GITHUB_MIRROR=""

# =========================
# 通知配置
# =========================

# Bark 通知（iOS）
ENABLE_BARK="false"                          # 是否启用 Bark 通知
BARK_URL="https://api.day.app"               # Bark 服务器地址
BARK_KEY=""                                  # Bark 设备密钥

# Telegram 通知
ENABLE_TELEGRAM="false"                      # 是否启用 Telegram 通知
TG_BOT_TOKEN=""                              # Telegram Bot Token
TG_CHAT_ID=""                                # Telegram Chat ID

# 通知配置说明：
# 1. Bark 通知：
#    - 在 iPhone 上安装 Bark 应用
#    - 获取设备密钥，填入 BARK_KEY
#    - 设置 ENABLE_BARK="true" 启用
#
# 2. Telegram 通知：
#    - 与 @BotFather 对话创建机器人，获取 Bot Token
#    - 与机器人对话后，访问 https://api.telegram.org/bot<TOKEN>/getUpdates 获取 Chat ID
#    - 填入 TG_BOT_TOKEN 和 TG_CHAT_ID
#    - 设置 ENABLE_TELEGRAM="true" 启用
#
# 3. 可以同时启用多种通知方式
