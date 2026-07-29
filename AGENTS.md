# AI / Agent 说明（tm-stock 题材宝典）

本仓库是**独立工程**，与其他项目完全隔离，不共享规范目录。

## 规则入口（单一事实源）

统一入口 **[CLAUDE.md](CLAUDE.md)**；规范文件在 `.claude/agents/`。
Cursor / Codex / 其他 Agent 同样以 CLAUDE.md 为入口，**不要另起一套规则**。

## 🔴 最高优先级

**`.claude/agents/compliance-redline.md`** — 合规红线。本项目做的是证券信息服务，
违反红线是**公司级法律风险**（无牌证券投资咨询），比任何技术缺陷都严重。

## 必读

- `.claude/agents/methodology.md` — 工作方法论铁律
- `.claude/agents/domain-glossary.md` — 业务概念口径
- `.kiro/skills/tmui4x-dev/SKILL.md` — 前端组件技能库（已有资产，优先复用）
- [WORKFLOW.md](WORKFLOW.md) — 研发流程

## 人类维护者

协作约定变更时**只改 `.claude/agents/` 下对应文件**，并同步 CLAUDE.md 路由表。
禁止把本项目规则写到仓库外的全局位置。
