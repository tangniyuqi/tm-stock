#!/bin/bash
# =============================================================================
# repository 集成测试 —— 起一次性 MySQL，把每一条 SQL 真执行一遍。
#
# 为什么需要它：
#   theme_sql_test.go 只能断言「SQL 字符串里含某些片段」，
#   查不出【列名拼错、表名写错、类型不匹配、JOIN 条件写反、Scan 顺序错位】——
#   那些只有真执行才暴露，而且上线后表现为接口 500 或数据串列。
#
# 与 verify-migrations.sh 的分工：
#   verify-migrations.sh  验【DDL 与约束】是否真生效（CHECK/唯一键/默认值）
#   verify-repository.sh  验【Go 里的 DML】能不能跑通、取出来的值对不对
#
# 用法：bash scripts/dev/verify-repository.sh
# =============================================================================
set -uo pipefail

CONTAINER="tm-stock-repo-it"
DB="tm_stock_it"
ROOT_PW="fake-local-verify-only"   # 一次性容器；用 fake- 前缀是本仓约定
IMAGE="mysql:8.0"
# 端口刻意避开 3306/3307：3307 已被 MLR 项目的 mlr-mysql-local 占用
PORT=13306

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

cleanup() { docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

# 环境不具备时【优雅跳过】而不是失败（known-pitfalls P9）
command -v docker >/dev/null 2>&1 || { echo "[跳过] 本机无 docker"; exit 0; }
docker version --format '{{.Server.Version}}' >/dev/null 2>&1 || {
  echo "[跳过] docker daemon 未运行（Windows 上需先启动 Docker Desktop）"; exit 0; }
docker image inspect "$IMAGE" >/dev/null 2>&1 || {
  echo "[跳过] 本地无 $IMAGE 镜像，先执行：docker pull $IMAGE"; exit 0; }
command -v go >/dev/null 2>&1 || { echo "[跳过] 本机未安装 Go"; exit 0; }

if command -v ss >/dev/null 2>&1 && ss -ltn 2>/dev/null | grep -q ":$PORT "; then
  echo "[X] 端口 $PORT 已被占用，请先释放或改脚本里的 PORT"; exit 1
fi

echo "▶ 启动 MySQL（映射 127.0.0.1:$PORT，仅本机可连）..."
docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$CONTAINER" \
  -e MYSQL_ROOT_PASSWORD="$ROOT_PW" -e MYSQL_DATABASE="$DB" \
  -p "127.0.0.1:$PORT:3306" "$IMAGE" >/dev/null || { echo "[X] 容器启动失败"; exit 1; }

# 就绪判据用 TCP 而不是 mysqladmin ping：
# 首次初始化时会先起一个 --skip-networking 的临时服务器，ping 对它会立刻成功，
# 而真服务器还没起来 —— 撞进这个窗口后续全部查空（verify-migrations.sh 踩过）。
echo "▶ 等待就绪（TCP 探测，循环在容器内）..."
if ! docker exec "$CONTAINER" bash -c '
    for i in $(seq 1 90); do
      if mysql -h 127.0.0.1 --protocol=TCP -uroot -p"$MYSQL_ROOT_PASSWORD" \
           -N -B -e "SELECT 1" >/dev/null 2>&1; then exit 0; fi
      sleep 2
    done; exit 1' >/dev/null 2>&1; then
  echo "[X] 180 秒内未就绪"; docker logs --tail 15 "$CONTAINER"; exit 1
fi
echo "  容器内已就绪"

# 容器内就绪不等于宿主机能连上（端口转发要单独等）
echo "▶ 等待宿主机端口可连..."
READY=false
for i in $(seq 1 30); do
  if (exec 3<>/dev/tcp/127.0.0.1/$PORT) 2>/dev/null; then exec 3>&- 2>/dev/null; READY=true; break; fi
  sleep 2
done
[ "$READY" = true ] || { echo "[X] 宿主机 60 秒内连不上 127.0.0.1:$PORT"; exit 1; }
echo "  端口可连"

# multiStatements=true：测试要一次执行整份迁移脚本
# parseTime=true：datetime(3) 才能扫进 time.Time（缺了会报 []uint8 无法 Scan）
# loc=Local + charset：避免时区与编码坑
export TM_TEST_DSN="root:${ROOT_PW}@tcp(127.0.0.1:${PORT})/${DB}?parseTime=true&multiStatements=true&charset=utf8mb4&loc=Local"

echo ""
echo "▶ 执行集成测试（-tags=integration）"
cd "$REPO_ROOT/server" || exit 1
go test -tags=integration ./internal/repository/... -count=1 -v 2>&1 | tee /tmp/it.log | grep -E '^(=== RUN|--- (PASS|FAIL|SKIP)|ok|FAIL|panic)' || true
RC=${PIPESTATUS[0]}

PASSN=$(grep -c '^[[:space:]]*--- PASS' /tmp/it.log || true)
FAILN=$(grep -c '^[[:space:]]*--- FAIL' /tmp/it.log || true)
SKIPN=$(grep -c '^[[:space:]]*--- SKIP' /tmp/it.log || true)

echo ""
echo "═══════════════════════════════"
echo "  通过 $PASSN / 失败 $FAILN / 跳过 $SKIPN"
echo "═══════════════════════════════"
if [ "$FAILN" -gt 0 ]; then
  echo "失败明细："; grep -A6 '^[[:space:]]*--- FAIL' /tmp/it.log | head -40
fi
# 一条都没跑 = 没测到，等同失败（不能让 SKIP 冒充通过）
if [ "$PASSN" -eq 0 ]; then
  echo "[X] 没有任何测试被执行 —— 检查 TM_TEST_DSN 与构建标签"
  rm -f /tmp/it.log
  exit 1
fi
rm -f /tmp/it.log
exit "$RC"
