#!/bin/bash
# =============================================================================
# 任务卡校验 —— 写入队列前先跑，避免执行器拿到残缺任务空转
# 用法：bash scripts/autonomous/validate-task.sh tasks/0001-xxx.md
# =============================================================================
set -uo pipefail
F="${1:-}"
[ -z "$F" ] && { echo "用法: bash scripts/autonomous/validate-task.sh <任务文件>"; exit 2; }
[ -f "$F" ] || { echo "[X] 文件不存在: $F"; exit 1; }

RESULT=0
need() {
  grep -qE "^$1:" "$F" || { echo "[X] 缺少必填字段: $1"; RESULT=1; }
}
# ★ 必须剥离行内 # 注释与尾随空格，否则 "pending   # pending|done|..." 会被整段当成值（踩过）
val() { sed -n "s/^$1:[[:space:]]*//p" "$F" | head -1 | sed 's/[[:space:]]*#.*$//' | tr -d "\r" | sed 's/[[:space:]]*$//'; }

# frontmatter 必须存在
head -1 "$F" | grep -q "^---$" || { echo "[X] 缺少 YAML frontmatter（首行应为 ---）"; exit 1; }

for k in id title status priority max_retry branch task_type verify_level; do need "$k"; done

# 枚举值校验
ST="$(val status)"
case "$ST" in pending|in_progress|done|failed|blocked) ;; *) echo "[X] status 非法: $ST"; RESULT=1 ;; esac

VL="$(val verify_level)"
case "$VL" in build|test|harness|web|custom|none) ;; *) echo "[X] verify_level 非法: $VL"; RESULT=1 ;; esac
[ "$VL" = "custom" ] && [ -z "$(val verify_command)" ] && { echo "[X] verify_level=custom 必须填 verify_command"; RESULT=1; }

TT="$(val task_type)"
case "$TT" in backend|web|docs|script|mixed) ;; *) echo "[X] task_type 非法: $TT"; RESULT=1 ;; esac

# 正文必须有可验证的验收标准
grep -q "验收标准" "$F" || { echo "[X] 正文缺少「验收标准」段落"; RESULT=1; }
grep -qE "^- \[ \]" "$F" || { echo "[X] 验收标准里没有可勾选项（- [ ] 形式）"; RESULT=1; }
grep -q "边界" "$F" || echo "[!] 建议补充「边界」段落（明确不要动什么）"

[ $RESULT -eq 0 ] && echo "[OK] 任务卡校验通过: $F"
exit $RESULT
