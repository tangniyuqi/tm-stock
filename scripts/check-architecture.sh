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

# imports_of <目录> <路径正则> —— 只匹配【真正的 import 行】，不匹配注释里的提及。
#
# ★ 为什么不能用裸 grep（2026-07-31 实测踩过）：
#   原规则 2 写作 grep -rln "internal/repository"，结果把 handler 包里
#   一句「本层不得 import internal/repository」的**注释**当成了违规。
#   而那句注释恰恰是有价值的——天真的修法是删注释，那是把文档改坏去迁就工具。
#   正确修法是让规则认得 Go 的 import 语法：
#   行首空白 + 可选别名（_ / . / 标识符）+ 引号包裹的路径。
imports_of() {
  grep -rlnE "^[[:space:]]*([_.]|[A-Za-z][A-Za-z0-9_]*)?[[:space:]]*\"[^\"]*($2)\"" "$1" 2>/dev/null || true
}

# ── 规则 1：handler 不得直连数据库 ──
HITS="$(imports_of "$SRV/internal/handler" 'database/sql|gorm\.io/gorm|github\.com/jmoiron/sqlx')"
[ -n "$HITS" ] && fail "handler 层直连数据库（必须经 service → repository）" "$HITS"

# ── 规则 2：handler 不得跨层直接引用 repository ──
HITS="$(imports_of "$SRV/internal/handler" 'internal/repository')"
[ -n "$HITS" ] && fail "handler 跨层引用 repository（应只依赖 service）" "$HITS"

# ── 规则 3：service 不得依赖 HTTP 框架（业务层与传输层解耦）──
HITS="$(imports_of "$SRV/internal/service" 'github\.com/gin-gonic/gin|net/http')"
[ -n "$HITS" ] && fail "service 依赖 HTTP 框架/net.http（业务层不应知道 HTTP）" "$HITS"

# ── 规则 4：repository 不得反向依赖 service ──
HITS="$(imports_of "$SRV/internal/repository" 'internal/service')"
[ -n "$HITS" ] && fail "repository 反向依赖 service（依赖必须单向）" "$HITS"

# ── 规则 5：禁止在代码里硬编码密钥（与密钥扫描互补，这里查 Go 常量）──
HITS="$(grep -rnE '(Password|Secret|Token|ApiKey)[[:space:]]*=[[:space:]]*"[^"]{8,}"' "$SRV" 2>/dev/null | grep -v _test.go | grep -vi "getenv\|os\.\|example\|your_" || true)"
[ -n "$HITS" ] && fail "Go 代码里疑似硬编码密钥（应从环境变量读）" "$HITS"

# ── 规则 6：对外 DTO 的时间字段必须是时间戳（禁 string）──
HITS="$(grep -rnE '(Time|Date|At)[[:space:]]+string' "$SRV/internal/dto" 2>/dev/null | grep -v _test.go || true)"
[ -n "$HITS" ] && fail "DTO 时间字段声明为 string（必须用 int64 毫秒时间戳，见 api-contract-alignment）" "$HITS"

# =============================================================================
# 🔴 合规结构守护（规则 7–12）
#
# 来源：docs/specs/*/design.md 的「契约层面的合规约束」。
# 为什么放在这里：这些约束是【结构性】的——只要字段/依赖存在，它迟早会被用。
# 在代码写出来之前先立规则，比事后清理便宜得多。
# =============================================================================

# ── 规则 7：禁止价值评价类字段（对个股做投资价值评价 = 落入荐股要件一）──
# 只列【无正当用途】的字段名。刻意不含 Score / Relevance：
#   搜索相关度打分是合法的检索技术指标（见 flash-news/design.md），
#   一并封禁会误伤。那两个词靠人工评审把关。
EVAL_PAT='(IsLeader|LeaderRank|Purity|PurityScore|RecommendLevel|RecommendScore|BenefitLevel|Importance)[[:space:]]+(float|int|string|bool)'
HITS="$(grep -rnE "$EVAL_PAT" "$SRV/internal/dto" "$SRV/internal/model" 2>/dev/null | grep -v _test.go || true)"
[ -n "$HITS" ] && fail "出现价值评价类字段（龙头/纯正度/推荐度/受益度/重要性）——对个股做价值评价即落入荐股定义，删除该字段，不要改名保留" "$HITS"

# ── 规则 8：禁止个股详情类字段（基本面/K线/技术指标，ADR-0003 一期整体不做）──
# ★ 坑：POSIX 括号表达式里 \ 不是转义符。曾写成 [[:alnum:]_*\[\]]* ，
#   被解析为「字符集 + 一个字面 ]」，导致该规则永远不命中（静默失效）。
#   改用「结构体字段行」锚定：缩进 + 字段名 + 空白 + 类型首字符（含 [ 与 *）。
DETAIL_PAT='^[[:space:]]+(Kline|KLine|Candlestick|Fundamental|FinancialReport|PeRatio|PbRatio|Macd|MACD|Kdj|KDJ|RsiValue|TechnicalIndicator)[[:space:]]+[A-Za-z[*]'
HITS="$(grep -rnE "$DETAIL_PAT" "$SRV/internal/dto" "$SRV/internal/model" 2>/dev/null | grep -v _test.go || true)"
[ -n "$HITS" ] && fail "出现个股详情类字段（基本面/K线/技术指标）——这是红线而非排期，一期整体不做（ADR-0003）" "$HITS"

# ── 规则 9：禁止「无客观依据的推荐位」字段（变相推荐）──
# 高亮只能是前端的用户选中态，不能由服务端下发（见 theme-query/design.md）
HL_PAT='(Highlight|Highlighted|IsFeatured|Featured|IsTop|IsRecommended|Pinned)[[:space:]]+(bool|int)'
HITS="$(grep -rnE "$HL_PAT" "$SRV/internal/dto" 2>/dev/null | grep -v _test.go || true)"
[ -n "$HITS" ] && fail "服务端下发高亮/置顶/推荐位——无客观依据的置顶构成变相推荐；高亮应是前端的用户选中态" "$HITS"

# ── 规则 10：禁止快讯定性字段（利好/利空/情绪 = 我方定性）──
SENT_PAT='(Sentiment|Impact|IsPositive|IsNegative|NewsLevel)[[:space:]]+(float|int|string|bool)'
HITS="$(grep -rnE "$SENT_PAT" "$SRV/internal/dto" "$SRV/internal/model" 2>/dev/null | grep -v _test.go || true)"
[ -n "$HITS" ] && fail "出现快讯定性字段（利好/利空/情绪）——转载只做客观呈现，不加我方定性（flash-news §3.4）" "$HITS"

# ── 规则 11：禁止支付 SDK（一期不做在线支付 → 免经营性 ICP）──
PAY_PAT='(wechatpay|alipay|stripe|paypal|unionpay|adyen|payjs)'
HITS="$(grep -rniE "$PAY_PAT" "$SRV/go.mod" "$SRV/go.sum" 2>/dev/null || true)"
HITS="$HITS$(grep -rlniE "$PAY_PAT" "$SRV" --include='*.go' 2>/dev/null || true)"
[ -n "$(printf '%s' "$HITS" | tr -d '[:space:]')" ] && fail "引入支付 SDK——一期走站外收款 + 兑换码激活以免经营性 ICP（member-center §3.1）" "$HITS"

# ── 规则 12：对外 DTO 禁止明文手机号 ──
# 允许 PhoneMasked / PhoneHash；禁止裸 Phone / Mobile
PHONE_PAT='^[[:space:]]*(Phone|Mobile|PhoneNumber)[[:space:]]+string'
HITS="$(grep -rnE "$PHONE_PAT" "$SRV/internal/dto" 2>/dev/null | grep -v _test.go || true)"
[ -n "$HITS" ] && fail "对外 DTO 含明文手机号字段——脱敏必须在服务端完成（不是改个字段名了事），出参只允许 PhoneMasked" "$HITS"

if [ $RESULT -eq 0 ]; then
  echo "[OK] 架构约定检查通过（分层 6 条 + 合规结构 6 条）"
else
  echo ""
  echo "[依据] .claude/agents/backend-standards-go.md 分层约定"
  echo "[依据] .claude/agents/compliance-redline.md + docs/specs/*/design.md 契约约束"
  echo "[红线] 不要为了过检查而绕过分层（如给 handler 包一层同名壳）"
  echo "[红线] 合规类规则（7–12）触发时，正确做法是【删掉该能力】，不是改字段名规避检查"
fi
exit $RESULT
