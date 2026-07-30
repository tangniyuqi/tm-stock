#!/bin/bash
# =============================================================================
# 迁移与约束的【真实执行】验证 —— 用 Docker 起一次性 MySQL 8，
# 跑完 server/migrations/*.sql，然后【实测】合规约束是否真的拦得住脏数据。
#
# 为什么需要它：
#   建表脚本里写了 NOT NULL + CHECK，不等于它生效。
#   MySQL 5.7 会【静默忽略】CHECK 子句；8.0.16 之前也不强制。
#   "写了约束"和"约束生效"是两件事，只有真插一条脏数据才知道。
#
# 2026-07-30 首次实测结果（MySQL 8.0.45，两种拒绝各自点名了机制）：
#   证据字段为空串 → ERROR 3819 (HY000): Check constraint 'chk_evidence_not_blank' is violated.
#   证据字段为 NULL → ERROR 1048 (23000): Column 'source_type' cannot be null
#   同环节重复股票 → 唯一键 uk_node_stock 拒绝
#   → 6 条 INSERT 只有 1 条（证据齐全的）进库，status 默认 DRAFT。约束确认生效。
#
# ★ 设计要点：宿主机进程数压到最少（约 5 个）。
#   初版在每次探测都 docker exec，共 ~80 个宿主进程，
#   在内存吃紧的 Windows 上直接把 Git Bash 的 fork 打崩
#   （cygheap read copy failed / Resource temporarily unavailable）。
#   改法：① 就绪等待放进容器内循环，宿主只调一次
#         ② 断言用 mysql --force 一次性跑完，再【查表看实际落库结果】——
#            这比逐条判断退出码更强：它验的是最终状态，不是中间信号。
#
# 用法：
#   bash scripts/dev/verify-migrations.sh          # 跑完自动清理
#   bash scripts/dev/verify-migrations.sh --keep   # 保留容器供手工排查
# =============================================================================
set -uo pipefail

CONTAINER="tm-stock-verify-db"
DB="tm_stock_verify"
ROOT_PW="fake-local-verify-only"    # 一次性容器，不映射端口，不含真实数据
                                    # 用 fake- 前缀是本仓约定（见 check-secret-scan.sh 放行规则）
IMAGE="mysql:8.0"
KEEP=false
[ "${1:-}" = "--keep" ] && KEEP=true

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PASS=0; FAIL=0
ok()  { echo "  [OK] $1"; PASS=$((PASS+1)); }
bad() { echo "  [X]  $1"; FAIL=$((FAIL+1)); }

cleanup() {
  if [ "$KEEP" = false ]; then
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  else
    echo ""
    echo "容器已保留：docker exec -it $CONTAINER mysql -uroot -p$ROOT_PW $DB"
  fi
}
trap cleanup EXIT

# 环境不具备时【优雅跳过】而不是失败（known-pitfalls P9：红灯常态化=门禁失效）
command -v docker >/dev/null 2>&1 || { echo "[跳过] 本机无 docker"; exit 0; }
if ! docker version --format '{{.Server.Version}}' >/dev/null 2>&1; then
  echo "[跳过] docker daemon 未运行（Windows 上需先启动 Docker Desktop）"
  exit 0
fi
docker image inspect "$IMAGE" >/dev/null 2>&1 || {
  echo "[跳过] 本地无 $IMAGE 镜像，先执行：docker pull $IMAGE"; exit 0; }

# ── 1) 起容器 ──
echo "▶ 启动一次性 MySQL（不映射端口）..."
docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$CONTAINER" \
  -e MYSQL_ROOT_PASSWORD="$ROOT_PW" -e MYSQL_DATABASE="$DB" \
  "$IMAGE" >/dev/null || { echo "[X] 容器启动失败"; exit 1; }

# ── 2) 就绪等待：循环在【容器内】，宿主机只起一个进程 ──
#
# ★★ 关键坑（实测踩过）：不能用 `mysqladmin ping` 判就绪。
#    mysql 镜像首次初始化时会先起一个【临时服务器】（--skip-networking，只走 socket），
#    ping 对它立刻成功；随后临时服务器关闭、真服务器启动。
#    探测撞进这个切换窗口 → 后续查询全部返回空，而脚本却以为已就绪。
#    → 改用 TCP 连接（--protocol=TCP -h 127.0.0.1）：
#      初始化期不监听网络，只有真服务器起来 TCP 才通，这才是可靠判据。
echo "▶ 等待 MySQL 就绪（TCP 探测，循环在容器内执行）..."
if ! docker exec "$CONTAINER" bash -c '
    for i in $(seq 1 90); do
      if mysql -h 127.0.0.1 --protocol=TCP -uroot -p"$MYSQL_ROOT_PASSWORD" \
           -N -B -e "SELECT 1" >/dev/null 2>&1; then exit 0; fi
      sleep 2
    done
    exit 1' >/dev/null 2>&1; then
  echo "[X] MySQL 180 秒内未就绪"
  docker logs --tail 15 "$CONTAINER"
  exit 1
fi

VER="$(docker exec "$CONTAINER" mysql -h 127.0.0.1 --protocol=TCP \
        -uroot -p"$ROOT_PW" -N -B -e 'SELECT VERSION()' 2>/dev/null | tr -d '\r')"
# ★ 空结果守卫：初版没有这条，于是拿着空 TABLES 一路跑出 9 个假失败，
#   把「连不上」误报成「表全缺失 + 约束全失效」。空结果必须当错误处理。
if [ -z "$VER" ]; then
  echo "[X] 已就绪但取不到版本号 —— 连接异常，中止（不带着空结果继续断言）"
  docker logs --tail 15 "$CONTAINER"
  exit 1
fi
echo "  MySQL 版本：$VER"

# ── 3) 跑迁移 + 断言，全部在【一次 docker exec】里完成 ──
echo ""
echo "▶ 执行迁移并实测约束（单次 exec，--force 使脏数据逐条被拒后继续）"

shopt -s nullglob
MIGS=("$REPO_ROOT"/server/migrations/*.sql)
[ ${#MIGS[@]} -eq 0 ] && { echo "[跳过] 无迁移脚本"; exit 0; }
echo "  迁移文件：${#MIGS[@]} 个"

FULL_SQL="$(cat "${MIGS[@]}")
-- ══════════ 以下为断言用数据 ══════════
INSERT INTO stock(code,name,market,delisted,created_at,updated_at)
  VALUES('688502','样例股','SH',0,NOW(),NOW());

-- 正例：证据齐全 → 应成功（node 100）
INSERT INTO theme_stock_mapping
 (theme_id,chain_node_id,stock_code,source_type,source_excerpt,source_url,collected_at,created_at,updated_at)
 VALUES (1,100,'688502','年报','该业务收入占比 34.2%','https://example.com/r',NOW(),NOW(),NOW());

-- 反例 201：source_type 为空串 → CHECK 应拒
INSERT INTO theme_stock_mapping
 (theme_id,chain_node_id,stock_code,source_type,source_excerpt,source_url,collected_at,created_at,updated_at)
 VALUES (1,201,'688502','','摘录','https://example.com/r',NOW(),NOW(),NOW());

-- 反例 202：source_excerpt 为空串 → CHECK 应拒
INSERT INTO theme_stock_mapping
 (theme_id,chain_node_id,stock_code,source_type,source_excerpt,source_url,collected_at,created_at,updated_at)
 VALUES (1,202,'688502','年报','','https://example.com/r',NOW(),NOW(),NOW());

-- 反例 203：source_url 为空串 → CHECK 应拒
INSERT INTO theme_stock_mapping
 (theme_id,chain_node_id,stock_code,source_type,source_excerpt,source_url,collected_at,created_at,updated_at)
 VALUES (1,203,'688502','年报','摘录','',NOW(),NOW(),NOW());

-- 反例 204：source_type 为 NULL → NOT NULL 应拒
INSERT INTO theme_stock_mapping
 (theme_id,chain_node_id,stock_code,source_type,source_excerpt,source_url,collected_at,created_at,updated_at)
 VALUES (1,204,'688502',NULL,'摘录','https://example.com/r',NOW(),NOW(),NOW());

-- 反例：同环节同股票重复（node 100 再插一次）→ 唯一键应拒
INSERT INTO theme_stock_mapping
 (theme_id,chain_node_id,stock_code,source_type,source_excerpt,source_url,collected_at,created_at,updated_at)
 VALUES (1,100,'688502','年报','另一段摘录','https://example.com/r2',NOW(),NOW(),NOW());

-- ══════════ 输出实际落库结果供断言 ══════════
SELECT 'TABLES', GROUP_CONCAT(table_name ORDER BY table_name)
  FROM information_schema.tables WHERE table_schema=DATABASE();
SELECT 'MAPPING_ROWS', IFNULL(GROUP_CONCAT(CONCAT(chain_node_id,'/',status) ORDER BY chain_node_id),'NONE')
  FROM theme_stock_mapping;
SELECT 'MAPPING_COUNT', COUNT(*) FROM theme_stock_mapping;
"

OUT="$(printf '%s\n' "$FULL_SQL" | docker exec -i "$CONTAINER" \
        mysql -h 127.0.0.1 --protocol=TCP -uroot -p"$ROOT_PW" --force -N -B "$DB" 2>&1 | tr -d '\r')"

# 空结果守卫（同上）：拿不到 TABLES 说明连接或执行整体失败，不要继续断言
if ! printf '%s\n' "$OUT" | grep -q '^TABLES'; then
  echo "[X] SQL 执行未返回结果，中止。原始输出前 20 行："
  printf '%s\n' "$OUT" | head -20
  exit 1
fi

TABLES="$(printf '%s\n' "$OUT" | awk -F'\t' '$1=="TABLES"{print $2}')"
ROWS="$(printf '%s\n' "$OUT"   | awk -F'\t' '$1=="MAPPING_ROWS"{print $2}')"
COUNT="$(printf '%s\n' "$OUT"  | awk -F'\t' '$1=="MAPPING_COUNT"{print $2}')"

# ── 4) 断言 ──
echo ""
echo "▶ 表结构"
for t in theme_category theme theme_chain_node stock theme_stock_mapping theme_event; do
  case ",$TABLES," in *",$t,"*) ok "表 $t 存在" ;; *) bad "表 $t 缺失（TABLES=$TABLES）" ;; esac
done

echo ""
echo "▶ 🔴 合规约束实测（判据 = 实际落库结果，不是中间退出码）"
echo "  实际落库：$ROWS   行数：$COUNT"

if [ "$COUNT" = "1" ]; then
  ok "6 条 INSERT 只有 1 条进库 —— 5 条脏数据全部被拒"
else
  bad "应只有 1 条进库，实际 $COUNT 条。进库的是：$ROWS"
  case "$ROWS" in
    *201*) bad "  └ node 201（source_type 空串）竟然进库 → CHECK 未生效！MySQL $VER" ;;
  esac
  case "$ROWS" in
    *202*) bad "  └ node 202（source_excerpt 空串）竟然进库 → CHECK 未生效！" ;;
  esac
  case "$ROWS" in
    *203*) bad "  └ node 203（source_url 空串）竟然进库 → CHECK 未生效！" ;;
  esac
  case "$ROWS" in
    *204*) bad "  └ node 204（source_type NULL）竟然进库 → NOT NULL 未生效！" ;;
  esac
fi

case "$ROWS" in
  100/*) ok "证据齐全的映射成功入库（正例，约束没过严）" ;;
  *)     bad "证据齐全的映射未能入库（约束过严）：$ROWS" ;;
esac

case "$ROWS" in
  *"/DRAFT"*) ok "status 默认 DRAFT（fail-safe：误插入的行不会直接对外可见）" ;;
  *)          bad "status 默认值不是 DRAFT：$ROWS" ;;
esac

echo ""
echo "═══════════════════════════════"
echo "  通过 $PASS 项 / 失败 $FAIL 项"
echo "═══════════════════════════════"
[ "$FAIL" -eq 0 ] || exit 1
