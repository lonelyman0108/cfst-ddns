FROM alpine:latest

# 安装必要的依赖
RUN apk add --no-cache \
    bash \
    curl \
    jq \
    ca-certificates \
    tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建工作目录
WORKDIR /app

# 复制脚本文件
COPY cfst_ddns.sh /app/
COPY install.sh /app/
COPY config.example.sh /app/

# 设置执行权限
RUN chmod +x /app/cfst_ddns.sh /app/install.sh

# 创建 cfst 目录（安装脚本会下载到这里）
RUN mkdir -p /app/cfst

# 创建数据目录（用于持久化测速结果和配置）
RUN mkdir -p /app/data

# 环境变量：GitHub 镜像站点（可选）
ENV GITHUB_MIRROR=""

# 环境变量：是否在启动时自动安装 cfst
ENV AUTO_INSTALL_CFST="true"

# 环境变量：数据目录（用于持久化测速结果）
ENV DATA_DIR="/app/data"

# 环境变量：是否启用定时任务
ENV ENABLE_CRON="false"

# 环境变量：定时任务执行频率（cron 表达式）
ENV CRON_SCHEDULE="0 */6 * * *"

# 创建启动脚本
RUN echo '#!/bin/bash' > /app/entrypoint.sh && \
    echo 'set -e' >> /app/entrypoint.sh && \
    echo '' >> /app/entrypoint.sh && \
    echo '# 检查配置文件是否存在' >> /app/entrypoint.sh && \
    echo 'if [[ ! -f /app/data/config.sh ]]; then' >> /app/entrypoint.sh && \
    echo '    echo "========================================"' >> /app/entrypoint.sh && \
    echo '    echo "错误: 未找到配置文件"' >> /app/entrypoint.sh && \
    echo '    echo "========================================"' >> /app/entrypoint.sh && \
    echo '    echo ""' >> /app/entrypoint.sh && \
    echo '    echo "请按以下步骤配置："' >> /app/entrypoint.sh && \
    echo '    echo "1. 创建配置文件: cp config.example.sh data/config.sh"' >> /app/entrypoint.sh && \
    echo '    echo "2. 编辑配置文件: vim data/config.sh"' >> /app/entrypoint.sh && \
    echo '    echo "3. 填入 Cloudflare API 凭证和域名信息"' >> /app/entrypoint.sh && \
    echo '    echo ""' >> /app/entrypoint.sh && \
    echo '    echo "配置文件应位于: ./data/config.sh"' >> /app/entrypoint.sh && \
    echo '    echo "========================================"' >> /app/entrypoint.sh && \
    echo '    exit 1' >> /app/entrypoint.sh && \
    echo 'fi' >> /app/entrypoint.sh && \
    echo '' >> /app/entrypoint.sh && \
    echo '# 创建配置文件软链接' >> /app/entrypoint.sh && \
    echo 'if [[ ! -f /app/config.sh ]]; then' >> /app/entrypoint.sh && \
    echo '    echo "使用配置文件: /app/data/config.sh"' >> /app/entrypoint.sh && \
    echo '    ln -s /app/data/config.sh /app/config.sh' >> /app/entrypoint.sh && \
    echo 'fi' >> /app/entrypoint.sh && \
    echo '' >> /app/entrypoint.sh && \
    echo '# 如果启用自动安装且 cfst 不存在，则运行安装脚本' >> /app/entrypoint.sh && \
    echo 'if [[ "$AUTO_INSTALL_CFST" == "true" ]] && [[ ! -f /app/cfst/cfst ]]; then' >> /app/entrypoint.sh && \
    echo '    echo "正在安装 CloudflareSpeedTest..."' >> /app/entrypoint.sh && \
    echo '    cd /app && ./install.sh' >> /app/entrypoint.sh && \
    echo 'fi' >> /app/entrypoint.sh && \
    echo '' >> /app/entrypoint.sh && \
    echo '# 判断是否启用定时任务' >> /app/entrypoint.sh && \
    echo 'if [[ "$ENABLE_CRON" == "true" ]]; then' >> /app/entrypoint.sh && \
    echo '    echo "========================================"' >> /app/entrypoint.sh && \
    echo '    echo "启用定时任务模式"' >> /app/entrypoint.sh && \
    echo '    echo "========================================"' >> /app/entrypoint.sh && \
    echo '    echo "定时任务设置: ${CRON_SCHEDULE}"' >> /app/entrypoint.sh && \
    echo '    echo "${CRON_SCHEDULE} cd /app && ./cfst_ddns.sh >> /var/log/cfst-ddns.log 2>&1" | crontab -' >> /app/entrypoint.sh && \
    echo '    echo "定时任务已设置，crond 启动中..."' >> /app/entrypoint.sh && \
    echo '    exec crond -f -l 2' >> /app/entrypoint.sh && \
    echo 'else' >> /app/entrypoint.sh && \
    echo '    echo "========================================"' >> /app/entrypoint.sh && \
    echo '    echo "单次执行模式"' >> /app/entrypoint.sh && \
    echo '    echo "========================================"' >> /app/entrypoint.sh && \
    echo '    exec /app/cfst_ddns.sh' >> /app/entrypoint.sh && \
    echo 'fi' >> /app/entrypoint.sh && \
    chmod +x /app/entrypoint.sh

# 挂载点
VOLUME ["/app/data"]

# 启动脚本
ENTRYPOINT ["/app/entrypoint.sh"]
