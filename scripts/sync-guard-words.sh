#!/bin/bash
# =============================================================================
# 同步合规词表到各处副本（P1-6 词表副本同步守卫）
#   源（唯一真源）：scripts/compliance-forbidden-words.txt
#   副本（go:embed 不能嵌入包目录之外的文件，所以每个使用方都必须有副本）：
#     1. server/internal/ai/guard/words.txt                  AI 护栏（tm-stock 后端）
#     2. backend/server/service/quant/compliance_words.txt   GVA 题材股票写入口的依据摘录校验
#
# 用法：
#   bash scripts/sync-guard-words.sh           # 复制源到全部副本（统一成 LF）
#   bash scripts/sync-guard-words.sh --check   # 只检查是否都与真源一致；任何一处不一致退出 1（CI / 门禁用）
#
# ★ 为什么要守卫：副本悄悄落后于源，护栏就会比用户可见文案的门禁更松，
#   而这正是红线检测要"至少一样严"的地方。词表规则不变：只增不减，禁止为过门禁删词。
# =============================================================================
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
SRC="$ROOT/scripts/compliance-forbidden-words.txt"
DSTS=(
  "$ROOT/server/internal/ai/guard/words.txt"
  "$ROOT/backend/server/service/quant/compliance_words.txt"
)
MODE="${1:-sync}"

[ -f "$SRC" ] || { echo "[X] 找不到词表真源 $SRC" >&2; exit 1; }

case "$MODE" in
  --check)
    BAD=0
    for DST in "${DSTS[@]}"; do
      REL="${DST#"$ROOT"/}"
      if [ ! -f "$DST" ]; then
        echo "[X] 缺少副本 $REL，请先运行：bash scripts/sync-guard-words.sh" >&2
        BAD=1
        continue
      fi
      if diff <(tr -d '\r' < "$SRC") <(tr -d '\r' < "$DST") > /dev/null; then
        echo "[OK] 词表副本与真源一致：$REL"
      else
        echo "[X] 词表副本与真源不一致：$REL（副本被改动，或真源新增了词而副本未同步）" >&2
        diff <(tr -d '\r' < "$SRC") <(tr -d '\r' < "$DST") | head -20 >&2 || true
        BAD=1
      fi
    done
    if [ "$BAD" -ne 0 ]; then
      echo "[修复] 运行：bash scripts/sync-guard-words.sh 然后提交各副本文件" >&2
      exit 1
    fi
    exit 0
    ;;
  sync)
    for DST in "${DSTS[@]}"; do
      mkdir -p "$(dirname "$DST")"
      tr -d '\r' < "$SRC" > "$DST"
      echo "[OK] 已同步：${SRC#"$ROOT"/} -> ${DST#"$ROOT"/}"
    done
    ;;
  *)
    echo "用法：bash scripts/sync-guard-words.sh [--check]" >&2
    exit 2
    ;;
esac
