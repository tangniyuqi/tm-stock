#!/bin/bash
# GVA反馈转接核心覆盖率首版棘轮85%，不得靠删除用例或降低下限过门禁。
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT/backend/server"
OUT="$(go test -race ./service/feedbackconsole -count=1 -cover)"
VALUE="$(printf '%s\n' "$OUT" | sed -nE 's/.*coverage: ([0-9]+(\.[0-9]+)?)% of statements.*/\1/p')"
if [ -z "$VALUE" ] || ! awk -v n="$VALUE" 'BEGIN{exit !(n>=85)}'; then
  echo '[X] 反馈后台核心覆盖率低于85%或未取得真实计数'; exit 1
fi
echo "[OK] 反馈后台转接核心覆盖率 $VALUE%（下限85%）"
