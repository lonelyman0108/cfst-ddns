# syntax=docker/dockerfile:1
#
# 构建目标：
#   standard（默认）：首次启动时自动下载 cfst（国内网络请在设置中配置 GitHub 镜像）
#   bundled         ：镜像内预置 cfst，离线可用
#
#   docker build -t cfst-ddns .
#   docker build --target bundled -t cfst-ddns:bundled .

# ---------- 前端 ----------
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ---------- 后端（交叉编译，无 CGO） ----------
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH TARGETVARIANT
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web/embed.go ./web/embed.go
COPY --from=web /src/web/dist ./web/dist
RUN export CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH; \
    if [ "$TARGETARCH" = "arm" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
      -o /out/cfst-ddns ./cmd/cfst-ddns

# ---------- 预置 cfst（仅 bundled 使用） ----------
FROM --platform=$BUILDPLATFORM alpine:3.22 AS cfst
ARG TARGETARCH TARGETVARIANT
ARG CFST_VERSION=v2.3.5
# 压缩包内可能带一层目录，按文件名查找（旧版可执行文件名为 CloudflareST）
RUN set -eux -o pipefail; \
    case "$TARGETARCH" in \
      arm) arch="armv${TARGETVARIANT#v}" ;; \
      *)   arch="$TARGETARCH" ;; \
    esac; \
    mkdir -p /tmp/cfst /opt/cfst; \
    wget -qO- "https://github.com/XIU2/CloudflareSpeedTest/releases/download/${CFST_VERSION}/cfst_linux_${arch}.tar.gz" \
      | tar -xz -C /tmp/cfst; \
    bin=$(find /tmp/cfst -type f \( -name cfst -o -name CloudflareST \) | head -n 1); \
    [ -n "$bin" ]; \
    install -m 755 "$bin" /opt/cfst/cfst; \
    for f in ip.txt ipv6.txt; do \
      p=$(find /tmp/cfst -type f -name "$f" | head -n 1); \
      [ -z "$p" ] || cp "$p" /opt/cfst/; \
    done; \
    echo "$CFST_VERSION" > /opt/cfst/VERSION

# ---------- 运行时 ----------
FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai \
    CFST_DDNS_LISTEN=:8080 \
    CFST_DDNS_DATA=/app/data
COPY --from=build /out/cfst-ddns /usr/local/bin/cfst-ddns
WORKDIR /app
VOLUME ["/app/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["cfst-ddns", "healthcheck"]
ENTRYPOINT ["cfst-ddns"]
CMD ["serve"]

FROM runtime AS bundled
COPY --from=cfst /opt/cfst /opt/cfst
ENV CFST_DDNS_BUNDLE_DIR=/opt/cfst

# 默认目标放在最后
FROM runtime AS standard
