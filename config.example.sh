#!/usr/bin/env bash

# =========================
# CloudflareSpeedTest DDNS 配置文件
# =========================
# 使用说明：
# 1. 复制此文件为 config.sh: cp config.example.sh config.sh
# 2. 编辑 config.sh 填入你的实际配置
# 3. config.sh 会被 .gitignore 忽略，不会泄露敏感信息

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

# Zone ID 和域名配置
# Zone ID 获取：Cloudflare Dashboard → 选择域名 → 右侧 API 区域
CF_ZONE_ID="your_zone_id_here"

# 要更新的域名列表（空格分隔，支持多个域名）
# 示例：CF_RECORD_NAMES="test1.example.com test2.example.com test3.example.com"
CF_RECORD_NAMES="test.example.com"

# DNS 记录类型
# A=IPv4, AAAA=IPv6
CF_RECORD_TYPE="A"

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

# 测速模式
# auto: 根据 CF_RECORD_TYPE 自动选择（默认）
# v4: 仅测速 IPv4
# v6: 仅测速 IPv6
# both: 同时测速 IPv4 和 IPv6
CFST_TEST_MODE="auto"

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

# GitHub 镜像站点（可选，加速下载 CloudflareSpeedTest）
# 留空使用官方 GitHub，或使用以下镜像站点之一：
# GITHUB_MIRROR="https://ghproxy.com/https://github.com"
# GITHUB_MIRROR="https://mirror.ghproxy.com/https://github.com"
# GITHUB_MIRROR="https://gh.api.99988866.xyz/https://github.com"
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
