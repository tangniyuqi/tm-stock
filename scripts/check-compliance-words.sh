#!/bin/bash
# =============================================================================
# 合规禁用词检查 —— 把「合规红线」变成机器门禁
# 用法：
#   bash scripts/check-compliance-words.sh            # 检查暂存区新增行（pre-commit）
#   bash scripts/check-compliance-words.sh --all      # 全量检查用户可见文案（门禁）
#   bash scripts/check-compliance-words.sh <file>     # 检查单个文件（L1 hook）
#
# ★ 为什么要机器检查：合规靠人记必然遗漏，能机检的就不要靠自觉。
# ★ 检查范围只含【用户可见文案】：pages/ server/ static/ hybrid/
#   排除规范文档（.claude/ .kiro/ docs/ scripts/）——它们必然包含这些词（是在定义禁令）。
# =============================================================================
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
WORDS_FILE="$REPO_ROOT/scripts/compliance-forbidden-words.txt"
MODE="${1:-staged}"

[ -f "$WORDS_FILE" ] || { echo "[跳过] 未找到词表 $WORDS_FILE"; exit 0; }

# 载入词表（去注释与空行）
WORDS="$(grep -vE '^[[:space:]]*(#|$)' "$WORDS_FILE" || true)"
[ -z "$WORDS" ] && { echo "[跳过] 词表为空"; exit 0; }

HITS=""
if [ "$MODE" = "--all" ]; then
  for d in pages server static hybrid; do
    [ -d "$REPO_ROOT/$d" ] || continue
    FOUND="$(grep -rnF "$WORDS" "$REPO_ROOT/$d" \
        --include='*.uvue' --include='*.uts' --include='*.go' \
        --include='*.json' --include='*.html' --include='*.js' --include='*.vue' \
        --exclude-dir=node_modules --exclude-dir=uni_modules 2>/dev/null || true)"
    [ -n "$FOUND" ] && HITS="$HITS$FOUND\n"
  done
elif [ "$MODE" = "staged" ]; then
  STAGED="$(git diff --cached --name-only --diff-filter=ACM 2>/dev/null \
      | grep -E '^(pages|server|static|hybrid)/' | grep -E '\.(uvue|uts|go|json|html|js|vue)$' || true)"
  for f in $STAGED; do
    [ -f "$REPO_ROOT/$f" ] || continue
    FOUND="$(grep -nF "$WORDS" "$REPO_ROOT/$f" 2>/dev/null | sed "s|^|$f:|" || true)"
    [ -n "$FOUND" ] && HITS="$HITS$FOUND\n"
  done
else
  # 单文件模式
  TARGET="$MODE"
  [ -f "$TARGET" ] || exit 0
  case "$TARGET" in
    */.claude/*|*/.kiro/*|*/docs/*|*/scripts/*|*uni_modules*|*node_modules*) exit 0 ;;
  esac
  case "$TARGET" in
    *.uvue|*.uts|*.go|*.json|*.html|*.js|*.vue)
      HITS="$(grep -nF "$WORDS" "$TARGET" 2>/dev/null || true)" ;;
    *) exit 0 ;;
  esac
fi

if [ -n "$(printf '%b' "$HITS" | tr -d '[:space:]')" ]; then
  echo "[X] 检测到合规禁用词（用户可见文案不得出现）：" >&2
  printf '%b' "$HITS" | head -15 >&2
  echo "" >&2
  echo "[依据] .claude/agents/compliance-redline.md —— 荐股认定采功能实质测试，免责声明不能豁免" >&2
  echo "[修复] 改成客观中性表述；描述事实而非给出评价/预测/建议" >&2
  echo "[红线] 禁止为了过门禁而从词表里删词（删词须有律师意见）" >&2
  exit 1
fi

echo "[OK] 未发现合规禁用词"
exit 0
