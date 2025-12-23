#!/usr/bin/env bash
set -e

# =========================
# CloudflareSpeedTest 自动安装脚本
# =========================

# 颜色输出
RED() { echo -e "\033[31m$1\033[0m"; }
GREEN() { echo -e "\033[32m$1\033[0m"; }
YELLOW() { echo -e "\033[33m$1\033[0m"; }
BLUE() { echo -e "\033[36m$1\033[0m"; }

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CFST_DIR="${SCRIPT_DIR}/cfst"

# GitHub 镜像站点配置
# 如果 GITHUB_MIRROR 为空，使用官方 GitHub
# 如果有值，使用指定的镜像站点
# 例如：export GITHUB_MIRROR="https://ghproxy.com"
GITHUB_MIRROR="${GITHUB_MIRROR:-}"

BLUE "=== CloudflareSpeedTest 自动安装脚本 ==="
echo ""

# 检测系统架构
detect_platform() {
    local os arch

    # 检测操作系统
    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        MINGW*|MSYS*|CYGWIN*)    os="windows" ;;
        *)
            RED "错误: 不支持的操作系统: $(uname -s)"
            exit 1
            ;;
    esac

    # 检测 CPU 架构
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        aarch64|arm64)  arch="arm64" ;;
        armv7l)         arch="armv7" ;;
        armv6l)         arch="armv6" ;;
        armv5*)         arch="armv5" ;;
        i386|i686)      arch="386" ;;
        mips)           arch="mips" ;;
        mips64)         arch="mips64" ;;
        mips64le)       arch="mips64le" ;;
        mipsle)         arch="mipsle" ;;
        *)
            RED "错误: 不支持的 CPU 架构: $(uname -m)"
            exit 1
            ;;
    esac

    # 文件扩展名
    local ext
    if [[ "$os" == "windows" ]]; then
        ext="zip"
    elif [[ "$os" == "darwin" ]]; then
        ext="zip"
    else
        ext="tar.gz"
    fi

    # 返回文件名
    echo "cfst_${os}_${arch}.${ext}"
}

# 获取最新版本号
get_latest_version() {
    # 输出到 stderr 避免混入返回值
    BLUE "正在获取最新版本信息..." >&2

    # API 总是使用官方 GitHub API
    local api_url="https://api.github.com/repos/XIU2/CloudflareSpeedTest/releases/latest"

    local version=$(curl -sL "$api_url" | grep -o '"tag_name": *"[^"]*"' | sed 's/"tag_name": *"//;s/"//')

    if [[ -z "$version" ]]; then
        RED "错误: 无法获取最新版本信息" >&2
        exit 1
    fi

    echo "$version"
}

# 下载并解压
download_and_extract() {
    local version="$1"
    local filename="$2"

    BLUE "\n正在下载 CloudflareSpeedTest $version..."
    BLUE "文件名: $filename"

    # 创建临时目录
    local tmp_dir=$(mktemp -d)
    cd "$tmp_dir"

    # 构建下载 URL
    local download_url
    if [[ -n "$GITHUB_MIRROR" ]]; then
        # 使用镜像站点
        download_url="${GITHUB_MIRROR}/XIU2/CloudflareSpeedTest/releases/download/${version}/${filename}"
        BLUE "使用镜像站点: $GITHUB_MIRROR"
    else
        # 使用官方 GitHub
        download_url="https://github.com/XIU2/CloudflareSpeedTest/releases/download/${version}/${filename}"
    fi

    BLUE "下载地址: $download_url"

    # 下载文件
    if ! curl -L -o "$filename" "$download_url"; then
        RED "错误: 下载失败"
        rm -rf "$tmp_dir"
        exit 1
    fi

    GREEN "✓ 下载完成"

    # 解压文件
    BLUE "\n正在解压..."
    if [[ "$filename" == *.tar.gz ]]; then
        tar -xzf "$filename"
    elif [[ "$filename" == *.zip ]]; then
        # 对于 zip 文件，只解压我们需要的文件，避免中文文件名编码问题
        # 明确指定需要解压的文件，不使用通配符
        unzip -o -j "$filename" cfst CloudflareST ip.txt ipv6.txt 2>/dev/null || \
        unzip -o -j "$filename" cfst.exe CloudflareST.exe ip.txt ipv6.txt 2>/dev/null || true
    else
        RED "错误: 不支持的压缩格式"
        rm -rf "$tmp_dir"
        exit 1
    fi

    GREEN "✓ 解压完成"

    # 创建 cfst 目录
    mkdir -p "$CFST_DIR"

    # 移动文件
    BLUE "\n正在安装文件到 $CFST_DIR..."

    # 查找可执行文件（cfst 或 cfst.exe）
    local exe_file
    if [[ -f "cfst" ]]; then
        exe_file="cfst"
    elif [[ -f "CloudflareST" ]]; then
        exe_file="CloudflareST"
        mv "$exe_file" "cfst"
        exe_file="cfst"
    elif [[ -f "cfst.exe" ]]; then
        exe_file="cfst.exe"
    elif [[ -f "CloudflareST.exe" ]]; then
        exe_file="CloudflareST.exe"
        mv "$exe_file" "cfst.exe"
        exe_file="cfst.exe"
    else
        RED "错误: 找不到可执行文件"
        rm -rf "$tmp_dir"
        exit 1
    fi

    # 移动可执行文件和 IP 列表文件
    mv "$exe_file" "$CFST_DIR/"
    [[ -f "ip.txt" ]] && mv ip.txt "$CFST_DIR/"
    [[ -f "ipv6.txt" ]] && mv ipv6.txt "$CFST_DIR/"

    # 设置执行权限
    chmod +x "$CFST_DIR/$exe_file"

    # 清理临时文件
    cd "$SCRIPT_DIR"
    rm -rf "$tmp_dir"

    GREEN "✓ 安装完成"
}

# 验证安装
verify_installation() {
    BLUE "\n正在验证安装..."

    local exe_file
    if [[ -f "$CFST_DIR/cfst" ]]; then
        exe_file="$CFST_DIR/cfst"
    elif [[ -f "$CFST_DIR/cfst.exe" ]]; then
        exe_file="$CFST_DIR/cfst.exe"
    else
        RED "错误: 安装验证失败，找不到可执行文件"
        exit 1
    fi

    if [[ ! -x "$exe_file" ]]; then
        RED "错误: 可执行文件没有执行权限"
        exit 1
    fi

    # 检查 IP 列表文件
    if [[ ! -f "$CFST_DIR/ip.txt" ]]; then
        YELLOW "警告: 未找到 ip.txt，IPv4 测速可能无法使用"
    fi

    if [[ ! -f "$CFST_DIR/ipv6.txt" ]]; then
        YELLOW "警告: 未找到 ipv6.txt，IPv6 测速可能无法使用"
    fi

    GREEN "✓ 安装验证通过"
    GREEN "\n可执行文件: $exe_file"
    [[ -f "$CFST_DIR/ip.txt" ]] && GREEN "IPv4 列表: $CFST_DIR/ip.txt"
    [[ -f "$CFST_DIR/ipv6.txt" ]] && GREEN "IPv6 列表: $CFST_DIR/ipv6.txt"
}

# 主函数
main() {
    # 检测平台
    local filename=$(detect_platform)
    GREEN "检测到系统: $(uname -s) $(uname -m)"
    GREEN "将下载: $filename"

    # 检查是否已安装
    if [[ -f "$CFST_DIR/cfst" ]] || [[ -f "$CFST_DIR/cfst.exe" ]]; then
        YELLOW "\n检测到已安装 CloudflareSpeedTest"
        read -p "是否覆盖安装? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            BLUE "已取消安装"
            exit 0
        fi
        rm -rf "$CFST_DIR"
    fi

    # 获取最新版本
    local version=$(get_latest_version)
    GREEN "最新版本: $version"

    # 下载并解压
    download_and_extract "$version" "$filename"

    # 验证安装
    verify_installation

    echo ""
    GREEN "==================================="
    GREEN "CloudflareSpeedTest 安装成功！"
    GREEN "==================================="
    echo ""
    BLUE "提示："
    BLUE "1. 使用 ./cfst_ddns.sh 运行 DDNS 脚本"
    BLUE "2. 编辑 config.sh 配置文件"
    echo ""
    YELLOW "GitHub 镜像站点："
    YELLOW "如需使用镜像站点加速下载，可设置环境变量："
    YELLOW "  export GITHUB_MIRROR='https://ghp.ci'"
    YELLOW "  export GITHUB_MIRROR='https://ghproxy.cc'"
    YELLOW "  export GITHUB_MIRROR='https://mirror.ghproxy.com'"
    echo ""
}

# 执行主函数
main
