---
name: requirement-pipeline
description: 需求流水线总指挥。当提出一个新业务需求（说"/需求 xxx"或描述想要的功能）时使用，按 tm-stock 流程编排：合规预检 → 澄清验收标准 → 设计 → 用例 → 开发 → 审查 → 门禁 → 验收。
---

# 需求流水线（tm-stock）

> 完整流程见 [WORKFLOW.md](../../../WORKFLOW.md)；角色分工见 [PIPELINE.md](../../agents/PIPELINE.md)。

## 🔴 第 0 步：合规预检（本项目特有，先做这个）

在澄清需求之前，先判断该需求是否触碰合规红线：

- 需要展示**个股/成分股/个股涨跌幅**吗？→ **一期不做**，先按方案 C 收敛
- 需要**排序/榜单/评级**吗？→ 涉绩效排序即红线
- 需要**预测/择时/推荐**吗？→ 红线
- AI 输出会落到个股层吗？→ 红线

**碰红线就先收敛或否掉，不要"先做了再说"**。依据 `.claude/agents/compliance-redline.md`。

## 第 1 步：澄清（pm-agent）

- 模棱两可处**列出所有可能性让人选**，不要自行假设
- 挂到 `docs/GOALS.md` 当季目标；挂不上要提示"是否该现在做"
- 判定需求等级 A/B/C/D，决定哪些环节可跳过
- 产出 `docs/specs/{需求}/requirements.md`（模板在 `docs/specs/_TEMPLATE/`）

**验收标准必须可测**：❌"体验流畅" → ✅"列表首屏 p95 < 1.5s"；
必须覆盖正常流 + 边界 + 异常 + 权限。

## 第 2 步：设计（pm-agent + arch-agent）

- 页面规格：结构、字段、状态（加载/空/错误/无权限）
- **API 契约**（前后端并行的前提）：路径/入参/出参/错误码，时间字段用毫秒时间戳
- 重大决策 → `docs/adr/NNNN-xxx.md`
- 产出 `design.md`

## 第 3 步：用例（qa-agent）

每条验收标准 ≥1 条用例，覆盖边界/异常/权限，回写 requirements.md。

## 第 4 步：开发（back-agent / web-agent 并行）

契约已定 → 前后端可并行。L1 hooks 即时纠错。

## 第 5 步：审查 → 门禁 → 验收

- review-agent 对照 spec + **合规红线**
- `bash scripts/harness-checks.sh` 全绿（**自述不算数**）
- acceptance-agent 逐条核对验收标准

## 第 6 步：复盘

坑点写进 `.claude/agents/known-pitfalls.md`；**能机检的做成门禁**。
