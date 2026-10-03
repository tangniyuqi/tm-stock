#!/bin/bash
# =============================================================================
# 合规禁用词门禁【自测】—— 用临时目录树里的探针文件，验证门禁本身没坏。
#
# 为什么需要给门禁写测试：门禁失效是【静默】的——它照样打印 [OK]，只是什么都没拦住。
# 本脚本覆盖三件事：
#   ① 范围：每个扫描根目录里放一个带禁用词的探针，必须被拦住；范围之外、词表副本、红队夹具目录必须放行；
#   ② 三种模式口径一致：--all、单文件（绝对路径、相对路径、Windows 反斜杠路径）、暂存区；
#   ③ 禁用词取自词表第一行，不在本文件里写死违规措辞（本文件在 scripts/，不受门禁扫描，但习惯上也不留）。
#
# 用法：bash scripts/dev/verify-compliance-gate.sh
# =============================================================================
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="$REPO_ROOT/scripts/check-compliance-words.sh"
WORDS_SRC="$REPO_ROOT/scripts/compliance-forbidden-words.txt"
PASS=0; FAIL=0

[ -f "$GATE" ] || { echo "[X] 找不到 $GATE"; exit 1; }
[ -f "$WORDS_SRC" ] || { echo "[X] 找不到词表 $WORDS_SRC"; exit 1; }
W="$(grep -vE '^[[:space:]]*(#|$)' "$WORDS_SRC" | head -1)"
[ -n "$W" ] || { echo "[X] 词表为空"; exit 1; }

TMP="$(mktemp -d)"
OUTSIDE="$(mktemp -d)"
STAGE="$(mktemp -d)"
trap 'rm -rf "$TMP" "$OUTSIDE" "$STAGE"' EXIT
mkdir -p "$TMP/scripts" "$STAGE/scripts"
cp "$WORDS_SRC" "$TMP/scripts/"
cp "$WORDS_SRC" "$STAGE/scripts/"

# 往目录树里放一个含禁用词的探针文件
put() { mkdir -p "$(dirname "$1")"; printf '// %s\n' "$W" > "$1"; }
reset_tree() { find "$TMP" -mindepth 1 -maxdepth 1 ! -name scripts -exec rm -rf {} + 2>/dev/null; }

record() { # <期望退出码> <实际退出码> <说明>
  if [ "$2" -eq "$1" ]; then
    echo "  [OK] $3（退出码 $2）"; PASS=$((PASS+1))
  else
    echo "  [X]  $3：期望退出码 $1，实际 $2"; FAIL=$((FAIL+1))
  fi
}

# all_case <期望退出码> <说明> <相对路径>...：清空树，放好探针，跑 --all
all_case() {
  local want="$1" desc="$2" f rc; shift 2
  reset_tree
  for f in "$@"; do put "$TMP/$f"; done
  TM_REPO_ROOT="$TMP" bash "$GATE" --all >/dev/null 2>&1; rc=$?
  record "$want" "$rc" "--all：$desc"
}

# file_case <期望退出码> <说明> <探针文件的路径>：在树根下以给定写法跑单文件模式
file_case() {
  local want="$1" desc="$2" arg="$3" cwd="${4:-$TMP}" rc
  ( cd "$cwd" && TM_REPO_ROOT="$TMP" bash "$GATE" "$arg" >/dev/null 2>&1 ); rc=$?
  record "$want" "$rc" "单文件：$desc"
}

echo "▶ 前置：真实仓库当前必须是干净的（否则后续断言不可信）"
if ( cd "$REPO_ROOT" && bash "$GATE" --all >/dev/null 2>&1 ); then
  echo "  [OK] 真实仓库 --all 通过"; PASS=$((PASS+1))
else
  echo "  [X] 真实仓库当前就有命中，先修掉再跑本自测"; exit 1
fi

echo ""
echo "▶ --all：每个扫描根目录里的违规探针必须被拦住"
all_case 1 "pages 下的页面"                     pages/a/b.uvue
all_case 1 "server 下的 Go 服务"                server/internal/x/y.go
all_case 1 "static 下的 json"                   static/z.json
all_case 1 "hybrid 下的 html"                   hybrid/h.html
all_case 1 "GVA 的 C 端接口（api/client）"       backend/server/api/client/v1/quant/x.go
all_case 1 "GVA 的数据模型（model）"             backend/server/model/quant/m.go
all_case 1 "GVA 的题材业务层（service/quant）"    backend/server/service/quant/s.go
all_case 1 "GVA 的配置（config）"                backend/server/config/c.go
all_case 1 "扫描根目录下很深的子目录"             backend/server/service/quant/sub/dir/deep.go

echo ""
echo "▶ --all：范围之外、词表副本与红队夹具必须放行"
all_case 0 "空目录树"
all_case 0 "GVA 题材业务层里的词表副本"           backend/server/service/quant/compliance_words.txt
all_case 0 "server 里的词表副本"                  server/internal/ai/guard/words.txt
all_case 0 "红队夹具目录 eval/"                   eval/redteam/x.go
all_case 0 "扫描根目录下的 eval 子目录"           backend/server/service/quant/eval/x.go
all_case 0 "GVA 的系统管理代码（不在范围内）"      backend/server/service/system/s.go
all_case 0 "后台管理界面（不在范围内）"           backend/web/src/view/x.vue
all_case 0 "规范文档目录"                         docs/x.json
all_case 0 "脚本目录"                             scripts/y.go
all_case 0 "第三方依赖目录"                       pages/node_modules/x.js

echo ""
echo "▶ 单文件模式（L1 钩子用）：与 --all 同一口径"
reset_tree; put "$TMP/backend/server/api/client/v1/quant/x.go"; put "$TMP/docs/x.json"; put "$TMP/backend/server/service/quant/eval/x.go"
put "$TMP/backend/server/service/quant/compliance_words.txt"; put "$OUTSIDE/y.go"; put "$OUTSIDE/backend/server/api/client/x.go"
file_case 1 "范围内文件，绝对路径"                   "$TMP/backend/server/api/client/v1/quant/x.go"
file_case 1 "范围内文件，相对仓库根的路径"            "backend/server/api/client/v1/quant/x.go"
file_case 1 "范围内文件，在别的工作目录下用相对路径"   "api/client/v1/quant/x.go" "$TMP/backend/server"
file_case 1 "范围内文件，带 ./ 前缀"                 "./backend/server/api/client/v1/quant/x.go"
if command -v cygpath >/dev/null 2>&1; then
  file_case 1 "范围内文件，Windows 反斜杠绝对路径"   "$(cygpath -w "$TMP/backend/server/api/client/v1/quant/x.go")"
fi
file_case 0 "文档目录里的文件"                       "docs/x.json"
file_case 0 "扫描根目录下的 eval 子目录"             "backend/server/service/quant/eval/x.go"
file_case 0 "词表副本"                               "backend/server/service/quant/compliance_words.txt"
file_case 0 "仓库之外的文件（即使扩展名是 .go）"      "$OUTSIDE/y.go"
file_case 0 "另一棵目录树里目录结构相同的文件（不属于本仓库）" "$OUTSIDE/backend/server/api/client/x.go"
file_case 0 "不存在的文件"                           "backend/server/api/client/v1/quant/nope.go"

echo ""
echo "▶ 暂存区模式（pre-commit 实际走的路径）"
stage_case() { # <期望退出码> <说明> <要暂存的相对路径>...
  local want="$1" desc="$2" f rc; shift 2
  ( cd "$STAGE" && rm -rf .git && git init -q . && git config core.autocrlf false )
  find "$STAGE" -mindepth 1 -maxdepth 1 ! -name scripts ! -name .git -exec rm -rf {} + 2>/dev/null
  for f in "$@"; do put "$STAGE/$f"; ( cd "$STAGE" && git add -f "$f" ); done
  ( cd "$STAGE" && bash "$GATE" >/dev/null 2>&1 ); rc=$?
  record "$want" "$rc" "暂存区：$desc"
}
stage_case 1 "暂存了范围内的违规文件"                 backend/server/model/quant/m.go
stage_case 1 "暂存了 pages 下的违规文件"              pages/a.uvue
stage_case 0 "只暂存了范围外的文件"                   backend/server/service/system/s.go
stage_case 0 "只暂存了词表副本"                       backend/server/service/quant/compliance_words.txt
stage_case 0 "只暂存了红队夹具"                       eval/redteam/x.go
stage_case 0 "暂存了扫描根目录下的 eval 子目录"        backend/server/service/quant/eval/x.go
stage_case 1 "范围外与范围内混着暂存，仍要拦"          backend/server/service/system/s.go backend/server/config/c.go

echo ""
echo "═══════════════════════════════"
echo "  通过 $PASS 项 / 失败 $FAIL 项"
echo "═══════════════════════════════"
[ "$FAIL" -eq 0 ] || exit 1
