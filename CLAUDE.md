# 题材宝典 tm-stock · 开发规范（AI 入口）

> 本文件是 AI 与人的**共同入口**。任何任务开始前，按类型读对应规范。
> 规范在 `.claude/agents/`，本文件只做路由；改规范只改那里。

## 🔴 第一优先级：合规红线（违反 = 公司级法律风险）

**开工前必读：[`.claude/agents/compliance-redline.md`](.claude/agents/compliance-redline.md)**

一句话记住：**不荐股 · 不预测 · AI 只分析到「题材/行业/事件」层 · 一期不含个股 · 用户自主判断**。

> 监管对"荐股软件"采**功能实质测试**：只要具备"对具体证券分析评价 / 预测走势 / 选择建议 / 买卖时机"任一功能，
> 即属须持牌业务；**免责声明不能豁免**。证券投顾牌照 2016 年起停发。

## 🌏 语言偏好（强制）

**所有交流、注释、提交信息、文档必须用中文（简体）。** 见 `.claude/agents/a-language-preference.md`。

## 🧭 方法论底座（所有任务通用）

**[`.claude/agents/methodology.md`](.claude/agents/methodology.md)** — 基础优先 / 实地考察 / 数据真实 /
对抗审查 / 零信任验收 / 三问自检 / 自主闭环 / 模棱两可先问。

## 📋 按任务类型读规范

| 任务类型 | 先读 |
|---------|------|
| **前端（uni-app x）** | `.claude/agents/uniappx-standards.md` + **已有技能库** `.kiro/skills/tmui4x-dev/SKILL.md` |
| **后端（Go）** | `.claude/agents/backend-standards-go.md` + `.claude/agents/back-agent.md` |
| 架构设计 | `.claude/agents/arch-agent.md` |
| 需求分析 | `.claude/agents/pm-agent.md` |
| 代码审查 | `.claude/agents/review-agent.md`（**审查必查合规红线**） |
| 业务概念 | `.claude/agents/domain-glossary.md`（题材/事件/产业链口径） |
| 写文档 | `.claude/agents/no-summary-docs.md`（禁止任务总结文档） |

> **tmui4x 组件开发**：仓库已有成熟技能库 `.kiro/skills/tmui4x-dev/`（组件目录、CSS 系统、页面开发指南、
> 各组件文档）。**优先查它，不要重复造轮子、不要另写一套组件规范。**

## 🏗️ 项目结构

```
tm-stock/
├── App.uvue / main.uts / pages.json    # uni-app x 前端入口（仓库根即前端）
├── pages/                              # 页面：index · news(快讯) · segment(题材) · member · express
├── uni_modules/                        # 组件依赖（含 tmx-ui）
├── static/ · hybrid/                   # 静态资源
├── .kiro/skills/tmui4x-dev/            # ★ 已有：tmui4x 开发技能库（复用）
├── .claude/                            # AI 协作：settings / hooks / agents
├── server/                             # Go 后端（Gin）
├── scripts/                            # 质量门禁与 git hooks
├── docs/                               # GOALS / adr / specs
└── tasks/                              # 半自主任务队列
```

## 🛡️ 四层护栏

| 层 | 触发 | 载体 |
|----|-----|------|
| L1 即时反馈 | AI 每次改文件 | `.claude/hooks/`（自动：密钥扫描 + **合规禁用词** + Go 格式） |
| L2 提交门 | git commit / push | `scripts/git-hooks/`（先跑 `bash scripts/git-hooks/install.sh`） |
| L3 质量门禁 | 里程碑完成 | `bash scripts/harness-checks.sh` |
| L4 半自主 | 无人值守 | `tasks/` 队列 |

**铁律：Agent 自述不算数，门禁绿灯才算数。**

## 🔑 凭据约定

真实密钥放本地 `~/.tmstock-credentials`（仓库外，600），使用前 `source`。
仓库内**只允许占位符**（`$TM_XXX`）；明文密钥会被 L1/L2 双层拦截。

## ⚠️ 交付纪律

1. 指令模棱两可 → **先列可能性让人选**再动手。
2. 完成后简洁汇报（1-2 句 + 改动点 + 文件路径），**不写总结文档**。
3. 经验教训 → 沉淀成 `.claude/agents/` 里的规则，能机检的做成门禁。
