#!/bin/bash
# =============================================================================
# 密钥扫描器【自测】—— 用真实探针文件验证扫描器本身没坏。
#
# 为什么需要给门禁写测试：
#   门禁失效是【静默】的。它照样打印 [OK]，只是什么都没拦住。
#   2026-07-30 实测到两个这样的失效：
#     ① 键名模式漏了 PW 缩写 → ROOT_PW= / DB_PW= 带真密码完全隐形
#     ② 基线文件的注释行被当固定字符串模式 → 第 3 行的 "#" 匹配任何含 # 的行
#        → 值里带 # 的密钥全部被当"已豁免"丢掉（# 是密码里极常见的字符）
#   两个都不会报错，只会让人以为"扫过了、没问题"。
#
# 用法：bash scripts/dev/verify-secret-scan.sh
# =============================================================================
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCAN="$REPO_ROOT/scripts/check-secret-scan.sh"
PROBE_DIR="$REPO_ROOT/server/configs"
PROBE="$PROBE_DIR/__secret_scan_probe.yaml"
PASS=0; FAIL=0

cleanup() { rm -f "$PROBE"; }
trap cleanup EXIT

# expect_caught <说明> <探针内容>
expect_caught() {
  printf '%s\n' "$2" > "$PROBE"
  if bash "$SCAN" --all >/dev/null 2>&1; then
    echo "  [X]  应被拦却放过了：$1"
    echo "       探针内容：$2"
    FAIL=$((FAIL+1))
  else
    echo "  [OK] 拦住了：$1"
    PASS=$((PASS+1))
  fi
  rm -f "$PROBE"
}

# expect_allowed <说明> <探针内容>
expect_allowed() {
  printf '%s\n' "$2" > "$PROBE"
  if bash "$SCAN" --all >/dev/null 2>&1; then
    echo "  [OK] 放行：$1"
    PASS=$((PASS+1))
  else
    echo "  [X]  应放行却拦了（误报会让门禁常红→被忽略）：$1"
    echo "       探针内容：$2"
    FAIL=$((FAIL+1))
  fi
  rm -f "$PROBE"
}

[ -f "$SCAN" ] || { echo "[X] 找不到 $SCAN"; exit 1; }
mkdir -p "$PROBE_DIR"

echo "▶ 前置：当前仓库必须是干净的（否则后续断言不可信）"
if bash "$SCAN" --all >/dev/null 2>&1; then
  echo "  [OK] 基线干净"
  PASS=$((PASS+1))
else
  echo "  [X] 仓库当前就有命中，先修掉再跑本自测"
  exit 1
fi

echo ""
echo "▶ 必须拦住的（反例）"
expect_caught "含 # 的密码（曾因基线注释行而完全隐形）" 'ROOT_PW="Xk9#mQ2vLp8s"'
expect_caught "PW 缩写变量名（曾因键名模式遗漏而隐形）"  'DB_PW=Zt7wRn4Bq1eK'
expect_caught "标准 password 键"                        'password: Aa1bb22cc33dd'
expect_caught "api_key"                                 'api_key = "sk-live-9f8e7d6c5b4a"'
expect_caught "token"                                   'access_token: ghp_1a2b3c4d5e6f7g8h'
expect_caught "含 * 的密码"                             'MYSQL_PWD=Pa**word123x'

echo ""
echo "▶ 必须放行的（正例，防止误报把门禁逼成常红）"
expect_allowed "环境变量占位"     'DB_PW=${TM_DB_PW}'
expect_allowed "your_/_here 占位" 'ROOT_PW="your_password_here"'
expect_allowed "fake- 前缀"       'ROOT_PW="fake-local-verify-only"'
expect_allowed "从环境读取"       'pw := os.Getenv("TM_DB_PW")'
expect_allowed "配置对象取值"     'password = config.DB.Password'
expect_allowed "表达式赋值"       '_token = (body.get("data") or {}).get("token")'

echo ""
echo "═══════════════════════════════"
echo "  通过 $PASS 项 / 失败 $FAIL 项"
echo "═══════════════════════════════"
[ "$FAIL" -eq 0 ] || exit 1
