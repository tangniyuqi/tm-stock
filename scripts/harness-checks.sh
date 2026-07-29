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

# [2] 明文密钥
echo -e "\n${BLUE}[2/4] 明文密钥扫描...${NC}"
if bash scripts/check-secret-scan.sh --all; then
  echo -e "${GREEN}  ✅ 未发现明文密钥${NC}"
else
  echo -e "${RED}  ❌ 发现疑似明文密钥${NC}"; RESULT=1
fi

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
  if go test ./... -race -count=1 2>&1 | tail -20; then echo -e "${GREEN}  ✅ 测试通过${NC}"; else echo -e "${RED}  ❌ 测试失败${NC}"; RESULT=1; fi
  if [ "$WITH_LINT" = true ] && command -v golangci-lint >/dev/null 2>&1; then
    golangci-lint run ./... 2>&1 | tail -20 || { echo -e "${RED}  ❌ lint 未通过${NC}"; RESULT=1; }
  fi
  if [ "$WITH_COVERAGE" = true ]; then
    go test ./... -coverprofile=coverage.out >/dev/null 2>&1 || true
    [ -f coverage.out ] && echo "  覆盖率：$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}')（新增代码目标 ≥70%）"
  fi
  cd "$REPO_ROOT"
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
