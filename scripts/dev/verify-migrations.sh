#!/bin/bash
# =============================================================================
# 迁移与约束的【真实执行】验证 —— 用 Docker 起一个一次性 MySQL 8，
# 把 server/migrations/*.sql 全部跑一遍，然后【实测】合规约束是否真的拦得住。
#
# 为什么需要它：
#   建表脚本写了 NOT NULL + CHECK 不等于它生效。
#   MySQL 5.7 会【静默忽略】CHECK 子句；8.0.16 之前也不强制。
#   "写了约束"和"约束生效"是两件事，只有真插一条脏数据才知道。
#
# 用法：
#   bash scripts/dev/verify-migrations.sh          # 跑完自动清理容器
#   bash scripts/dev/verify-migrations.sh --keep   # 保留容器供手工排查
# =============================================================================
set -uo pipefail

CONTAINER="tm-stock-verify-db"
DB="tm_stock_verify"
ROOT_PW="fake-local-verify-only"    # 一次性容器，不映射端口，不含真实数据
                                    # 用 fake- 前缀是本仓约定（见 check-secret-scan.sh 放行规则），
                                    # 不是为了绕门禁——它确实是个假口令
IMAGE="mysql:8.0"
KEEP=false
[ "${1:-}" = "--keep" ] && KEEP=true

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PASS=0; FAIL=0

ok()   { echo "  [OK] $1"; PASS=$((PASS+1)); }
bad()  { echo "  [X]  $1"; FAIL=$((FAIL+1)); }
info() { echo "$1"; }

cleanup() {
  if [ "$KEEP" = false ]; then
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  else
    info ""
    info "容器已保留：docker exec -it $CONTAINER mysql -uroot -p$ROOT_PW $DB"
  fi
}
trap cleanup EXIT

# 环境不具备时【优雅跳过】而不是失败。
# 依据 known-pitfalls P9：红灯常态化就等于门禁失效——
# 因环境缺失而天天红的检查，很快就会被所有人忽略。
command -v docker >/dev/null 2>&1 || { echo "[跳过] 本机无 docker"; exit 0; }
if ! docker version --format '{{.Server.Version}}' >/dev/null 2>&1; then
  echo "[跳过] docker daemon 未运行（Windows 上需先启动 Docker Desktop）"
  echo "       CLI 存在但 daemon 不可用时必须跳过，否则这条检查会长期红灯。"
  exit 0
fi

# ── 起容器 ──
info "▶ 启动一次性 MySQL（不映射端口，仅容器内访问）..."
docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$CONTAINER" \
  -e MYSQL_ROOT_PASSWORD="$ROOT_PW" \
  -e MYSQL_DATABASE="$DB" \
  "$IMAGE" >/dev/null || { echo "[X] 容器启动失败"; exit 1; }

info "▶ 等待 MySQL 就绪..."
READY=false
for i in $(seq 1 60); do
  if docker exec "$CONTAINER" mysqladmin ping -uroot -p"$ROOT_PW" --silent >/dev/null 2>&1; then
    READY=true; break
  fi
  sleep 2
done
[ "$READY" = true ] || { echo "[X] MySQL 120 秒内未就绪"; docker logs --tail 20 "$CONTAINER"; exit 1; }

VER="$(docker exec "$CONTAINER" mysql -uroot -p"$ROOT_PW" -N -B -e "SELECT VERSION()" 2>/dev/null | tr -d '\r')"
info "  MySQL 版本：$VER"

# 辅助：在容器内执行 SQL。成功返回 0，失败返回非 0（stderr 落到 $ERRFILE）
ERRFILE="$(mktemp)"
sql() { docker exec -i "$CONTAINER" mysql -uroot -p"$ROOT_PW" "$DB" 2>"$ERRFILE"; }
sqlq() { docker exec -i "$CONTAINER" mysql -uroot -p"$ROOT_PW" -N -B "$DB" 2>/dev/null; }

# ── 1) 跑全部迁移 ──
info ""
info "▶ 执行 server/migrations/*.sql"
shopt -s nullglob
MIGS=("$REPO_ROOT"/server/migrations/*.sql)
[ ${#MIGS[@]} -eq 0 ] && { echo "[跳过] 无迁移脚本"; exit 0; }
for f in "${MIGS[@]}"; do
  if sql < "$f"; then
    ok "$(basename "$f")"
  else
    bad "$(basename "$f") 执行失败：$(head -3 "$ERRFILE")"
  fi
done

# ── 2) 建表结果核对 ──
info ""
info "▶ 表结构核对"
for t in theme_category theme theme_chain_node stock theme_stock_mapping theme_event; do
  N="$(printf 'SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name="%s";' "$t" | sqlq | tr -d '\r')"
  [ "$N" = "1" ] && ok "表 $t 存在" || bad "表 $t 缺失"
done

# ── 3) 🔴 实测 CHECK 约束是否真的拦得住空证据 ──
info ""
info "▶ 合规约束实测（这是本脚本的核心）"

# 前置：造一条合法映射，确认正常路径能插进去
printf "INSERT INTO stock(code,name,market,delisted,created_at,updated_at) VALUES('688502','样例股','SH',0,NOW(),NOW());\n" | sql >/dev/null
GOOD="INSERT INTO theme_stock_mapping
 (theme_id,chain_node_id,stock_code,source_type,source_excerpt,source_url,collected_at,created_at,updated_at)
 VALUES (1,100,'688502','年报','公司该业务收入占比 34.2%','https://example.com/r',NOW(),NOW(),NOW());"
if printf '%s\n' "$GOOD" | sql >/dev/null; then
  ok "证据齐全的映射可以插入（正例）"
else
  bad "证据齐全的映射被拒，约束过严：$(head -3 "$ERRFILE")"
fi

# 反例：三项证据分别置空，应【全部被拒】
i=0
for col in source_type source_excerpt source_url; do
  i=$((i+1))
  vals=("年报" "摘录文本" "https://example.com/r")
  case "$col" in
    source_type)    vals[0]="" ;;
    source_excerpt) vals[1]="" ;;
    source_url)     vals[2]="" ;;
  esac
  STMT="INSERT INTO theme_stock_mapping
   (theme_id,chain_node_id,stock_code,source_type,source_excerpt,source_url,collected_at,created_at,updated_at)
   VALUES (1,$((200+i)),'688502','${vals[0]}','${vals[1]}','${vals[2]}',NOW(),NOW(),NOW());"
  if printf '%s\n' "$STMT" | sql >/dev/null; then
    bad "$col 为空竟然插入成功 —— CHECK 约束未生效！（MySQL $VER）"
  else
    ok "$col 为空被拒绝（反例）"
  fi
done

# NULL 也必须被 NOT NULL 拦住
STMT="INSERT INTO theme_stock_mapping
 (theme_id,chain_node_id,stock_code,source_type,source_excerpt,source_url,collected_at,created_at,updated_at)
 VALUES (1,299,'688502',NULL,'x','https://e.com',NOW(),NOW(),NOW());"
if printf '%s\n' "$STMT" | sql >/dev/null; then
  bad "source_type 为 NULL 竟然插入成功 —— NOT NULL 未生效"
else
  ok "source_type 为 NULL 被拒绝"
fi

# ── 4) 默认 status 必须是 DRAFT（fail-safe：误插入的行不对外可见）──
DEF="$(printf "SELECT status FROM theme_stock_mapping WHERE chain_node_id=100 LIMIT 1;" | sqlq | tr -d '\r')"
[ "$DEF" = "DRAFT" ] && ok "status 默认 DRAFT（误插入的行不会直接对外可见）" \
                     || bad "status 默认值为 '$DEF'，应为 DRAFT"

# ── 5) 唯一键：同环节同股票不得重复 ──
if printf '%s\n' "$GOOD" | sql >/dev/null; then
  bad "同环节重复插入同一股票成功 —— uk_node_stock 未生效"
else
  ok "同环节重复插入被唯一键拒绝"
fi

rm -f "$ERRFILE"

info ""
info "═══════════════════════════════"
info "  通过 $PASS 项 / 失败 $FAIL 项"
info "═══════════════════════════════"
[ "$FAIL" -eq 0 ] || exit 1
