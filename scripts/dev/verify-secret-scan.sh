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

# 登记关系只在临时 Git 仓库中制造，不修改当前仓库的索引或工作树登记。
verify_worktree_scope() {
  local root="$TREE/scope root" other probe rc desc
  mkdir -p "$root"
  git -C "$root" init -q || return 1
  git -C "$root" config core.autocrlf false || return 1
  # Git Bash 的 /tmp 是挂载别名；用 Git 自己返回的绝对路径登记工作树。
  root="$(git -C "$root" rev-parse --show-toplevel)" || return 1
  other="$root/.claude/worktrees/registered copy [1]"
  git -C "$root" -c core.hooksPath=/dev/null -c commit.gpgsign=false \
    -c user.name=scan-probe -c user.email=scan-probe@invalid \
    commit -q --allow-empty -m '密钥扫描临时夹具' || return 1
  git -C "$root" -c core.hooksPath=/dev/null worktree add -q --detach "$other" HEAD || return 1
  [ -d "$other" ] || { echo '  [X] 临时工作树登记路径与落盘路径不一致'; return 1; }
  printf '%s\n' 'ROOT_PW="Xk9#mQ2vLp8s"' > "$other/registered.yaml" || return 1

  scope_expect() {
    local scope="$1" want="$2" desc="$3" rc
    TM_REPO_ROOT="$scope" bash "$SCAN" --all >/dev/null 2>&1; rc=$?
    if [ "$rc" -eq "$want" ]; then
      echo "  [OK] $desc（退出码 $rc）"; PASS=$((PASS+1))
    else
      echo "  [X] $desc：期望退出码 $want，实际 $rc"; FAIL=$((FAIL+1))
    fi
  }

  scope_expect "$root" 0 '只跳过已登记的其它工作树（路径含空格与方括号）'
  scope_expect "$(cd "$root" && pwd -P)" 0 'Git 与 Bash 两种绝对路径表示均只跳过登记副本'
  # 后续反例不能借用登记副本里的命中凑出退出码 1。
  rm -f "$other/registered.yaml"
  for desc in '当前根目录' '被 Git 忽略的文件' '未登记的 worktrees 目录' '同名但未登记的目录' '登记路径的前缀相似目录' '独立嵌套 Git 仓库' '只有伪造 .git 文件的目录'; do
    case "$desc" in
      '当前根目录') probe="$root/current.yaml" ;;
      '被 Git 忽略的文件')
        printf '%s\n' 'ignored/' > "$root/.gitignore"
        probe="$root/ignored/private.yaml" ;;
      '未登记的 worktrees 目录') probe="$root/.claude/worktrees/not-registered/private.yaml" ;;
      '同名但未登记的目录') probe="$root/elsewhere/registered copy [1]/private.yaml" ;;
      '登记路径的前缀相似目录') probe="$other-extra/private.yaml" ;;
      '独立嵌套 Git 仓库')
        mkdir -p "$root/independent"
        git -C "$root/independent" init -q || return 1
        probe="$root/independent/private.yaml" ;;
      *)
        mkdir -p "$root/fake-checkout"
        printf '%s\n' "gitdir: $root/.git/worktrees/missing" > "$root/fake-checkout/.git"
        probe="$root/fake-checkout/private.yaml" ;;
    esac
    mkdir -p "$(dirname "$probe")"
    printf '%s\n' 'ROOT_PW="Xk9#mQ2vLp8s"' > "$probe"
    scope_expect "$root" 1 "$desc 仍被扫描"
    rm -f "$probe"
  done
  printf '%s\n' 'ROOT_PW="Xk9#mQ2vLp8s"' > "$other/registered.yaml" || return 1
  scope_expect "$other" 1 '当前扫描根自身虽已登记也不能跳过'
  scope_expect "$root/.claude" 1 'TM_REPO_ROOT 指向非仓库根时不继承父仓库的跳过范围'
  git -C "$other" add registered.yaml || return 1
  (cd "$other" && bash "$SCAN" >/dev/null 2>&1); rc=$?
  if [ "$rc" -eq 1 ]; then
    echo '  [OK] 已登记工作树自己的暂存区仍受检'; PASS=$((PASS+1))
  else
    echo "  [X] 已登记工作树暂存区：期望退出码 1，实际 $rc"; FAIL=$((FAIL+1))
  fi
  # 其余探针会全量扫描 TREE；不能把本组的字面量留给后续正例。
  rm -f "$other/registered.yaml"
}

[ -f "$SCAN" ] || { echo "[X] 找不到 $SCAN"; exit 1; }
mkdir -p "$PROBE_DIR" "$TREE/scripts"
# 基线文件要一并带上：其中的注释行曾经让含 # 的密钥隐形，这条回归必须在真实的基线文件上验证
cp "$REPO_ROOT/scripts/secret-scan-baseline.txt" "$TREE/scripts/" 2>/dev/null || true

echo '▶ 已登记工作树的扫描边界（隔离临时仓库）'
verify_worktree_scope || exit 1
if [ "${1:-}" = '--worktree-scope-only' ]; then
  echo "  通过 $PASS 项 / 失败 $FAIL 项（仅工作树范围，非全套自测）"
  [ "$FAIL" -eq 0 ]; exit $?
fi

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
expect_caught "6 位密码（前端最小长度）"                'ROOT_PW=123456'
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
expect_allowed "UTS/TS 基础类型标注" 'type Credentials = { token: string }'

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
