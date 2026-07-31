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
# 2026-07-30 实测（MySQL 8.0.45），两种拒绝各自点名了机制：
#   证据字段为空串 → ERROR 3819: Check constraint ... is violated.
#   证据字段为 NULL → ERROR 1048: Column ... cannot be null
#
# ⚠️ 上游两表（addon_quant_theme / addon_quant_base_stock）由【现有系统】维护
#    （ADR-0007），本仓库不含其建表脚本。下面的 UPSTREAM_DDL 是【测试专用】的
#    最小复刻，仅为让关联表能建起来，**不是 schema 权威来源**。
#
# ★ 设计要点：宿主机进程数压到最少（约 5 个）。
#   初版每次探测都 docker exec，共 ~80 个宿主进程，
#   在内存吃紧的 Windows 上直接把 Git Bash 的 fork 打崩。
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
#    → 改用 TCP 连接：初始化期不监听网络，TCP 通了才是真服务器起来了。
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

MYSQL_RUN() { docker exec -i "$CONTAINER" mysql -h 127.0.0.1 --protocol=TCP \
                -uroot -p"$ROOT_PW" --force -N -B "$DB" 2>&1 | tr -d '\r'; }

VER="$(printf 'SELECT VERSION();\n' | MYSQL_RUN | grep -v Warning | head -1)"
# ★ 空结果守卫：初版没有这条，于是拿着空结果一路跑出一堆假失败，
#   把「连不上」误报成「表全缺失 + 约束全失效」。空结果必须当错误处理。
if [ -z "$VER" ]; then
  echo "[X] 已就绪但取不到版本号 —— 连接异常，中止（不带着空结果继续断言）"
  docker logs --tail 15 "$CONTAINER"
  exit 1
fi
echo "  MySQL 版本：$VER"

# ── 3) 上游两表的【测试专用】最小复刻 ──
UPSTREAM_DDL="
CREATE TABLE addon_quant_theme (
  id bigint UNSIGNED NOT NULL AUTO_INCREMENT, name varchar(100) DEFAULT NULL,
  code varchar(100) DEFAULT NULL, level tinyint DEFAULT 0, parent_id bigint DEFAULT 0,
  description varchar(250) DEFAULT NULL, remark varchar(250) DEFAULT NULL,
  source tinyint DEFAULT 0, sort int DEFAULT 0, status tinyint DEFAULT 1,
  created_at datetime(3) DEFAULT NULL, updated_at datetime(3) DEFAULT NULL,
  deleted_at datetime(3) DEFAULT NULL,
  PRIMARY KEY (id), UNIQUE KEY uk_source_code (source, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE addon_quant_base_stock (
  id bigint UNSIGNED NOT NULL AUTO_INCREMENT, ts_code varchar(20) DEFAULT NULL,
  symbol varchar(10) DEFAULT NULL, name varchar(50) DEFAULT NULL,
  industry varchar(50) DEFAULT NULL, cnspell varchar(50) DEFAULT NULL,
  market varchar(20) DEFAULT NULL, exchange varchar(10) DEFAULT NULL,
  list_status varchar(10) DEFAULT NULL,
  created_at datetime(3) DEFAULT NULL, updated_at datetime(3) DEFAULT NULL,
  deleted_at datetime(3) DEFAULT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT INTO addon_quant_theme(id,name,level,parent_id,created_at,updated_at)
  VALUES (100001,'光刻机',1,0,NOW(3),NOW(3)),(100010,'光源',2,100001,NOW(3),NOW(3));
INSERT INTO addon_quant_base_stock(id,ts_code,symbol,name,created_at,updated_at)
  VALUES (1,'688502.SH','688502','茂莱光学',NOW(3),NOW(3)),
         (2,'000001.SZ','000001','平安银行',NOW(3),NOW(3));
"

shopt -s nullglob
MIGS=("$REPO_ROOT"/server/migrations/*.sql)
[ ${#MIGS[@]} -eq 0 ] && { echo "[跳过] 无迁移脚本"; exit 0; }

echo ""
echo "▶ 建上游测试表 + 执行 ${#MIGS[@]} 个迁移 + 实测约束（单次 exec）"

# ── 4) 断言用数据 ──
# 判据是【实际落库结果】而不是逐条退出码：验最终状态比验中间信号更强。
ASSERT_SQL="
-- 正例：证据齐全 → 应成功（环节 100010）
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,created_at)
 VALUES (100010,1,'688502.SH',2,'光源模组业务收入占比34.2%','https://e.com/r',NOW(3),NOW(3));
-- 正例：同一股票挂到【另一个】题材 → 应成功（多对多的核心诉求）
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,created_at)
 VALUES (100001,1,'688502.SH',2,'公司属光刻机产业链','https://e.com/r3',NOW(3),NOW(3));
-- 反例：摘录空串 → CHECK 应拒
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,created_at)
 VALUES (100010,2,'000001.SZ',2,'','https://e.com/r',NOW(3),NOW(3));
-- 反例：链接空串 → CHECK 应拒
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,created_at)
 VALUES (100010,2,'000001.SZ',2,'摘录','',NOW(3),NOW(3));
-- 反例：source_type=0 → CHECK 应拒
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,created_at)
 VALUES (100010,2,'000001.SZ',0,'摘录','https://e.com/r',NOW(3),NOW(3));
-- 反例：摘录为 NULL → NOT NULL 应拒
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,created_at)
 VALUES (100010,2,'000001.SZ',2,NULL,'https://e.com/r',NOW(3),NOW(3));
-- 反例：同环节重复挂同一股票 → 唯一键应拒
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,created_at)
 VALUES (100010,1,'688502.SH',1,'另一段摘录','https://e.com/r2',NOW(3),NOW(3));

SELECT 'TABLES', GROUP_CONCAT(table_name ORDER BY table_name)
  FROM information_schema.tables WHERE table_schema=DATABASE();
SELECT 'ROWS', IFNULL(GROUP_CONCAT(CONCAT(theme_id,'/',audit_status) ORDER BY theme_id),'NONE')
  FROM addon_quant_theme_stock WHERE deleted_at IS NULL;
SELECT 'COUNT', COUNT(*) FROM addon_quant_theme_stock;

-- 软删除后重新添加同一对 → 应成功（生成列 alive 的价值）
UPDATE addon_quant_theme_stock SET deleted_at=NOW(3) WHERE theme_id=100010 AND stock_id=1;
INSERT INTO addon_quant_theme_stock
 (theme_id,stock_id,ts_code,source_type,source_excerpt,source_url,collected_at,created_at)
 VALUES (100010,1,'688502.SH',2,'重新收录','https://e.com/r4',NOW(3),NOW(3));
SELECT 'AFTER_READD', COUNT(*) FROM addon_quant_theme_stock;

-- C 端三重过滤：未审核时应为 0
SELECT 'CSIDE_BEFORE_AUDIT', COUNT(*) FROM addon_quant_theme_stock ts
  JOIN addon_quant_base_stock s ON s.id=ts.stock_id AND s.deleted_at IS NULL
  JOIN addon_quant_theme      t ON t.id=ts.theme_id AND t.deleted_at IS NULL
 WHERE ts.deleted_at IS NULL AND ts.audit_status=2 AND ts.status=1;
UPDATE addon_quant_theme_stock SET audit_status=2 WHERE deleted_at IS NULL;
SELECT 'CSIDE_AFTER_AUDIT', COUNT(*) FROM addon_quant_theme_stock ts
  JOIN addon_quant_base_stock s ON s.id=ts.stock_id AND s.deleted_at IS NULL
  JOIN addon_quant_theme      t ON t.id=ts.theme_id AND t.deleted_at IS NULL
 WHERE ts.deleted_at IS NULL AND ts.audit_status=2 AND ts.status=1;
"

OUT="$( { printf '%s\n' "$UPSTREAM_DDL"; cat "${MIGS[@]}"; printf '%s\n' "$ASSERT_SQL"; } | MYSQL_RUN )"

if ! printf '%s\n' "$OUT" | grep -q '^TABLES'; then
  echo "[X] SQL 执行未返回结果，中止。原始输出前 20 行："
  printf '%s\n' "$OUT" | head -20
  exit 1
fi

f() { printf '%s\n' "$OUT" | awk -F'\t' -v k="$1" '$1==k{print $2}'; }
TABLES="$(f TABLES)"; ROWS="$(f ROWS)"; COUNT="$(f COUNT)"
AFTER_READD="$(f AFTER_READD)"; C_BEFORE="$(f CSIDE_BEFORE_AUDIT)"; C_AFTER="$(f CSIDE_AFTER_AUDIT)"

# ── 5) 断言 ──
echo ""
echo "▶ 表结构"
for t in addon_quant_theme addon_quant_base_stock addon_quant_theme_stock; do
  case ",$TABLES," in *",$t,"*) ok "表 $t 存在" ;; *) bad "表 $t 缺失（TABLES=$TABLES）" ;; esac
done

echo ""
echo "▶ 🔴 合规约束实测（判据 = 实际落库结果）"
echo "  存活行：$ROWS   总行数：$COUNT"

[ "$COUNT" = "2" ] && ok "7 条 INSERT 只有 2 条进库 —— 5 条脏数据全部被拒" \
                   || bad "应只有 2 条进库，实际 $COUNT 条。进库的是：$ROWS"

case "$ROWS" in
  *100010*) ok "证据齐全的映射成功入库（正例，约束没过严）" ;;
  *)        bad "证据齐全的映射未能入库（约束过严）：$ROWS" ;;
esac
case "$ROWS" in
  *100001*) ok "同一股票可挂多个题材（多对多成立）" ;;
  *)        bad "跨题材映射未入库：$ROWS" ;;
esac
case "$ROWS" in
  *"/0"*) ok "audit_status 默认 0=草稿（fail-safe：误插入的行不对外可见）" ;;
  *)      bad "audit_status 默认值不是草稿：$ROWS" ;;
esac

[ "$AFTER_READD" = "3" ] && ok "软删除后可重新添加同一对（生成列 alive 生效）" \
                         || bad "软删后重加失败，总行数 $AFTER_READD（期望 3）"

echo ""
echo "▶ C 端三重过滤"
[ "$C_BEFORE" = "0" ] && ok "未审核时 C 端查到 0 条" || bad "未审核却能查到 $C_BEFORE 条 —— 过滤失效！"
[ "$C_AFTER" = "2" ]  && ok "审核通过后 C 端查到 2 条" || bad "审核后应查到 2 条，实际 $C_AFTER"

echo ""
echo "═══════════════════════════════"
echo "  通过 $PASS 项 / 失败 $FAIL 项"
echo "═══════════════════════════════"
[ "$FAIL" -eq 0 ] || exit 1
