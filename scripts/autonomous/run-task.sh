#!/bin/bash
# =============================================================================
# 任务执行器（半自主）—— 取一个任务 → 建分支 → 交给 AI → 独立复验 → 提 PR
# 用法：
#   bash scripts/autonomous/run-task.sh --dry-run     # 干跑：只挑任务不执行
#   bash scripts/autonomous/run-task.sh               # 实际执行
#
# 🔴 红线（写死在脚本里，不要改）：
#   1. 只推 feature 分支 + 开 PR，绝不自动 merge 到 main
#   2. 复验不过就标 failed 等人介入，不允许"自我宣布成功"
#   3. 超过 max_retry 不再重试（防死循环烧钱）
# =============================================================================
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"
DRY=false
[ "${1:-}" = "--dry-run" ] && DRY=true

# ★ 剥离行内 # 注释与尾随空格（与 validate-task.sh 同源，别只改一处）
val() { sed -n "s/^$2:[[:space:]]*//p" "$1" | head -1 | sed 's/[[:space:]]*#.*$//' | tr -d "\r" | sed 's/[[:space:]]*$//'; }

# ── 挑选任务：status=pending，按 priority 升序，依赖已完成 ──
PICK=""
for f in $(ls tasks/[0-9]*.md 2>/dev/null | sort); do
  [ "$(val "$f" status)" = "pending" ] || continue
  DEPS="$(val "$f" deps)"
  if [ -n "$DEPS" ] && [ "$DEPS" != "[]" ]; then
    BLOCKED=false
    for d in $(echo "$DEPS" | tr -d "[]," ); do
      DF="$(ls tasks/${d}*.md 2>/dev/null | head -1)"
      [ -n "$DF" ] && [ "$(val "$DF" status)" != "done" ] && BLOCKED=true
    done
    $BLOCKED && continue
  fi
  PICK="$f"; break
done

[ -z "$PICK" ] && { echo "[空闲] 队列中没有可执行的 pending 任务"; exit 0; }

ID="$(val "$PICK" id)"; TITLE="$(val "$PICK" title)"
BRANCH="$(val "$PICK" branch)"; VL="$(val "$PICK" verify_level)"
echo "[选中] $ID —— $TITLE"
echo "  分支: $BRANCH   复验级别: $VL"

# 先校验任务卡
bash scripts/autonomous/validate-task.sh "$PICK" || { echo "[X] 任务卡不合格，跳过"; exit 1; }

if $DRY; then
  echo ""
  echo "[干跑] 若实际执行将："
  echo "  1) git checkout -b $BRANCH"
  echo "  2) 交给 AI 按任务卡实施（人工或 CI 触发）"
  echo "  3) 独立复验：$(verify_desc() { case "$1" in build) echo "go build";; test) echo "go test -race";; harness) echo "harness-checks.sh";; web) echo "pages.json 校验";; custom) echo "自定义命令";; *) echo "无";; esac; }; verify_desc "$VL")"
  echo "  4) 复验绿 → 提交 + 推 feature 分支 + 开 PR（绝不自动 merge）"
  echo "  5) 复验红 → status=failed，等人介入"
  exit 0
fi

echo ""
echo "[提示] 本脚本负责【挑选 / 校验 / 复验 / 收尾】，实际编码由 AI 会话完成。"
echo "       请在新会话中执行该任务卡，完成后回到这里跑复验："
echo ""
echo "  bash scripts/autonomous/run-task.sh --verify $PICK"
echo ""

# ── 复验模式 ──
if [ "${1:-}" = "--verify" ] && [ -n "${2:-}" ]; then
  T="$2"; VL="$(val "$T" verify_level)"
  case "$VL" in
    build)   (cd server && go build ./...) ;;
    test)    (cd server && go test ./... -race -count=1) ;;
    harness) bash scripts/harness-checks.sh ;;
    web)     node -e "JSON.parse(require('fs').readFileSync('pages.json','utf8'))" ;;
    custom)  eval "$(val "$T" verify_command)" ;;
    none)    echo "[跳过复验] verify_level=none" ;;
  esac
  RC=$?
  [ $RC -eq 0 ] && echo "[复验通过] 可提交并开 PR（禁止直接 merge 到 main）" \
                || echo "[复验失败] 请把该任务 status 改为 failed 并等人介入"
  exit $RC
fi
