#!/usr/bin/env bash
set -euo pipefail

git rev-parse --show-toplevel >/dev/null 2>&1
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

DATE_STR="$(date +%F)"
OUT="docs/plans/${DATE_STR}-upstream-sync-review.md"
mkdir -p docs/plans

HEAD_SHA="$(git rev-parse HEAD)"
BRANCH="$(git branch --show-current)"
LATEST_TAG="$(git tag --merged upstream/main --sort=-version:refname | head -1)"
LOCAL_VERSION="$(cat backend/cmd/server/VERSION 2>/dev/null || echo 'N/A')"
UPSTREAM_VERSION="$(git show upstream/main:backend/cmd/server/VERSION 2>/dev/null || echo 'N/A')"

if git rev-parse HEAD^2 >/dev/null 2>&1; then
  LOCAL_BASELINE="$(git rev-parse HEAD^1)"
  UPSTREAM_SHA="$(git rev-parse HEAD^2)"
  MERGE_BASE="$(git merge-base "$LOCAL_BASELINE" "$UPSTREAM_SHA")"
else
  LOCAL_BASELINE="$HEAD_SHA"
  UPSTREAM_SHA="$(git rev-parse upstream/main)"
  MERGE_BASE="$(git merge-base HEAD upstream/main)"
fi

cat > "$OUT" <<EOF
# 上游同步评审记录（${DATE_STR}）

## 基线

- 当前分支：\`${BRANCH}\`
- 当前 HEAD：\`${HEAD_SHA}\`
- 合入前本地基线：\`${LOCAL_BASELINE}\`
- 上游引用：\`upstream/main\`
- 上游 SHA：\`${UPSTREAM_SHA}\`
- merge-base：\`${MERGE_BASE}\`
- 同步方式：\`git merge upstream/main\`
- upstream 最新 release tag：\`${LATEST_TAG}\`
- upstream/main 版本文件：\`${UPSTREAM_VERSION}\`
- 本地最终 VERSION：\`${LOCAL_VERSION}\`

## 上游更新摘要

- 待补：概述本轮吸收的 feature / fix / docs / CI 变化
- 待补：列出命中的高风险模块

## 本地定制保护点

- 待补：品牌与文案
- 待补：部署/回滚/版本链路
- 待补：OpenAI OAuth / Codex 兼容
- 待补：后台入口与默认配置

## 冲突与取舍

- 待补：Git 冲突文件与处理方式
- 待补：无冲突但做过语义复核的热点文件

## 测试记录

- 待补：后端 unit / integration / lint
- 待补：前端 lint / typecheck / test
- 待补：版本链路与部署校验

## 剩余风险与观察点

- 待补：尚未验证的风险
- 待补：灰度/回滚建议

## 结论

- 待补：是否建议保留当前结果
EOF

printf '%s\n' "$OUT"
