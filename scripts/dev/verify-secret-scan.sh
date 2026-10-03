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
TREE="$(mktemp -d)"          # 探针只写进临时目录树，不碰真实仓库；扫描器用 TM_REPO_ROOT 指向它
PROBE_DIR="$TREE/server/configs"
PROBE_BASE="$PROBE_DIR/__secret_scan_probe"
PASS=0; FAIL=0

cleanup() { rm -rf "$TREE"; }
trap cleanup EXIT

# expect_caught_ext <扩展名> <说明> <探针内容>
expect_caught_ext() {
  local probe="$PROBE_BASE.$1"
  printf '%s\n' "$3" > "$probe"
  if TM_REPO_ROOT="$TREE" bash "$SCAN" --all >/dev/null 2>&1; then
    echo "  [X]  应被拦却放过了：$2"
    echo "       探针内容（.$1）：$3"
    FAIL=$((FAIL+1))
  else
    echo "  [OK] 拦住了：$2"
    PASS=$((PASS+1))
  fi
  rm -f "$probe"
}

# expect_allowed_ext <扩展名> <说明> <探针内容>
expect_allowed_ext() {
  local probe="$PROBE_BASE.$1"
  printf '%s\n' "$3" > "$probe"
  if TM_REPO_ROOT="$TREE" bash "$SCAN" --all >/dev/null 2>&1; then
    echo "  [OK] 放行：$2"
    PASS=$((PASS+1))
  else
    echo "  [X]  应放行却拦了（误报会让门禁常红→被忽略）：$2"
    echo "       探针内容（.$1）：$3"
    FAIL=$((FAIL+1))
  fi
  rm -f "$probe"
}

# expect_caught_file <探针文件名> <说明> <探针内容>：文件名由调用方给，用来验证"路径里的字样不能让文件免检"
expect_caught_file() {
  local probe="$PROBE_DIR/$1"
  printf '%s\n' "$3" > "$probe"
  if TM_REPO_ROOT="$TREE" bash "$SCAN" --all >/dev/null 2>&1; then
    echo "  [X]  应被拦却放过了：$2"
    echo "       探针文件：$1  内容：$3"
    FAIL=$((FAIL+1))
  else
    echo "  [OK] 拦住了：$2"
    PASS=$((PASS+1))
  fi
  rm -f "$probe"
}

# staged_expect <期望退出码 0|1> <说明> <文件名> <内容>：在一次性的临时仓库里走真正的暂存区路径（pre-commit 用的那条）
staged_expect() {
  local want="$1" desc="$2" name="$3" content="$4" tmp rc
  tmp="$(mktemp -d)"
  ( cd "$tmp" && git init -q . && git config core.autocrlf false && printf '%s\n' "$content" > "$name" && git add "$name" )
  ( cd "$tmp" && bash "$SCAN" >/dev/null 2>&1 ); rc=$?
  rm -rf "$tmp"
  if [ "$rc" -eq "$want" ]; then
    echo "  [OK] ${desc}（退出码 $rc）"
    PASS=$((PASS+1))
  else
    echo "  [X]  ${desc}：期望退出码 $want，实际 $rc"
    echo "       文件：$name  内容：$content"
    FAIL=$((FAIL+1))
  fi
}

# 原有用例都是 yaml 探针
expect_caught()  { expect_caught_ext  yaml "$1" "$2"; }
expect_allowed() { expect_allowed_ext yaml "$1" "$2"; }

[ -f "$SCAN" ] || { echo "[X] 找不到 $SCAN"; exit 1; }
mkdir -p "$PROBE_DIR" "$TREE/scripts"
# 基线文件要一并带上：其中的注释行曾经让含 # 的密钥隐形，这条回归必须在真实的基线文件上验证
cp "$REPO_ROOT/scripts/secret-scan-baseline.txt" "$TREE/scripts/" 2>/dev/null || true

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
# —— 以下是"代码语言里裸标识符放行"这条规则的边界：它不能放走真密钥 ——
expect_caught_ext sh "shell 里的裸词就是字面量（不属于代码语言放行范围）"   'export DB_password=hunter2hunter2'
expect_caught_ext sh "shell 里看着像标识符的裸词同样要拦"                   'API_token=abcdefghijkl'
expect_caught_ext js "代码文件里带引号的字面量照拦"                         'cfg.password = "Xk9mQ2vLp8s1";'
expect_caught_ext js "同一行既有变量引用又有真字面量：整行照拦"             'x.password = password; x.token = "ghp_1a2b3c4d5e6f7g8h";'
expect_caught_file __secret_scan_probe_config.example.js "路径里有 config. 与 example 字样，文件不能因此免检" 'x.password = "Xk9mQ2vLp8s1";'
expect_caught_ext js "数字开头的值不是标识符，照拦"                         'token = 9f8e7d6c5b4a3921;'

echo ""
echo "▶ 必须放行的（正例，防止误报把门禁逼成常红）"
expect_allowed "环境变量占位"     'DB_PW=${TM_DB_PW}'
expect_allowed "your_/_here 占位" 'ROOT_PW="your_password_here"'
expect_allowed "fake- 前缀"       'ROOT_PW="fake-local-verify-only"'
expect_allowed "从环境读取"       'pw := os.Getenv("TM_DB_PW")'
expect_allowed "配置对象取值"     'password = config.DB.Password'
expect_allowed "表达式赋值"       '_token = (body.get("data") or {}).get("token")'
# —— 实测 13 处误报的几种形态：变量引用与界面属性，不是字面量密钥 ——
expect_allowed_ext uts "对象简写：键与变量同名"                'a.password: password,'
expect_allowed_ext js  "属性赋变量"                            'payload.password = password;'
expect_allowed_ext py  "Python 变量赋值"                       'DB_password = password_from_vault'
expect_allowed_ext vue "Vue 密码框的 show-password 属性"           '<el-input v-model="f.password" type="password" show-password :clearable="true" />'

echo ""
echo "▶ 暂存区模式（pre-commit 实际走的路径，与 --all 的判定口径必须一致）"
staged_expect 1 "暂存区：字面量密钥被拦"                       src.js    'x.password = "Xk9mQ2vLp8s1";'
staged_expect 1 "暂存区：路径有 config. 与 example 也被拦"      config.example.js 'x.password = "Xk9mQ2vLp8s1";'
staged_expect 1 "暂存区：shell 里的裸词被拦"                    run.sh    'export DB_password=hunter2hunter2'
staged_expect 0 "暂存区：代码里的变量引用放行"                  ok.js     'payload.password = password;'
staged_expect 0 "暂存区：占位符放行"                            ok.yaml   'password: ${TM_DB_password}'

echo ""
echo "═══════════════════════════════"
echo "  通过 $PASS 项 / 失败 $FAIL 项"
echo "═══════════════════════════════"
[ "$FAIL" -eq 0 ] || exit 1
