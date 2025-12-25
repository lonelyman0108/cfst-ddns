#!/usr/bin/env bash

# 简单的 CHANGELOG 更新脚本（不需要额外工具）
# 使用方法: ./simple-changelog.sh v1.2.0

set -e

if [ -z "$1" ]; then
    echo "使用方法: $0 <version>"
    echo "示例: $0 v1.2.0"
    exit 1
fi

VERSION="$1"
VERSION_NUMBER="${VERSION#v}"
DATE=$(date +%Y-%m-%d)

echo "准备为版本 $VERSION_NUMBER 更新 CHANGELOG.md"

# 获取远程仓库 URL 并转换为 GitHub URL
REMOTE_URL=$(git config --get remote.origin.url)
if [[ "$REMOTE_URL" =~ git@github.com:(.+)\.git ]]; then
    REPO_PATH="${BASH_REMATCH[1]}"
elif [[ "$REMOTE_URL" =~ github.com/(.+)\.git ]]; then
    REPO_PATH="${BASH_REMATCH[1]}"
elif [[ "$REMOTE_URL" =~ github.com/(.+) ]]; then
    REPO_PATH="${BASH_REMATCH[1]}"
else
    echo "警告: 无法从远程 URL 提取仓库路径: $REMOTE_URL"
    REPO_PATH=""
fi

GITHUB_URL="https://github.com/${REPO_PATH}"

# 获取当前分支
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

# 根据当前分支确定提交范围
if [ "$CURRENT_BRANCH" = "dev" ]; then
    # 在 dev 分支上，只统计相对于 main 分支的新提交
    echo "当前在 dev 分支，统计相对于 main 分支的新提交"

    # 确保有 main 分支的最新信息
    git fetch origin main:refs/remotes/origin/main 2>/dev/null || true

    # 检查 main 分支是否存在
    if git rev-parse --verify origin/main >/dev/null 2>&1; then
        COMMIT_RANGE="origin/main..HEAD"
        echo "提交范围: origin/main..HEAD"
    elif git rev-parse --verify main >/dev/null 2>&1; then
        COMMIT_RANGE="main..HEAD"
        echo "提交范围: main..HEAD"
    else
        echo "警告: 未找到 main 分支，使用所有提交"
        COMMIT_RANGE="HEAD"
    fi
else
    # 在其他分支（如 main）上，使用上一个 tag 作为基准
    LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

    if [ -z "$LAST_TAG" ]; then
        echo "警告: 未找到之前的 tag，将显示所有 commits"
        COMMIT_RANGE="HEAD"
    else
        echo "当前在 $CURRENT_BRANCH 分支，从 $LAST_TAG 以来的更新"
        COMMIT_RANGE="$LAST_TAG..HEAD"
    fi
fi

# 创建临时文件
TEMP_FILE=$(mktemp)

# 提取不同类型的 commits（包含 commit hash）
echo "## [$VERSION_NUMBER] - $DATE" > "$TEMP_FILE"
echo "" >> "$TEMP_FILE"

# 用于跟踪是否已添加任何内容
HAS_CONTENT=false

# 新功能
FEATURES=$(git log $COMMIT_RANGE --pretty=format:"- %s (%h)" --grep="^feat:" --grep="^feat(" -i | grep -v "^chore.*CHANGELOG" | grep -v "^docs.*CHANGELOG" || true)
if [ -n "$FEATURES" ]; then
    [ "$HAS_CONTENT" = true ] && echo "" >> "$TEMP_FILE"
    echo "### Added" >> "$TEMP_FILE"
    echo "$FEATURES" >> "$TEMP_FILE"
    HAS_CONTENT=true
fi

# Bug 修复
FIXES=$(git log $COMMIT_RANGE --pretty=format:"- %s (%h)" --grep="^fix:" --grep="^fix(" -i | grep -v "^chore.*CHANGELOG" | grep -v "^docs.*CHANGELOG" || true)
if [ -n "$FIXES" ]; then
    [ "$HAS_CONTENT" = true ] && echo "" >> "$TEMP_FILE"
    echo "### Fixed" >> "$TEMP_FILE"
    echo "$FIXES" >> "$TEMP_FILE"
    HAS_CONTENT=true
fi

# 变更
CHANGES=$(git log $COMMIT_RANGE --pretty=format:"- %s (%h)" --grep="^update:" --grep="^update(" --grep="^change:" --grep="^change(" --grep="^refactor:" --grep="^refactor(" -i | grep -v "^chore.*CHANGELOG" | grep -v "^docs.*CHANGELOG" || true)
if [ -n "$CHANGES" ]; then
    [ "$HAS_CONTENT" = true ] && echo "" >> "$TEMP_FILE"
    echo "### Changed" >> "$TEMP_FILE"
    echo "$CHANGES" >> "$TEMP_FILE"
    HAS_CONTENT=true
fi

# 文档更新
DOCS=$(git log $COMMIT_RANGE --pretty=format:"- %s (%h)" --grep="^docs:" --grep="^docs(" -i | grep -v "^chore.*CHANGELOG" | grep -v "^docs.*CHANGELOG" || true)
if [ -n "$DOCS" ]; then
    [ "$HAS_CONTENT" = true ] && echo "" >> "$TEMP_FILE"
    echo "### Documentation" >> "$TEMP_FILE"
    echo "$DOCS" >> "$TEMP_FILE"
    HAS_CONTENT=true
fi

echo ""
echo "=== 生成的更新内容 ==="
cat "$TEMP_FILE"

echo ""
echo "========================"
read -p "是否将此内容添加到 CHANGELOG.md? (y/N) " -n 1 -r
echo

if [[ $REPLY =~ ^[Yy]$ ]]; then
    # 检查 CHANGELOG.md 是否存在
    if [ ! -f "CHANGELOG.md" ]; then
        echo "# Changelog" > CHANGELOG.md
        echo "" >> CHANGELOG.md
        echo "All notable changes to this project will be documented in this file." >> CHANGELOG.md
        echo "" >> CHANGELOG.md
        echo "The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)," >> CHANGELOG.md
        echo "and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)." >> CHANGELOG.md
        echo "" >> CHANGELOG.md
        echo "## [Unreleased]" >> CHANGELOG.md
        echo "" >> CHANGELOG.md
    fi

    # 备份
    cp CHANGELOG.md CHANGELOG.md.bak

    # 在 CHANGELOG.md 中找到 ## [Unreleased] 或第一个 ## [ 的位置插入
    if grep -q "## \[Unreleased\]" CHANGELOG.md; then
        # 在 Unreleased 后面插入
        sed '/^## \[Unreleased\]/r /dev/stdin' CHANGELOG.md < <(echo ""; cat "$TEMP_FILE") > CHANGELOG.md.new
        mv CHANGELOG.md.new CHANGELOG.md
    else
        # 在第一个版本之前插入
        if grep -q "## \[" CHANGELOG.md; then
            # 找到第一个版本号的行号，在它之前插入
            LINE_NUM=$(grep -n "^## \[" CHANGELOG.md | head -1 | cut -d: -f1)
            if [ -n "$LINE_NUM" ]; then
                head -n $((LINE_NUM - 1)) CHANGELOG.md > CHANGELOG.md.new
                cat "$TEMP_FILE" >> CHANGELOG.md.new
                echo "" >> CHANGELOG.md.new
                tail -n +$LINE_NUM CHANGELOG.md >> CHANGELOG.md.new
                mv CHANGELOG.md.new CHANGELOG.md
            else
                # 直接追加到文件末尾
                echo "" >> CHANGELOG.md
                cat "$TEMP_FILE" >> CHANGELOG.md
            fi
        else
            # 直接追加到文件末尾
            echo "" >> CHANGELOG.md
            cat "$TEMP_FILE" >> CHANGELOG.md
        fi
    fi

    # 更新底部的链接部分
    # 删除现有的链接部分（从第一个 [链接]: 开始到文件末尾）
    sed '/^\[.*\]:/,$d' CHANGELOG.md > CHANGELOG.md.new

    # 删除文件末尾的空行：从后往前删除空行直到遇到非空行
    while [ -s CHANGELOG.md.new ] && tail -1 CHANGELOG.md.new | grep -q '^[[:space:]]*$'; do
        sed -i '' '$d' CHANGELOG.md.new
    done

    mv CHANGELOG.md.new CHANGELOG.md

    # 添加新的链接部分
    if [ -n "$REPO_PATH" ]; then
        echo "" >> CHANGELOG.md

        # 获取所有版本号（按时间倒序）
        ALL_VERSIONS=($(grep -o '## \[[0-9.]*\]' CHANGELOG.md | sed 's/## \[\(.*\)\]/\1/' | head -20))

        # 生成 Unreleased 链接
        if [ ${#ALL_VERSIONS[@]} -gt 0 ]; then
            LATEST_VERSION="${ALL_VERSIONS[0]}"
            echo "[Unreleased]: ${GITHUB_URL}/compare/v${LATEST_VERSION}...HEAD" >> CHANGELOG.md
        fi

        # 生成版本比较链接
        for i in "${!ALL_VERSIONS[@]}"; do
            CURRENT_VER="${ALL_VERSIONS[$i]}"
            if [ $i -eq $((${#ALL_VERSIONS[@]} - 1)) ]; then
                # 最后一个版本，链接到该版本的 release
                echo "[${CURRENT_VER}]: ${GITHUB_URL}/releases/tag/v${CURRENT_VER}" >> CHANGELOG.md
            else
                # 其他版本，链接到版本比较
                NEXT_VER="${ALL_VERSIONS[$((i + 1))]}"
                echo "[${CURRENT_VER}]: ${GITHUB_URL}/compare/v${NEXT_VER}...v${CURRENT_VER}" >> CHANGELOG.md
            fi
        done
    fi

    rm "$TEMP_FILE"

    echo "✓ CHANGELOG.md 已更新"
    echo "✓ 备份保存在 CHANGELOG.md.bak"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📋 建议的 Commit Message (可直接复制):"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "docs: update CHANGELOG for $VERSION"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "下一步操作:"
    echo ""
    echo "1. 查看并编辑 CHANGELOG.md (可选)"
    echo "   vim CHANGELOG.md"
    echo ""
    echo "2. 提交更改并创建 tag"
    echo "   git add CHANGELOG.md"
    echo "   git commit -m 'docs: update CHANGELOG for $VERSION'"
    echo "   git tag $VERSION"
    echo "   git push origin main $VERSION"
    echo ""
    echo "或者一键执行 (Ctrl+C 可中断):"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "git add CHANGELOG.md && git commit -m 'docs: update CHANGELOG for $VERSION' && git tag $VERSION && git push origin main $VERSION"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
    rm "$TEMP_FILE"
    echo "已取消"
fi
