#!/bin/bash
# =============================================================================
# Stop hook：回合结束提醒（Harness L1，非阻塞）
# 工作区有未验证的 Go/前端改动时，提示跑质量门禁。
# ★ 非阻塞设计：只提醒不拦截，遵循「批量里程碑」——不打断连续开发节奏。
#   输出走 stdout 的 systemMessage JSON（exit 0 时 stderr 会被丢弃）。
# =============================================================================

set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT" || exit 0

CHANGED="$(git status --porcelain 2>/dev/null | grep -cE '\.(go|ts|vue|js)$' || true)"

if [ "${CHANGED:-0}" -gt 0 ]; then
  printf '{"systemMessage": "⚠️ 工作区有 %s 个代码文件改动未验证。里程碑完成前请跑质量门禁：bash scripts/harness-checks.sh"}\n' "$CHANGED"
fi

exit 0
