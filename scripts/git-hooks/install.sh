#!/bin/bash
# 安装 git hooks 到 .git/hooks（每个 clone / worktree 都要跑一次）
set -euo pipefail
REPO_ROOT="$(git rev-parse --show-toplevel)"
SRC="$REPO_ROOT/scripts/git-hooks"
DST="$(git rev-parse --git-path hooks)"
mkdir -p "$DST"
for h in pre-commit pre-push; do
  cp "$SRC/$h" "$DST/$h"
  chmod +x "$DST/$h"
  echo "✓ 已安装 $h"
done
echo "完成。绕过方式：git commit/push --no-verify（应视为异常）"
