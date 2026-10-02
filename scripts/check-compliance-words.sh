#!/bin/bash
# =============================================================================
# 合规禁用词检查 —— 把「合规红线」变成机器门禁
# 用法：
#   bash scripts/check-compliance-words.sh            # 检查暂存区新增行（pre-commit）
#   bash scripts/check-compliance-words.sh --all      # 全量检查用户可见文案（门禁）
#   bash scripts/check-compliance-words.sh <file>     # 检查单个文件（L1 hook；绝对或相对路径均可）
#
# ★ 为什么要机器检查：合规靠人记必然遗漏，能机检的就不要靠自觉。
# ★ 检查范围只含【用户可见文案与会流向用户可见面的代码】，三种模式用同一份范围（ROOTS 与 in_scope）：
#     pages/ server/ static/ hybrid/            —— 前端页面与 C 端 Go 服务
#     backend/server/api/client/                —— GVA 的 C 端接口与 DTO（直接面向用户）
#     backend/server/model/                     —— 数据模型（字段名与注释决定了接口会吐出什么）
#     backend/server/service/quant/             —— 题材、个股、AI 任务的业务层（文案与提示词的来源）
#     backend/server/config/                    —— 配置（含 AI 提示词与开关）
#   不查后台管理界面（backend/web）与 GVA 的系统管理代码：它们不面向用户，且多是上游代码。
#   排除规范文档（.claude/ .kiro/ docs/ scripts/）——它们必然包含这些词（是在定义禁令）；
#   排除词表的副本（compliance_words.txt、words.txt）与红队夹具目录（eval/）——它们的内容就是违规措辞本身。
# ★ 含违规措辞的测试夹具只能放 eval/ 或 testdata/*.jsonl；Go 测试里要用到禁用词，请在运行时从嵌入的词表里取。
# ★ 门禁自己有自测：bash scripts/dev/verify-compliance-gate.sh
# =============================================================================
set -uo pipefail

# TM_REPO_ROOT 只给门禁自测用（在临时目录树里跑），平时不设。
REPO_ROOT="${TM_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
WORDS_FILE="$REPO_ROOT/scripts/compliance-forbidden-words.txt"
MODE="${1:-staged}"

[ -f "$WORDS_FILE" ] || { echo "[跳过] 未找到词表 $WORDS_FILE"; exit 0; }

# 载入词表（去注释与空行）
WORDS="$(grep -vE '^[[:space:]]*(#|$)' "$WORDS_FILE" || true)"
[ -z "$WORDS" ] && { echo "[跳过] 词表为空"; exit 0; }

# 扫描根目录（相对仓库根）。三种模式共用，改这一处就是改全部。
ROOTS="pages server static hybrid backend/server/api/client backend/server/model backend/server/service/quant backend/server/config"

# in_scope <相对仓库根的路径>：该文件是否属于检查范围
in_scope() {
  case "$1" in
    eval/*|*/eval/*) return 1 ;;
    compliance_words.txt|*/compliance_words.txt|words.txt|*/words.txt) return 1 ;;
    *node_modules*|*uni_modules*) return 1 ;;
  esac
  local d under=1
  for d in $ROOTS; do
    case "$1" in "$d"/*) under=0; break ;; esac
  done
  [ "$under" -eq 0 ] || return 1
  case "$1" in
    *.uvue|*.uts|*.go|*.json|*.html|*.js|*.vue) return 0 ;;
    *) return 1 ;;
  esac
}

lc() { printf '%s' "$1" | tr 'A-Z' 'a-z'; }

HITS=""
if [ "$MODE" = "--all" ]; then
  for d in $ROOTS; do
    [ -d "$REPO_ROOT/$d" ] || continue
    FOUND="$(grep -rnF "$WORDS" "$REPO_ROOT/$d" \
        --include='*.uvue' --include='*.uts' --include='*.go' \
        --include='*.json' --include='*.html' --include='*.js' --include='*.vue' \
        --exclude-dir=node_modules --exclude-dir=uni_modules --exclude-dir=eval \
        --exclude='compliance_words.txt' --exclude='words.txt' 2>/dev/null || true)"
    [ -n "$FOUND" ] && HITS="$HITS$FOUND\n"
  done
elif [ "$MODE" = "staged" ]; then
  STAGED="$(git -c core.quotepath=false diff --cached --name-only --diff-filter=ACM 2>/dev/null || true)"
  for f in $STAGED; do
    in_scope "$f" || continue
    [ -f "$REPO_ROOT/$f" ] || continue
    FOUND="$(grep -nF "$WORDS" "$REPO_ROOT/$f" 2>/dev/null | sed "s|^|$f:|" || true)"
    [ -n "$FOUND" ] && HITS="$HITS$FOUND\n"
  done
else
  # 单文件模式：统一成"相对仓库根的路径"再套同一份范围。
  # （原实现直接拿传入的路径做 case 匹配：相对路径匹配不到 */docs/* 之类的排除项，
  #   仓库外的 .go 与 .json 文件也会被拿来对词表，和 --all、staged 的口径不一致。）
  TARGET="${MODE//\\//}"
  if command -v cygpath >/dev/null 2>&1; then   # Windows：D:/x/y 与 /d/x/y 两种写法归一
    TARGET="$(cygpath -u "$TARGET" 2>/dev/null || printf '%s' "$TARGET")"
  fi
  ROOT_POSIX="$(cd "$REPO_ROOT" 2>/dev/null && pwd -P)" || exit 0
  case "$TARGET" in
    /*) ABS="$TARGET" ;;
    *)  if [ -f "$PWD/$TARGET" ]; then ABS="$PWD/$TARGET"; else ABS="$ROOT_POSIX/$TARGET"; fi ;;
  esac
  [ -f "$ABS" ] || exit 0
  ABS="$(cd "$(dirname "$ABS")" && pwd -P)/$(basename "$ABS")"
  if [ "$(lc "${ABS:0:$((${#ROOT_POSIX} + 1))}")" = "$(lc "$ROOT_POSIX/")" ]; then
    REL="${ABS:$((${#ROOT_POSIX} + 1))}"
  else
    exit 0   # 仓库之外的文件不归本门禁管
  fi
  in_scope "$REL" || exit 0
  HITS="$(grep -nF "$WORDS" "$ABS" 2>/dev/null | sed "s|^|$REL:|" || true)"
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
