#!/bin/bash
# =============================================================================
# PostToolUse hook：文件编辑后的即时反馈（Harness L1）
# 参数：$1 = fast（密钥+合规词） | lint（Go 格式/vet）
#
# ★ 已固化的踩坑经验（勿删）：
#   1. 【不依赖 jq】很多环境没装 jq，用 sed 提取 file_path。
#   2. 【Windows 反斜杠】file_path 可能是 D:\a\b.uvue，必须转正斜杠再匹配。
#   3. 【exit 0 吞 stderr】要反馈给 AI 必须 exit 2。
#   4. 【worktree 各一份】settings.json 主仓库与 worktree 各存一份，改了要同步。
# =============================================================================
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="${1:-fast}"

PAYLOAD="$(cat)"
FILE_PATH="$(printf '%s' "$PAYLOAD" | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
[ -z "$FILE_PATH" ] && exit 0
FILE_PATH="$(printf '%s' "$FILE_PATH" | tr '\\\\' '/' | sed 's/\\\\//g')"
[ -f "$FILE_PATH" ] || exit 0

# ── ① 明文凭据扫描 ──
HITS=""
if [ -f "$HOME/.tmstock-secret-patterns" ]; then
  HITS="$(grep -nF -f "$HOME/.tmstock-secret-patterns" "$FILE_PATH" 2>/dev/null | head -5 || true)"
fi
if [ -z "$HITS" ]; then
  KEY_PAT='(pass(word|wd)?|pwd|secret|token|access_?key|api_?key|private_?key|credential)'
  VAL_PAT='[[:space:]]*[=:][[:space:]]*[^[:alnum:][:space:]]?[A-Za-z0-9#!@%^&*_.+-]{8,}'
  ALLOW='your_|_here|example|placeholder|xxx|fake-|test_|dummy|getenv|process\.env|viper\.|config\.'
  HITS="$(grep -nEi "${KEY_PAT}${VAL_PAT}" "$FILE_PATH" 2>/dev/null \
        | grep -v '\$' | grep -vEi "$ALLOW" | head -5 || true)"
fi
if [ -n "$HITS" ]; then
  echo "❌ 检测到疑似明文凭据：" >&2
  echo "$HITS" >&2
  echo "✅ 修复：真实密钥放 ~/.tmstock-credentials，文件里只写占位符（如 \$TM_DB_PASSWORD）" >&2
  exit 2
fi

# ── ② 合规禁用词（本项目生命线）──
if [ -x "$REPO_ROOT/scripts/check-compliance-words.sh" ] || [ -f "$REPO_ROOT/scripts/check-compliance-words.sh" ]; then
  OUT="$(bash "$REPO_ROOT/scripts/check-compliance-words.sh" "$FILE_PATH" 2>&1)" || {
    echo "$OUT" >&2
    exit 2
  }
fi

[ "$MODE" = "fast" ] && exit 0

# ── ③ Go：gofmt + vet ──
case "$FILE_PATH" in
  *.go)
    command -v gofmt >/dev/null 2>&1 || exit 0
    UNFMT="$(gofmt -l "$FILE_PATH" 2>/dev/null || true)"
    if [ -n "$UNFMT" ]; then
      echo "❌ gofmt 未格式化：$FILE_PATH" >&2
      echo "✅ 修复：gofmt -w \"$FILE_PATH\"" >&2
      exit 2
    fi
    exit 0
    ;;
esac

# ── ④ uni-app x：无 CLI lint，仅做基础提醒 ──
# （uni-app x 走 HBuilderX 构建，没有 eslint CLI；质量靠规范 + 合规词门禁 + 人工在 IDE 校验）
exit 0
