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
# 共表收敛（AC-O1）：本脚本按文件名顺序执行全部迁移，其中 20261002_converge_* 在"tm-stock 先建表"的库上
#   必须是空操作（下面断言它记录的分支是 already_converged）。"GVA 先启动 / tm-stock 先执行 / 重复多次"
#   各种形态的收敛由 server/internal/repository/migration_converge_integration_test.go 真实执行，
#   用 scripts/dev/verify-repository.sh 跑（CI 的 integration 任务也跑）。
#
# 用法：
#   bash scripts/dev/verify-migrations.sh          # 跑完自动清理
#   bash scripts/dev/verify-migrations.sh --keep   # 保留容器供手工排查
#   bash scripts/dev/verify-migrations.sh --self-test-probe # 仅探测函数自测，不连接数据库
#
# 没有 Docker 时的"原生模式"（2026-10-02 加入）：指定本机已有的 MySQL，不起容器，其余断言完全相同。
#   TM_VERIFY_MYSQL_CLIENT=/path/to/mysql.exe  TM_VERIFY_PASSWORD=...  [TM_VERIFY_PORT=3306]  \
#     [TM_VERIFY_HOST=127.0.0.1]  [TM_VERIFY_USER=root]  bash scripts/dev/verify-migrations.sh
#   ⚠️ 原生模式会 DROP 并重建数据库 tm_stock_verify，所以只允许连回环地址；口令只从环境变量读取，脚本里没有默认值。
#   用它可以在不同 MySQL 版本上各跑一遍（上线库是 9.7.0，本机与 CI 是 8.0.x，见 ADR-0008）。
# =============================================================================
set -uo pipefail

CONTAINER="tm-stock-verify-db"
DB="tm_stock_verify"
ROOT_PW="fake-local-verify-only"    # 一次性容器，不映射端口，不含真实数据
                                    # 用 fake- 前缀是本仓约定（见 check-secret-scan.sh 放行规则）
IMAGE="mysql:8.0"
KEEP=false
[ "${1:-}" = "--keep" ] && KEEP=true
# 原生模式：给了 mysql 客户端路径就不用 Docker（见上面的用法）
NATIVE=false
[ -n "${TM_VERIFY_MYSQL_CLIENT:-}" ] && NATIVE=true

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PASS=0; FAIL=0
ok()  { echo "  [OK] $1"; PASS=$((PASS+1)); }
bad() { echo "  [X]  $1"; FAIL=$((FAIL+1)); }

# 先完整读取客户端和去回车管道，再判断确认行；不能让grep -q提前关闭真实客户端的输出。
# 否则在pipefail下，即使已收到1，后续输出也可能使客户端/tr得到SIGPIPE，形成连接假阴性。
probe_native_mysql() {
  local probe_output probe_status=0
  probe_output="$(printf 'SELECT 1;\n' | native_sql)" || probe_status=$?
  if [ "$probe_status" -ne 0 ]; then
    echo "[X] 原生连接探针进程失败（退出码 $probe_status，客户端输出已隐藏）。"
    return "$probe_status"
  fi
  case $'\n'"$probe_output"$'\n' in
    *$'\n1\n'*) return 0 ;;
    *) echo "[X] 原生连接探针未返回确认行（客户端输出已隐藏）。"; return 1 ;;
  esac
}

if [ "${1:-}" = "--self-test-probe" ]; then
  # 仅此显式自测分支定义替身；默认原生/容器迁移路径不使用它，不写SQL或接触凭据。
  PROBE_CASE=''; PROBE_PASS=0; PROBE_FAIL=0
  native_sql() {
    local probe_query
    IFS= read -r probe_query || return 2
    [ "$probe_query" = 'SELECT 1;' ] || return 2
    case "$PROBE_CASE" in
      marker) printf '1\n' ;;
      diagnostic) printf 'test-only-probe-diagnostic\n1\ntest-only-probe-tail\n' ;;
      large) awk 'BEGIN { print "1"; for (i=0;i<50000;i++) print "test-only-probe-diagnostic" }' ;;
      empty) return 0 ;;
      wrong) printf '11\n' ;;
      padded) printf ' 1 \n' ;;
      client_error) printf '1\ntest-only-private-probe-marker\n'; return 7 ;;
      pipe_error) printf '1\ntest-only-private-probe-marker\n'; return 141 ;;
      *) return 2 ;;
    esac
  }
  check_probe() {
    local expected="$2" output rc=0
    PROBE_CASE="$1"
    output="$(probe_native_mysql)" || rc=$?
    if [ "$rc" -eq "$expected" ] && [[ "$output" != *test-only-private-probe-marker* ]]; then
      echo "  [OK] 探测反例/正例：$PROBE_CASE"
      PROBE_PASS=$((PROBE_PASS+1))
    else
      echo "  [X] 探测反例/正例：$PROBE_CASE（期望 $expected，实际 $rc，输出已隐藏）"
      PROBE_FAIL=$((PROBE_FAIL+1))
    fi
  }
  check_probe marker 0
  check_probe diagnostic 0
  check_probe large 0
  check_probe empty 1
  check_probe wrong 1
  check_probe padded 1
  check_probe client_error 7
  check_probe pipe_error 141
  echo "仅测试客户端探测函数，不替代真实迁移/约束断言。"
  echo "  通过 $PROBE_PASS 项 / 失败 $PROBE_FAIL 项"
  [ "$PROBE_FAIL" -eq 0 ] || exit 1
  exit 0
fi

cleanup() {
  if [ "$NATIVE" = true ]; then
    if [ "$KEEP" = false ] && type native_sql >/dev/null 2>&1; then
      printf 'DROP DATABASE IF EXISTS %s;\n' "$DB" | native_sql >/dev/null 2>&1 || true
    fi
    return
  fi
  if [ "$KEEP" = false ]; then
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  else
    echo ""
    echo "容器已保留：docker exec -it $CONTAINER mysql -uroot -p$ROOT_PW $DB"
  fi
}
trap cleanup EXIT

if [ "$NATIVE" = true ]; then
  # ── 原生模式：连本机已有的 MySQL，重建 $DB，不起容器 ──
  N_HOST="${TM_VERIFY_HOST:-127.0.0.1}"; N_PORT="${TM_VERIFY_PORT:-3306}"; N_USER="${TM_VERIFY_USER:-root}"
  case "$N_HOST" in
    127.0.0.1|localhost|::1) ;;
    *) echo "[X] 原生模式会 DROP DATABASE $DB，只允许连回环地址，收到 $N_HOST"; exit 1 ;;
  esac
  [ -n "${TM_VERIFY_PASSWORD:-}" ] || { echo "[X] 原生模式需要环境变量 TM_VERIFY_PASSWORD（脚本里不放默认口令）"; exit 1; }
  [ -f "$TM_VERIFY_MYSQL_CLIENT" ] || { echo "[X] 找不到 mysql 客户端：$TM_VERIFY_MYSQL_CLIENT"; exit 1; }
  native_sql() {
    MYSQL_PWD="$TM_VERIFY_PASSWORD" "$TM_VERIFY_MYSQL_CLIENT" --no-defaults -h"$N_HOST" -P"$N_PORT" -u"$N_USER" \
      --protocol=TCP --default-character-set=utf8mb4 --force -N -B "$@" 2>&1 | tr -d '\r'
  }
  echo "▶ 原生模式：连 $N_HOST:$N_PORT，重建数据库 $DB（不用 Docker）"
  if ! probe_native_mysql; then
    echo "[X] 连不上 MySQL（$N_HOST:$N_PORT）。"
    exit 1
  fi
  printf 'DROP DATABASE IF EXISTS %s;\nCREATE DATABASE %s CHARACTER SET utf8mb4;\n' "$DB" "$DB" | native_sql >/dev/null
  # 与 Docker 模式的 MYSQL_RUN 同一个口径：--force、-N -B、utf8mb4、去掉回车
  MYSQL_RUN() { native_sql "$DB"; }
else
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

# ★ --default-character-set=utf8mb4 不能省（实测踩过）：
#   不指定时 mysql CLI 按系统 locale 协商连接字符集，管道进来的 UTF-8 中文
#   会被当 latin1 再编一次存进库（双重编码）。列本身是 utf8mb4 也救不了。
#   症状很隐蔽：ID 与计数类断言全绿，只有中文取出来是乱码。
MYSQL_RUN() { docker exec -i "$CONTAINER" mysql -h 127.0.0.1 --protocol=TCP \
                -uroot -p"$ROOT_PW" --default-character-set=utf8mb4 \
                --force -N -B "$DB" 2>&1 | tr -d '\r'; }
fi

VER="$(printf 'SELECT VERSION();\n' | MYSQL_RUN | grep -v Warning | head -1)"
# ★ 空结果守卫：初版没有这条，于是拿着空结果一路跑出一堆假失败，
#   把「连不上」误报成「表全缺失 + 约束全失效」。空结果必须当错误处理。
if [ -z "$VER" ]; then
  echo "[X] 已就绪但取不到版本号 —— 连接异常，中止（不带着空结果继续断言）"
  [ "$NATIVE" = true ] || docker logs --tail 15 "$CONTAINER"
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

-- 账本迁移经 mysql 客户端执行也必须创建完整触发器；DELIMITER 错误不能静默漏过。
SELECT 'POINTS_CHECKS', COUNT(*) FROM information_schema.table_constraints
 WHERE constraint_schema=DATABASE() AND table_name LIKE 'tm_points_%' AND constraint_type='CHECK' AND enforced='YES';
SELECT 'POINTS_TRIGGERS', COUNT(*) FROM information_schema.triggers
 WHERE trigger_schema=DATABASE() AND event_object_table LIKE 'tm_points_%';
SELECT 'TASK_CHECKS', COUNT(*) FROM information_schema.table_constraints
 WHERE constraint_schema=DATABASE() AND table_name IN ('tm_ai_tasks','tm_ai_task_events') AND constraint_type='CHECK' AND enforced='YES' AND constraint_name<>'chk_tm_ai_task_admission';
SELECT 'TASK_TRIGGERS', COUNT(*) FROM information_schema.triggers
 WHERE trigger_schema=DATABASE() AND event_object_table IN ('tm_ai_tasks','tm_ai_task_events') AND trigger_name<>'tm_ai_task_admission_identity';
SELECT 'TASK_BUDGET_CHECKS', COUNT(*) FROM information_schema.table_constraints
 WHERE constraint_schema=DATABASE() AND table_name IN ('tm_ai_task_budgets','tm_ai_task_reservations') AND constraint_type='CHECK' AND enforced='YES';
SELECT 'TASK_BUDGET_TRIGGERS', COUNT(*) FROM information_schema.triggers
 WHERE trigger_schema=DATABASE() AND event_object_table IN ('tm_ai_task_budgets','tm_ai_task_reservations');
SELECT 'TASK_STAGE_CHECKS', COUNT(*) FROM information_schema.table_constraints
 WHERE constraint_schema=DATABASE() AND table_name IN ('tm_ai_task_step_results','tm_ai_task_steps') AND constraint_type='CHECK' AND enforced='YES';
SELECT 'TASK_STAGE_TRIGGERS', COUNT(*) FROM information_schema.triggers
 WHERE trigger_schema=DATABASE() AND event_object_table IN ('tm_ai_task_step_results','tm_ai_task_steps');
SELECT 'REPORT_CHECKS', COUNT(*) FROM information_schema.table_constraints
 WHERE constraint_schema=DATABASE() AND table_name='tm_ai_report_commits' AND constraint_type='CHECK' AND enforced='YES';
SELECT 'REPORT_TRIGGERS', COUNT(*) FROM information_schema.triggers
 WHERE trigger_schema=DATABASE() AND event_object_table='tm_ai_report_commits';
SELECT 'ADMISSION_CHECKS',COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND constraint_name IN ('chk_tm_ai_task_admission','chk_tm_ai_admission') AND constraint_type='CHECK' AND enforced='YES';
SELECT 'ADMISSION_TRIGGERS',COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND (event_object_table='tm_ai_task_admissions' OR trigger_name='tm_ai_task_admission_identity');
START TRANSACTION;
INSERT INTO tm_points_accounts(user_id,avail) VALUES (900000001,10);
INSERT INTO tm_points_accounts(user_id,avail) VALUES (900000002,-1);
SELECT 'POINTS_POSITIVE', COUNT(*) FROM tm_points_accounts WHERE user_id=900000001 AND avail=10;
SELECT 'POINTS_NEGATIVE', COUNT(*) FROM tm_points_accounts WHERE user_id=900000002;
UPDATE tm_points_accounts SET created_at=created_at+INTERVAL 1 SECOND WHERE user_id=900000001;
SELECT 'POINTS_IDENTITY', COUNT(*) FROM tm_points_accounts WHERE user_id=900000001 AND created_at=updated_at;
DELETE FROM tm_points_accounts WHERE user_id=900000001;
SELECT 'POINTS_NODELETE', COUNT(*) FROM tm_points_accounts WHERE user_id=900000001;
ROLLBACK;
SELECT 'POINTS_ROLLBACK', COUNT(*) FROM tm_points_accounts;
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
CONVERGE_STATE="$(f converge_addon_quant_theme_stock)"

# ── 5) 断言 ──
echo ""
echo "▶ 表结构"
for t in addon_quant_theme addon_quant_base_stock addon_quant_theme_stock; do
  case ",$TABLES," in *",$t,"*) ok "表 $t 存在" ;; *) bad "表 $t 缺失（TABLES=$TABLES）" ;; esac
done
for t in tm_points_accounts tm_points_txns tm_points_lots tm_points_entries tm_points_holds tm_points_hold_allocs tm_points_debts tm_points_debt_allocs; do
  case ",$TABLES," in *",$t,"*) ok "表 $t 存在" ;; *) bad "账本表 $t 缺失" ;; esac
done
for t in tm_ai_tasks tm_ai_task_events tm_ai_task_budgets tm_ai_task_reservations tm_ai_task_step_results tm_ai_task_steps tm_ai_report_commits tm_ai_task_admissions; do
  case ",$TABLES," in *",$t,"*) ok "表 $t 存在" ;; *) bad "任务表 $t 缺失" ;; esac
done
[ "$(f TASK_CHECKS)" = "3" ] && ok "任务 3 个 CHECK 强制执行" || bad "任务 CHECK 缺失或未强制执行"
[ "$(f TASK_TRIGGERS)" = "6" ] && ok "任务 6 个不可变/阶段触发器完整" || bad "任务触发器创建不完整"
[ "$(f TASK_BUDGET_CHECKS)" = "2" ] && ok "预算 2 个 CHECK 强制执行" || bad "预算 CHECK 缺失或未强制执行"
[ "$(f TASK_BUDGET_TRIGGERS)" = "7" ] && ok "预算 7 个触发器完整" || bad "预算触发器创建不完整"
[ "$(f TASK_STAGE_CHECKS)" = "2" ] && ok "阶段 2 个 CHECK 强制执行" || bad "阶段 CHECK 缺失或未强制执行"
[ "$(f TASK_STAGE_TRIGGERS)" = "7" ] && ok "阶段 7 个触发器完整" || bad "阶段触发器创建不完整"
[ "$(f REPORT_CHECKS)" = "1" ] && ok "报告 CHECK 强制执行" || bad "报告 CHECK 缺失或未强制执行"
[ "$(f REPORT_TRIGGERS)" = "3" ] && ok "报告 3 个触发器完整" || bad "报告触发器创建不完整"
[ "$(f ADMISSION_CHECKS)" = "2" ] && ok "准入 2 个 CHECK 强制执行" || bad "准入 CHECK 缺失或未强制执行"
[ "$(f ADMISSION_TRIGGERS)" = "4" ] && ok "准入 4 个不可变/绑定触发器完整" || bad "准入触发器不完整"

[ "$(f POINTS_CHECKS)" = "8" ] && ok "账本 8 个 CHECK 强制执行" || bad "账本 CHECK 缺失或未强制执行"
[ "$(f POINTS_TRIGGERS)" = "16" ] && ok "账本 16 个触发器完整" || bad "账本触发器创建不完整"
[ "$(f POINTS_POSITIVE)" = "1" ] && ok "账本正例可写" || bad "账本正例未写入"
[ "$(f POINTS_NEGATIVE)" = "0" ] && ok "账本负余额被拒" || bad "账本负余额未拒绝"
[ "$(f POINTS_IDENTITY)" = "1" ] && ok "账本账户身份不可改" || bad "账本账户身份保护失效"
[ "$(f POINTS_NODELETE)" = "1" ] && ok "账本账户不可删除" || bad "账本账户删除保护失效"
[ "$(f POINTS_ROLLBACK)" = "0" ] && ok "账本探针已回滚" || bad "账本探针遗留数据"

echo ""
echo "▶ 共表收敛迁移"
[ "$CONVERGE_STATE" = "already_converged" ] \
  && ok "收敛迁移在 tm-stock 先建表的库上是空操作（already_converged）" \
  || bad "收敛迁移记录的分支应为 already_converged，实际「$CONVERGE_STATE」——空值表示它没执行成功或中途中止"

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
