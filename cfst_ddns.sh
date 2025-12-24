#!/usr/bin/env bash
PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:~/bin
export PATH

# --------------------------------------------------------------
# 项目: CloudflareSpeedTest 自动 DDNS 更新脚本
# 说明: 自动测速并将最优 IP 更新到 Cloudflare DNS 记录
# --------------------------------------------------------------

# =========================
# 加载配置文件
# =========================

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/config.sh"

# 如果存在 config.sh 则加载，否则使用下面的默认配置
if [[ -f "$CONFIG_FILE" ]]; then
    source "$CONFIG_FILE"
    echo "已加载配置文件: $CONFIG_FILE"
fi

# =========================
# 默认配置（如未使用 config.sh）
# =========================

# Cloudflare API 配置
CF_API_TOKEN="${CF_API_TOKEN:-}"              # Cloudflare API Token (推荐)
CF_API_KEY="${CF_API_KEY:-}"                  # Cloudflare Global API Key
CF_EMAIL="${CF_EMAIL:-}"                      # Cloudflare 账号邮箱
CF_ZONE_ID="${CF_ZONE_ID:-}"                  # Zone ID
CF_RECORD_NAMES="${CF_RECORD_NAMES:-}"        # 要更新的域名列表（空格分隔）

# CFST 测速配置
CFST_BIN="${CFST_BIN:-${SCRIPT_DIR}/cfst/cfst}"
CFST_PARAMS="${CFST_PARAMS:-}"
CFST_TEST_MODE="${CFST_TEST_MODE:-v4}"        # 测速模式: v4, v6, both
DATA_DIR="${DATA_DIR:-${SCRIPT_DIR}}"         # 数据目录（测速结果保存位置）
RESULT_FILE="${RESULT_FILE:-result_ddns.txt}"
SKIP_SPEED_TEST="${SKIP_SPEED_TEST:-false}"   # 是否跳过测速（使用已有结果）

# 通知配置
ENABLE_BARK="${ENABLE_BARK:-false}"           # 是否启用 Bark 通知
BARK_URL="${BARK_URL:-}"                      # Bark 推送地址
BARK_KEY="${BARK_KEY:-}"                      # Bark 密钥（可选）

ENABLE_TELEGRAM="${ENABLE_TELEGRAM:-false}"   # 是否启用 Telegram 通知
TG_BOT_TOKEN="${TG_BOT_TOKEN:-}"              # Telegram Bot Token
TG_CHAT_ID="${TG_CHAT_ID:-}"                  # Telegram Chat ID

# 全局变量
declare -a UPDATE_RESULTS                     # 存储更新结果
declare -a BEST_IPS                           # 存储最优 IP 列表（支持 v4+v6）

# =========================
# 函数定义
# =========================

# 颜色输出
_RED() { echo -e "\033[31m$1\033[0m"; }
_GREEN() { echo -e "\033[32m$1\033[0m"; }
_YELLOW() { echo -e "\033[33m$1\033[0m"; }
_BLUE() { echo -e "\033[36m$1\033[0m"; }

# Bark 通知
_SEND_BARK() {
    local title="$1"
    local content="$2"

    if [[ "$ENABLE_BARK" != "true" ]] || [[ -z "$BARK_URL" ]]; then
        return
    fi

    local url="$BARK_URL"
    [[ -n "$BARK_KEY" ]] && url="$url/$BARK_KEY"

    # 将 \n 转换为实际换行符，然后 URL 编码
    title=$(printf %s "$title" | jq -sRr @uri)
    content=$(echo -e "$content" | jq -sRr @uri)

    curl -s "${url}/${title}/${content}" > /dev/null
    _GREEN "✓ Bark 通知已发送"
}

# Telegram 通知
_SEND_TELEGRAM() {
    local message="$1"

    if [[ "$ENABLE_TELEGRAM" != "true" ]] || [[ -z "$TG_BOT_TOKEN" ]] || [[ -z "$TG_CHAT_ID" ]]; then
        return
    fi

    local api_url="https://api.telegram.org/bot${TG_BOT_TOKEN}/sendMessage"

    # 将 \n 转换为实际换行符
    message=$(echo -e "$message")

    curl -s -X POST "$api_url" \
        -d "chat_id=${TG_CHAT_ID}" \
        -d "text=${message}" \
        -d "parse_mode=HTML" > /dev/null

    _GREEN "✓ Telegram 通知已发送"
}

# 发送通知（支持多种方式）
_SEND_NOTIFICATION() {
    local title="$1"
    local content="$2"

    _BLUE "\n=== 发送通知 ==="

    # 发送 Bark 通知
    _SEND_BARK "$title" "$content"

    # 发送 Telegram 通知
    local tg_message="<b>$title</b>\n\n$content"
    _SEND_TELEGRAM "$tg_message"
}

# 检查配置
_CHECK_CONFIG() {
    _BLUE "=== 检查配置 ==="

    if [[ -z "$CF_ZONE_ID" ]] || [[ -z "$CF_RECORD_NAMES" ]]; then
        _RED "错误: 请先配置 CF_ZONE_ID 和 CF_RECORD_NAMES"
        exit 1
    fi

    if [[ -z "$CF_API_TOKEN" ]] && [[ -z "$CF_API_KEY" || -z "$CF_EMAIL" ]]; then
        _RED "错误: 请配置 CF_API_TOKEN 或 (CF_API_KEY + CF_EMAIL)"
        exit 1
    fi

    if [[ ! -x "$CFST_BIN" ]]; then
        _RED "错误: cfst 可执行文件不存在或无执行权限: $CFST_BIN"
        exit 1
    fi

    _GREEN "配置检查通过"
    _BLUE "待更新域名: $CF_RECORD_NAMES"
}

# 执行测速
_SPEED_TEST() {
    # 如果设置跳过测速，直接读取已有结果
    if [[ "$SKIP_SPEED_TEST" == "true" ]]; then
        _YELLOW "\n=== 跳过测速，使用已有结果 ==="

        # 根据测速模式加载对应的结果文件
        if [[ "$CFST_TEST_MODE" == "both" ]]; then
            # both 模式：加载 IPv4 和 IPv6
            if [[ -e "${DATA_DIR}/${RESULT_FILE}.v4" ]]; then
                local best_ipv4=$(sed -n "2,1p" "${DATA_DIR}/${RESULT_FILE}.v4" | awk -F, '{print $1}')
                if [[ -n "$best_ipv4" ]]; then
                    BEST_IPS+=("v4:$best_ipv4")
                    _GREEN "✓ 已加载 IPv4: $best_ipv4"
                fi
            else
                _YELLOW "! IPv4 结果文件不存在: ${DATA_DIR}/${RESULT_FILE}.v4"
            fi

            if [[ -e "${DATA_DIR}/${RESULT_FILE}.v6" ]]; then
                local best_ipv6=$(sed -n "2,1p" "${DATA_DIR}/${RESULT_FILE}.v6" | awk -F, '{print $1}')
                if [[ -n "$best_ipv6" ]]; then
                    BEST_IPS+=("v6:$best_ipv6")
                    _GREEN "✓ 已加载 IPv6: $best_ipv6"
                fi
            else
                _YELLOW "! IPv6 结果文件不存在: ${DATA_DIR}/${RESULT_FILE}.v6"
            fi

            if [[ ${#BEST_IPS[@]} -eq 0 ]]; then
                _RED "错误: 未找到任何可用的测速结果"
                _RED "请先执行一次完整测速或设置 SKIP_SPEED_TEST=false"
                exit 1
            fi
        else
            # 单一类型模式：根据 CFST_TEST_MODE 加载
            local required_file
            if [[ "$CFST_TEST_MODE" == "v6" ]]; then
                required_file="${DATA_DIR}/${RESULT_FILE}.v6"
            else
                required_file="${DATA_DIR}/${RESULT_FILE}.v4"
            fi

            if [[ ! -e "$required_file" ]]; then
                _RED "错误: 结果文件不存在: $required_file"
                _RED "请先执行一次完整测速或设置 SKIP_SPEED_TEST=false"
                exit 1
            fi

            _LOAD_SPEED_RESULT
        fi
        return
    fi

    _BLUE "\n=== 开始 Cloudflare IP 测速 ==="
    _BLUE "测速模式: $CFST_TEST_MODE"

    # 清理旧的测速结果
    rm -f "${DATA_DIR}/${RESULT_FILE}" "${DATA_DIR}/${RESULT_FILE}.v4" "${DATA_DIR}/${RESULT_FILE}.v6"

    # 根据测速模式选择要测的 IP 类型
    case "$CFST_TEST_MODE" in
        v4)
            _BLUE "仅测速 IPv4"
            _TEST_IPV4
            ;;
        v6)
            _BLUE "仅测速 IPv6"
            _TEST_IPV6
            ;;
        both)
            _BLUE "同时测速 IPv4 和 IPv6"
            _TEST_IPV4
            _TEST_IPV6
            ;;
        *)
            _RED "错误: 无效的测速模式: $CFST_TEST_MODE"
            _RED "支持的模式: v4, v6, both"
            exit 1
            ;;
    esac

    _LOAD_SPEED_RESULT
}

# 测速 IPv4
_TEST_IPV4() {
    local ip_file="./cfst/ip.txt"
    local result_file="${DATA_DIR}/${RESULT_FILE}.v4"

    _BLUE "→ 测速 IPv4..."
    $CFST_BIN -f "$ip_file" -o "$result_file" $CFST_PARAMS

    if [[ ! -e "$result_file" ]]; then
        _RED "错误: IPv4 测速失败，未生成结果文件"
        return 1
    fi

    local best_ipv4=$(sed -n "2,1p" "$result_file" | awk -F, '{print $1}')
    if [[ -n "$best_ipv4" ]]; then
        _GREEN "✓ IPv4 最优 IP: $best_ipv4"
        BEST_IPS+=("v4:$best_ipv4")
    else
        _YELLOW "! IPv4 未找到可用 IP"
    fi
}

# 测速 IPv6
_TEST_IPV6() {
    local ip_file="./cfst/ipv6.txt"
    local result_file="${DATA_DIR}/${RESULT_FILE}.v6"

    _BLUE "→ 测速 IPv6..."
    $CFST_BIN -f "$ip_file" -o "$result_file" $CFST_PARAMS

    if [[ ! -e "$result_file" ]]; then
        _RED "错误: IPv6 测速失败，未生成结果文件"
        return 1
    fi

    local best_ipv6=$(sed -n "2,1p" "$result_file" | awk -F, '{print $1}')
    if [[ -n "$best_ipv6" ]]; then
        _GREEN "✓ IPv6 最优 IP: $best_ipv6"
        BEST_IPS+=("v6:$best_ipv6")
    else
        _YELLOW "! IPv6 未找到可用 IP"
    fi
}

# 加载测速结果
_LOAD_SPEED_RESULT() {
    # 根据测速模式选择对应的最优 IP
    if [[ "$CFST_TEST_MODE" == "v6" ]]; then
        # 查找 IPv6 结果
        for ip_entry in "${BEST_IPS[@]}"; do
            if [[ "$ip_entry" == v6:* ]]; then
                BEST_IP="${ip_entry#v6:}"
                break
            fi
        done
        if [[ -z "$BEST_IP" ]] && [[ -e "${DATA_DIR}/${RESULT_FILE}.v6" ]]; then
            BEST_IP=$(sed -n "2,1p" "${DATA_DIR}/${RESULT_FILE}.v6" | awk -F, '{print $1}')
        fi
    else
        # v4 模式：查找 IPv4 结果
        for ip_entry in "${BEST_IPS[@]}"; do
            if [[ "$ip_entry" == v4:* ]]; then
                BEST_IP="${ip_entry#v4:}"
                break
            fi
        done
        if [[ -z "$BEST_IP" ]] && [[ -e "${DATA_DIR}/${RESULT_FILE}.v4" ]]; then
            BEST_IP=$(sed -n "2,1p" "${DATA_DIR}/${RESULT_FILE}.v4" | awk -F, '{print $1}')
        fi
    fi

    if [[ -z "$BEST_IP" ]]; then
        _RED "错误: 未找到可用的 IP (模式: $CFST_TEST_MODE)"
        exit 1
    fi

    _GREEN "\n测速完成，当前使用 IP ($CFST_TEST_MODE): $BEST_IP"
}

# 获取 DNS 记录 ID
_GET_RECORD_ID() {
    local record_name="$1"
    local record_type="$2"
    _BLUE "\n=== 获取 DNS 记录: $record_name ($record_type) ==="

    if [[ -n "$CF_API_TOKEN" ]]; then
        RESPONSE=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records?type=$record_type&name=$record_name" \
            -H "Authorization: Bearer $CF_API_TOKEN" \
            -H "Content-Type: application/json")
    else
        RESPONSE=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records?type=$record_type&name=$record_name" \
            -H "X-Auth-Email: $CF_EMAIL" \
            -H "X-Auth-Key: $CF_API_KEY" \
            -H "Content-Type: application/json")
    fi

    # 检查 API 响应
    SUCCESS=$(echo "$RESPONSE" | grep -o '"success":\w*' | cut -d: -f2)
    if [[ "$SUCCESS" != "true" ]]; then
        _RED "错误: API 请求失败"
        echo "$RESPONSE"
        return 1
    fi

    # 获取记录 ID 和当前 IP
    RECORD_ID=$(echo "$RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    CURRENT_IP=$(echo "$RESPONSE" | grep -o '"content":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [[ -z "$RECORD_ID" ]]; then
        _YELLOW "警告: DNS 记录不存在，将创建新记录"
        CREATE_NEW=true
    else
        _GREEN "找到 DNS 记录，当前 IP: $CURRENT_IP"
        CREATE_NEW=false
    fi
    return 0
}

# 更新单个 DNS 记录
_UPDATE_SINGLE_DNS() {
    local record_name="$1"
    local record_type="$2"
    local best_ip="$3"
    _BLUE "\n=== 更新 DNS 记录: $record_name ($record_type) ==="

    # 获取记录信息
    _GET_RECORD_ID "$record_name" "$record_type" || return 1

    # 如果 IP 相同则跳过
    if [[ "$CURRENT_IP" == "$best_ip" ]] && [[ "$CREATE_NEW" == "false" ]]; then
        _GREEN "当前 DNS 记录已是最优 IP，无需更新"
        UPDATE_RESULTS+=("✓ $record_name ($record_type): 无需更新 (已是最优IP)")
        return 0
    fi

    # 准备 JSON 数据
    JSON_DATA=$(cat <<EOF
{
  "type": "$record_type",
  "name": "$record_name",
  "content": "$best_ip",
  "ttl": 1,
  "proxied": false
}
EOF
)

    if [[ "$CREATE_NEW" == "true" ]]; then
        # 创建新记录
        if [[ -n "$CF_API_TOKEN" ]]; then
            RESPONSE=$(curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records" \
                -H "Authorization: Bearer $CF_API_TOKEN" \
                -H "Content-Type: application/json" \
                --data "$JSON_DATA")
        else
            RESPONSE=$(curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records" \
                -H "X-Auth-Email: $CF_EMAIL" \
                -H "X-Auth-Key: $CF_API_KEY" \
                -H "Content-Type: application/json" \
                --data "$JSON_DATA")
        fi
        ACTION="创建"
    else
        # 更新现有记录
        if [[ -n "$CF_API_TOKEN" ]]; then
            RESPONSE=$(curl -s -X PUT "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records/$RECORD_ID" \
                -H "Authorization: Bearer $CF_API_TOKEN" \
                -H "Content-Type: application/json" \
                --data "$JSON_DATA")
        else
            RESPONSE=$(curl -s -X PUT "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records/$RECORD_ID" \
                -H "X-Auth-Email: $CF_EMAIL" \
                -H "X-Auth-Key: $CF_API_KEY" \
                -H "Content-Type: application/json" \
                --data "$JSON_DATA")
        fi
        ACTION="更新"
    fi

    # 检查更新结果
    SUCCESS=$(echo "$RESPONSE" | grep -o '"success":\w*' | cut -d: -f2)
    if [[ "$SUCCESS" == "true" ]]; then
        _GREEN "${ACTION}成功！"
        _GREEN "域名: $record_name"
        _GREEN "类型: $record_type"
        _GREEN "原IP: ${CURRENT_IP:-无}"
        _GREEN "新IP: $best_ip"
        UPDATE_RESULTS+=("✓ $record_name ($record_type): $ACTION成功 $CURRENT_IP → $best_ip")
        return 0
    else
        _RED "${ACTION}失败！"
        local error_msg=$(echo "$RESPONSE" | grep -o '"message":"[^"]*"' | cut -d'"' -f4)
        echo "$error_msg"
        UPDATE_RESULTS+=("✗ $record_name ($record_type): ${ACTION}失败 - $error_msg")
        return 1
    fi
}

# 更新所有 DNS 记录
_UPDATE_ALL_DNS() {
    _BLUE "\n=== 开始更新所有域名 ==="

    local success_count=0
    local fail_count=0

    # 如果测速模式为 both，同时更新 A 和 AAAA 记录
    if [[ "$CFST_TEST_MODE" == "both" ]]; then
        _BLUE "检测到测速模式为 both，将同时更新 A 和 AAAA 记录"

        for record_name in $CF_RECORD_NAMES; do
            # 更新 A 记录 (IPv4)
            local ipv4
            for ip_entry in "${BEST_IPS[@]}"; do
                if [[ "$ip_entry" == v4:* ]]; then
                    ipv4="${ip_entry#v4:}"
                    break
                fi
            done

            if [[ -n "$ipv4" ]]; then
                if _UPDATE_SINGLE_DNS "$record_name" "A" "$ipv4"; then
                    ((success_count++))
                else
                    ((fail_count++))
                fi
            fi

            # 更新 AAAA 记录 (IPv6)
            local ipv6
            for ip_entry in "${BEST_IPS[@]}"; do
                if [[ "$ip_entry" == v6:* ]]; then
                    ipv6="${ip_entry#v6:}"
                    break
                fi
            done

            if [[ -n "$ipv6" ]]; then
                if _UPDATE_SINGLE_DNS "$record_name" "AAAA" "$ipv6"; then
                    ((success_count++))
                else
                    ((fail_count++))
                fi
            fi
        done
    else
        # 单一类型记录更新
        local record_type
        if [[ "$CFST_TEST_MODE" == "v6" ]]; then
            record_type="AAAA"
        else
            record_type="A"
        fi

        for record_name in $CF_RECORD_NAMES; do
            if _UPDATE_SINGLE_DNS "$record_name" "$record_type" "$BEST_IP"; then
                ((success_count++))
            else
                ((fail_count++))
            fi
        done
    fi

    _BLUE "\n=== 更新汇总 ==="
    _GREEN "成功: $success_count"
    _RED "失败: $fail_count"
}

# 清理临时文件
_CLEANUP() {
    # 只在实际进行了测速时显示保存信息
    if [[ "$SKIP_SPEED_TEST" != "true" ]]; then
        _BLUE "\n=== 保留测速结果文件 ==="

        # 保留测速结果文件用于下次快速测试
        if [[ -e "${DATA_DIR}/${RESULT_FILE}.v4" ]]; then
            _GREEN "✓ IPv4 测速结果已保存: ${DATA_DIR}/${RESULT_FILE}.v4"
        fi

        if [[ -e "${DATA_DIR}/${RESULT_FILE}.v6" ]]; then
            _GREEN "✓ IPv6 测速结果已保存: ${DATA_DIR}/${RESULT_FILE}.v6"
        fi

        _YELLOW "提示: 下次可设置 SKIP_SPEED_TEST=true 跳过测速，直接使用已保存结果"
    fi
}

# =========================
# 主流程
# =========================

main() {
    echo "========================================"
    echo " CloudflareSpeedTest DDNS 自动更新脚本"
    echo "========================================"

    _CHECK_CONFIG
    _SPEED_TEST
    _UPDATE_ALL_DNS
    _CLEANUP

    # 生成通知内容
    local notify_title="CFST DDNS 更新完成"

    # 构建更清晰的通知内容
    local notify_content=""

    # 如果是 both 模式，显示两个 IP
    if [[ "$CFST_TEST_MODE" == "both" ]]; then
        local ipv4=""
        local ipv6=""
        for ip_entry in "${BEST_IPS[@]}"; do
            if [[ "$ip_entry" == v4:* ]]; then
                ipv4="${ip_entry#v4:}"
            elif [[ "$ip_entry" == v6:* ]]; then
                ipv6="${ip_entry#v6:}"
            fi
        done

        if [[ -n "$ipv4" && -n "$ipv6" ]]; then
            notify_content="最优 IPv4: ${ipv4}\n最优 IPv6: ${ipv6}\n"
        elif [[ -n "$ipv4" ]]; then
            notify_content="最优 IPv4: ${ipv4}\n"
        elif [[ -n "$ipv6" ]]; then
            notify_content="最优 IPv6: ${ipv6}\n"
        fi
    else
        notify_content="最优 IP: $BEST_IP\n"
    fi

    notify_content="${notify_content}\n更新结果:\n"
    for result in "${UPDATE_RESULTS[@]}"; do
        notify_content="${notify_content}${result}\n"
    done
    notify_content="${notify_content}\n执行时间: $(date '+%Y-%m-%d %H:%M:%S')"

    # 发送通知
    _SEND_NOTIFICATION "$notify_title" "$notify_content"

    _BLUE "\n=== 完成 ==="
    echo "执行时间: $(date '+%Y-%m-%d %H:%M:%S')"
}

# 执行主函数
main
