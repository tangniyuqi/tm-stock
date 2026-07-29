#!/bin/bash
# =============================================================================
# Stop hook：回合结束强制注入「交付自检清单」（Harness L1 · 第三重保证）
#
# ★ 为什么需要这一层：
#   规范写进 CLAUDE.md / .cursor/rules 只保证「被读到」，不保证「被执行」。
#   本 hook 在每个回合结束时把自检项强制推到模型面前——不依赖它"记得读"。
#
# ★ 踩坑固化：
#   - exit 0 时 stderr 会被丢弃 → 必须走 stdout 的 systemMessage JSON
#   - 非阻塞：只提醒不拦截，遵循批量里程碑，不打断连续开发
# =============================================================================

set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT" || exit 0

# 只在「本回合确实动了代码」时提醒，避免纯对话回合刷屏
CHANGED="$(git status --porcelain -uall 2>/dev/null | grep -cE '\.(uvue|uts|go|json|md|sh)$' || true)"
[ "${CHANGED:-0}" -eq 0 ] && exit 0

CODE="$(git status --porcelain -uall 2>/dev/null | grep -cE '\.(uvue|uts|go)$' || true)"

MSG="📋 交付自检（tm-stock 强制纪律，勿跳过）："
MSG="$MSG\n① 三问自检：最没把握的是什么？最大的遗漏是什么？会破坏/波及什么？"
MSG="$MSG\n② 零信任：关键逻辑真跑过吗（正例+反例）？还是只看了语法通过？"
MSG="$MSG\n③ 数据真实：有没有虚构数字/占位假值？拿不到就留空。"
if [ "${CODE:-0}" -gt 0 ]; then
  MSG="$MSG\n④ 合规红线：新增文案有没有碰个股/预测/排序/禁用词？"
  MSG="$MSG\n⑤ 质量门禁：本回合有 $CODE 个代码文件改动 → 里程碑前跑 bash scripts/harness-checks.sh"
fi

printf '{"systemMessage": "%s"}\n' "$MSG"
exit 0
