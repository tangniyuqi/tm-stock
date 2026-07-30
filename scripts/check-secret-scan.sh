#!/bin/bash
# =============================================================================
# 明文密钥扫描（L2/L3 共用）
# 用法：
#   bash scripts/check-secret-scan.sh          # 只扫暂存区新增行（pre-commit 用）
#   bash scripts/check-secret-scan.sh --all    # 全仓扫描（门禁用）
#
# ★ 设计原则（踩坑固化）：
#   1. 基线冻结：存量命中写进 secret-scan-baseline.txt，只拦【新增】——否则一上来全红。
#   2. 纯 bash 实现，不依赖 jq/python——避免环境缺依赖、以及 Windows 下中文/GBK 编码崩溃。
#   3. 占位符放行：$VAR / ${VAR} / your_xxx / xxx_here / example / placeholder / fake- / test_
# =============================================================================
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
BASELINE="$REPO_ROOT/scripts/secret-scan-baseline.txt"
MODE="${1:-staged}"

# 敏感键 + 明文值（≥8 位）
# ★ 不写引号字符类：grep -E 不支持 \x27，改用「值前允许 1 个非字母数字字符」涵盖任意引号
# ★ pw 必须单列：键名模式只写 pass/pwd 时，ROOT_PW= / DB_PW= 这类【常见缩写】
#   完全隐形——实测三条样本里只有 MYSQL_PWD 被命中，ROOT_PW 与 DB_PW 全漏。
#   PW 是极常见的密码变量名缩写，漏掉它等于给密钥留了一扇正门。
PATTERN='(pass(word|wd)?|pwd|pw|secret|token|access_?key|api_?key|private_?key|credential)[[:space:]]*[=:][[:space:]]*[^[:alnum:][:space:]]?[A-Za-z0-9#!@%^&*_.+-]{8,}'
# 放行：占位符、示例值、从环境/配置读取的写法
# ★ 关键一条：值是【函数调用或属性访问】（= foo( / = foo.bar）不是明文密钥——
#   否则第三方库里满地的 `const token = parser.fetch()` 会把门禁淹没成常红（实测踩过）。
#   另外：值以括号/方括号开头（= (expr) / = [..] / = {..}）同样是表达式而非字面量密钥。
ALLOW='\$|your_|_here|example|placeholder|xxxx|fake-|test_|dummy|<.*>|\*\*\*|getenv|process\.env|viper\.|config\.|[=:][[:space:]]*[A-Za-z_][A-Za-z0-9_]*[.(]|[=:][[:space:]]*[({[]'

# 自测脚本必须包含探针密钥字符串，否则测不出扫描器坏没坏。
# 与 P13（规范文档自身含禁用词）同理：定义/检验规则的文件必须被排除。
# 排除项只列具体文件名，不整目录放行——scripts/dev/ 下别的脚本仍受检。
SELF_TEST='scripts/dev/verify-secret-scan.sh'

if [ "$MODE" = "--all" ]; then
  # ★ 排除第三方与文档目录：uni_modules/hybrid/static 是三方库，.kiro/docs 是规范文档（含示例代码）
  CANDIDATES="$(grep -rnEi "$PATTERN" "$REPO_ROOT" \
      --include='*.go' --include='*.ts' --include='*.uts' --include='*.js' --include='*.vue' --include='*.uvue' \
      --include='*.yaml' --include='*.yml' --include='*.json' --include='*.env*' \
      --include='*.sh' --include='*.sql' --include='*.py' \
      --exclude-dir=node_modules --exclude-dir=.git --exclude-dir=vendor --exclude-dir=dist \
      --exclude-dir=uni_modules --exclude-dir=hybrid --exclude-dir=static \
      --exclude-dir=unpackage --exclude-dir=.kiro --exclude-dir=docs \
      --exclude="$(basename "$SELF_TEST")" \
      2>/dev/null || true)"
else
  # 只看暂存区【新增行】。
  #
  # ★ 用 awk 跟踪 +++ b/<path> 头，让每一行都带上所属文件名。
  #   原实现只做 `grep '^+' | sed 's/^+//'`，丢掉了文件归属，后果有两个：
  #     ① 无法排除任何文件 → 合法的测试夹具会永久卡住 pre-commit
  #     ② 报错时看不出命中在哪个文件，只能人工翻 diff
  #   这也顺带让 staged 与 --all 两种模式的【排除口径】终于一致（对照 P14）。
  CANDIDATES="$(git diff --cached -U0 2>/dev/null | awk '
      /^\+\+\+ b\// { file = substr($0, 7); next }
      /^\+/ && !/^\+\+\+/ { print file ":" substr($0, 2) }
    ' | grep -vF "$SELF_TEST:" | grep -Ei "$PATTERN" || true)"
fi

# 去掉占位符
HITS="$(printf '%s' "$CANDIDATES" | grep -vEi "$ALLOW" || true)"

# 应用基线（存量豁免）
#
# ★★ 严重坑（2026-07-30 实测）：绝不能直接 grep -vFf "$BASELINE"。
#    基线文件里的【注释行】会被当成固定字符串模式。本文件第 3 行是个只有 "#"
#    的注释行 → 模式 "#" 匹配任何含 # 的行 → grep -v 把它们全部当"已豁免"丢掉。
#    后果：任何值里带 # 的明文密钥【完全隐形】，而 # 是密码里极常见的字符。
#    同理，基线里出现空行会匹配所有行，等于整个扫描被静默关闭。
#    → 必须先剥掉注释与空行，再当模式用。
if [ -n "$HITS" ] && [ -f "$BASELINE" ]; then
  BL_CLEAN="$(grep -vE '^[[:space:]]*(#|$)' "$BASELINE" || true)"
  if [ -n "$BL_CLEAN" ]; then
    HITS="$(printf '%s' "$HITS" | grep -vF "$BL_CLEAN" || true)"
  fi
fi

if [ -n "$HITS" ]; then
  echo "[X] 检测到疑似明文密钥（新增）："
  printf '%s\n' "$HITS" | head -10
  echo ""
  echo "[修复] 真实密钥放 ~/.tmstock-credentials，代码里改用占位符：\$TM_XXX"
  echo "[说明] 确属误报时，把该行片段追加到 scripts/secret-scan-baseline.txt"
  echo "[红线] 禁止用 --no-verify 绕过；禁止为了过门禁而扩大基线"
  exit 1
fi

echo "[OK] 未发现新增明文密钥"
exit 0
