#!/bin/bash
# =============================================================================
# Harness Check —— 质量门禁（L3）  tm-stock
# 用法：bash scripts/harness-checks.sh [--with-lint] [--with-coverage]
#
# ★ 原则：宁可关掉也不要常红——门禁长期红灯就会被忽略，等于没有门禁。
# ★ 铁律：Agent 自述"做完了"不算数，本脚本绿灯才算数。
# =============================================================================
set -uo pipefail
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'

WITH_LINT=false; WITH_COVERAGE=false
for a in "$@"; do case $a in --with-lint) WITH_LINT=true;; --with-coverage) WITH_COVERAGE=true;; esac; done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$REPO_ROOT"
RESULT=0

# run_selftest <说明> <命令...>：跑一个门禁自测。通过时只显示结论行，失败时打印输出的末尾。
# 门禁失效是【静默】的（照样打印 OK，只是什么都没拦住），所以门禁本身必须有测试，且测试必须一起跑。
run_selftest() {
  local label="$1" out rc summary
  shift
  out="$("$@" 2>&1)"; rc=$?
  if [ "$rc" -eq 0 ]; then
    summary="$(printf '%s' "$out" | grep -E '通过 [0-9]+ 项' | tail -1 | sed 's/[═ ]//g')"
    echo -e "${GREEN}  ✅ ${label}${NC}${summary:+（$summary）}"
  else
    echo -e "${RED}  ❌ ${label}${NC}"
    printf '%s' "$out" | tail -40
    RESULT=1
  fi
}

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  tm-stock Harness Check${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# [1] 合规禁用词（本项目最高优先级）
echo -e "\n${BLUE}[1/4] 合规禁用词扫描...${NC}"
if bash scripts/check-compliance-words.sh --all; then
  echo -e "${GREEN}  ✅ 合规词检查通过${NC}"
else
  echo -e "${RED}  ❌ 存在合规禁用词（公司级风险，必须修）${NC}"; RESULT=1
fi
run_selftest "合规词门禁自测（范围与三种模式口径）" bash scripts/dev/verify-compliance-gate.sh
run_selftest "词表副本与单一源一致（GVA 与 ai/guard 各一份）" bash scripts/sync-guard-words.sh --check

# [2] 明文密钥
echo -e "\n${BLUE}[2/4] 明文密钥扫描...${NC}"
if bash scripts/check-secret-scan.sh --all; then
  echo -e "${GREEN}  ✅ 未发现明文密钥${NC}"
else
  echo -e "${RED}  ❌ 发现疑似明文密钥${NC}"; RESULT=1
fi
run_selftest "密钥扫描器自测（正反例）" bash scripts/dev/verify-secret-scan.sh

# [3] Go 后端
echo -e "\n${BLUE}[3/4] Go 后端（格式 / 编译 / 测试）...${NC}"
if [ ! -d server ]; then
  echo -e "${YELLOW}  跳过（无 server/ 目录）${NC}"
elif ! command -v go >/dev/null 2>&1; then
  echo -e "${YELLOW}  跳过（本机未安装 Go —— 安装后此项才生效）${NC}"
else
  cd server
  UNFMT="$(gofmt -l . 2>/dev/null | grep -v vendor || true)"
  [ -n "$UNFMT" ] && { echo -e "${RED}  ❌ 未格式化：${NC}"; echo "$UNFMT"; RESULT=1; }
  if go build ./... 2>&1 | tail -15; then echo -e "${GREEN}  ✅ 编译通过${NC}"; else echo -e "${RED}  ❌ 编译失败${NC}"; RESULT=1; fi
  go vet ./... 2>&1 | tail -15 && echo -e "${GREEN}  ✅ vet 通过${NC}" || { echo -e "${RED}  ❌ vet 未通过${NC}"; RESULT=1; }
  # ★ -race 需要 cgo，而 cgo 需要 C 编译器。Windows 开发机通常没有 gcc/clang，
  #   此时 `go test -race` 直接报 "-race requires cgo" —— 那是【环境限制】，不是测试失败。
  #   照原样标红会让本地门禁长期红灯，按 P9 就等于门禁失效。
  #   → 本地自动降级为不带 -race；CI（Linux）仍然强制 -race，不放宽标准。
  RACE_FLAG="-race"
  if [ "$(go env CGO_ENABLED 2>/dev/null)" != "1" ] \
     || ! { command -v gcc >/dev/null 2>&1 || command -v clang >/dev/null 2>&1; }; then
    RACE_FLAG=""
    echo -e "${YELLOW}  本机无 C 编译器 → 降级为不带 -race（CI 在 Linux 上仍强制 -race）${NC}"
  fi
  if go test ./... $RACE_FLAG -count=1 2>&1 | tail -20; then
    echo -e "${GREEN}  ✅ 测试通过${NC}"
  else
    echo -e "${RED}  ❌ 测试失败${NC}"; RESULT=1
  fi
  if [ "$WITH_LINT" = true ] && command -v golangci-lint >/dev/null 2>&1; then
    golangci-lint run ./... 2>&1 | tail -20 || { echo -e "${RED}  ❌ lint 未通过${NC}"; RESULT=1; }
  fi
  if [ "$WITH_COVERAGE" = true ]; then
    go test ./... -coverprofile=coverage.out >/dev/null 2>&1 || true
    [ -f coverage.out ] && echo "  覆盖率：$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}')（新增代码目标 ≥70%）"
    # 关键包（护栏、渲染器、模型适配层）的覆盖率棘轮，下限登记在 scripts/coverage-floors.txt
    (cd "$REPO_ROOT" && bash scripts/check-coverage-floor.sh) || { echo -e "${RED}  ❌ 关键包覆盖率低于登记的下限${NC}"; RESULT=1; }
  fi
  cd "$REPO_ROOT"
fi

# [3.2] GVA 后端（backend/server）关键包：令牌类型隔离、题材股票依据与审核、AI 任务下线、迁移守卫
echo -e "\n${BLUE}[3.2/4] GVA 后端关键包...${NC}"
if [ ! -d backend/server ]; then
  echo -e "${YELLOW}  跳过（无 backend/server 目录）${NC}"
elif ! command -v go >/dev/null 2>&1; then
  echo -e "${YELLOW}  跳过（本机未安装 Go）${NC}"
else
  # 包清单与 .github/workflows/ci.yml 的 gva-backend 作业保持一致
  GVA_PKGS="./middleware/... ./service/member/... ./router/... ./api/client/... ./initialize/... ./service/quant/... ./utils"
  if (cd backend/server && go test $GVA_PKGS -count=1 2>&1 | tail -20); then
    echo -e "${GREEN}  ✅ GVA 关键包测试通过${NC}"
  else
    echo -e "${RED}  ❌ GVA 关键包测试失败${NC}"; RESULT=1
  fi
fi

# [3.5] Go 架构约定（分层守护）
echo -e "\n${BLUE}[3.5/4] Go 架构约定...${NC}"
if bash scripts/check-architecture.sh; then
  echo -e "${GREEN}  ✅ 分层约定通过${NC}"
else
  echo -e "${RED}  ❌ 违反分层约定${NC}"; RESULT=1
fi

# [4] uni-app x 前端静态检查
echo -e "\n${BLUE}[4/4] 前端（uni-app x）...${NC}"
if [ -f pages.json ]; then
  # 校验 pages.json 是否合法 JSON（uni-app x 无 CLI lint，这是能自动做的最有价值检查）
  if command -v node >/dev/null 2>&1; then
    node -e "JSON.parse(require('fs').readFileSync('pages.json','utf8'))" 2>/dev/null \
      && echo -e "${GREEN}  ✅ pages.json 合法${NC}" \
      || { echo -e "${RED}  ❌ pages.json 不是合法 JSON${NC}"; RESULT=1; }
  fi
  echo -e "${YELLOW}  提示：uni-app x 需在 HBuilderX 内编译验证（无 CLI 构建）${NC}"
else
  echo -e "${YELLOW}  跳过${NC}"
fi

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
[ $RESULT -eq 0 ] && echo -e "${GREEN}  ✅ 全部通过，可以提交。${NC}" || echo -e "${RED}  ❌ 未通过，请修复后重跑。${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
exit $RESULT
