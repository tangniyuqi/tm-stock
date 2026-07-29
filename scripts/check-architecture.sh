#!/bin/bash
# =============================================================================
# Go 架构守护 —— 把分层约定变成机器检查（MLR ArchUnit 的轻量等价物）
# 用法：bash scripts/check-architecture.sh
#
# ★ 为什么需要：分层约定写在规范里没人天天记得，架构会在一次次"就这一次"里烂掉。
#   能机检的规则就不要靠自觉。
# =============================================================================
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRV="$REPO_ROOT/server"
[ -d "$SRV" ] || { echo "[跳过] 无 server/ 目录"; exit 0; }
RESULT=0

fail() { echo "[X] $1"; echo "    位置：$2"; RESULT=1; }

# ── 规则 1：handler 不得直连数据库 ──
HITS="$(grep -rlnE '"(database/sql|gorm\.io/gorm|github\.com/jmoiron/sqlx)"' "$SRV/internal/handler" 2>/dev/null || true)"
[ -n "$HITS" ] && fail "handler 层直连数据库（必须经 service → repository）" "$HITS"

# ── 规则 2：handler 不得跨层直接引用 repository ──
HITS="$(grep -rln "internal/repository" "$SRV/internal/handler" 2>/dev/null || true)"
[ -n "$HITS" ] && fail "handler 跨层引用 repository（应只依赖 service）" "$HITS"

# ── 规则 3：service 不得依赖 HTTP 框架（业务层与传输层解耦）──
HITS="$(grep -rln "github.com/gin-gonic/gin" "$SRV/internal/service" 2>/dev/null || true)"
[ -n "$HITS" ] && fail "service 依赖 gin（业务层不应知道 HTTP）" "$HITS"

# ── 规则 4：repository 不得反向依赖 service ──
HITS="$(grep -rln "internal/service" "$SRV/internal/repository" 2>/dev/null || true)"
[ -n "$HITS" ] && fail "repository 反向依赖 service（依赖必须单向）" "$HITS"

# ── 规则 5：禁止在代码里硬编码密钥（与密钥扫描互补，这里查 Go 常量）──
HITS="$(grep -rnE '(Password|Secret|Token|ApiKey)[[:space:]]*=[[:space:]]*"[^"]{8,}"' "$SRV" 2>/dev/null | grep -v _test.go | grep -vi "getenv\|os\.\|example\|your_" || true)"
[ -n "$HITS" ] && fail "Go 代码里疑似硬编码密钥（应从环境变量读）" "$HITS"

# ── 规则 6：对外 DTO 的时间字段必须是时间戳（禁 string）──
HITS="$(grep -rnE '(Time|Date|At)[[:space:]]+string' "$SRV/internal/dto" 2>/dev/null | grep -v _test.go || true)"
[ -n "$HITS" ] && fail "DTO 时间字段声明为 string（必须用 int64 毫秒时间戳，见 api-contract-alignment）" "$HITS"

if [ $RESULT -eq 0 ]; then
  echo "[OK] 架构约定检查通过"
else
  echo ""
  echo "[依据] .claude/agents/backend-standards-go.md 分层约定"
  echo "[红线] 不要为了过检查而绕过分层（如给 handler 包一层同名壳）"
fi
exit $RESULT
