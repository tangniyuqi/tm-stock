#!/bin/bash
# =============================================================================
# 覆盖率棘轮 —— 关键包的测试覆盖率不得低于 scripts/coverage-floors.txt 里登记的下限
#
# 用法：bash scripts/check-coverage-floor.sh      （在任意目录运行；会在 server/ 下执行 go test -cover）
#
# ★ 为什么是棘轮：只设"≥ 70%"的一次性门槛，覆盖率会在之后的改动里悄悄往下掉，没人发现。
#   这里把当前水平（略低一点，留出正常波动）登记为下限；覆盖率明显高于下限时会提示"可上调"，
#   上调只增不减——调低下限必须在 PR 里说明理由。
# ★ 只管登记在册的包：它们是合规与计费链路上的关键代码（护栏、渲染器、模型适配层），
#   不是全仓库的覆盖率指标。新增关键包时，在 coverage-floors.txt 里加一行。
# =============================================================================
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FLOORS="$REPO_ROOT/scripts/coverage-floors.txt"
[ -f "$FLOORS" ] || { echo "[X] 找不到 $FLOORS"; exit 1; }
command -v go >/dev/null 2>&1 || { echo "[跳过] 本机未安装 Go"; exit 0; }

# 读取登记：每行 "<相对 server/ 的包路径> <下限（整数百分比）>"，# 开头为注释
PKGS=""
while read -r pkg floor _; do
  case "$pkg" in ''|'#'*) continue ;; esac
  PKGS="$PKGS ./$pkg"
done < "$FLOORS"
[ -n "$PKGS" ] || { echo "[X] $FLOORS 里没有登记任何包"; exit 1; }

cd "$REPO_ROOT/server" || exit 1
# shellcheck disable=SC2086
OUT="$(go test -count=1 -cover $PKGS 2>&1)" || { printf '%s\n' "$OUT" | tail -30; echo "[X] 测试失败，无法评估覆盖率"; exit 1; }

FAIL=0
while read -r pkg floor _; do
  case "$pkg" in ''|'#'*) continue ;; esac
  # 取该包那一行里 "coverage: 98.5% of statements" 的数字
  cov="$(printf '%s\n' "$OUT" | grep -E "[[:space:]]([^[:space:]]*/)?$pkg[[:space:]]" | sed -nE 's/.*coverage: ([0-9]+(\.[0-9]+)?)% of statements.*/\1/p' | head -1)"
  if [ -z "$cov" ]; then
    echo "[X] $pkg：没有取到覆盖率（包没被测到，或输出格式变了）"; FAIL=1; continue
  fi
  if awk -v a="$cov" -v f="$floor" 'BEGIN { exit !(a + 0 >= f + 0) }'; then
    if awk -v a="$cov" -v f="$floor" 'BEGIN { exit !(a + 0 >= f + 3) }'; then
      echo "[OK] $pkg：${cov}%（下限 ${floor}%，已高出 3 个点以上，建议把下限上调到 $(awk -v a="$cov" 'BEGIN { printf "%d", a - 2 }')）"
    else
      echo "[OK] $pkg：${cov}%（下限 ${floor}%）"
    fi
  else
    echo "[X] $pkg：${cov}% 低于下限 ${floor}%"; FAIL=1
  fi
done < "$FLOORS"

if [ "$FAIL" -ne 0 ]; then
  echo "[修复] 给新增或改动的代码补测试；不得为过门禁而调低 scripts/coverage-floors.txt 里的下限"
  exit 1
fi
echo "[OK] 关键包覆盖率均不低于登记的下限"
