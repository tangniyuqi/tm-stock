# 角色协作流水线

> 谁在什么阶段做什么。完整流程见 [../../WORKFLOW.md](../../WORKFLOW.md)。

## 角色与阶段对应

```
① 需求提出 👤 人
      ↓
② 澄清·验收标准 ── pm-agent ──→ docs/specs/{需求}/requirements.md
      ↓
③ 设计 ── pm-agent（页面）+ arch-agent（API契约/数据模型）──→ design.md
      ↓                                    （重大决策 → docs/adr/）
④ 测试设计 ── qa-agent ──→ 用例清单（回写 requirements.md）
      ↓
⑤ 开发（并行）── back-agent（Go）│ web-agent（uni-app x）
      ↓
⑥ 代码审查 ── review-agent ──→ 对照 spec + 合规红线
      ↓
⑦ 质量门禁 ── 机器 ──→ bash scripts/harness-checks.sh（+ CI）
      ↓
⑧ 验收 ── acceptance-agent ──→ 逐条核对验收标准
      ↓
⑨ 上线确认 👤 人
      ↓
⑩ 复盘 ── 全体 ──→ 坑点写 known-pitfalls.md，能机检的做成门禁
```

## 角色清单

| 角色 | 文件 | 一句话职责 |
|-----|------|----------|
| 产品/需求 | `pm-agent.md` | 把模糊诉求澄清成可测的验收标准 |
| 架构 | `arch-agent.md` | API 契约、数据模型、重大决策 ADR |
| 后端 | `back-agent.md` | Go 分层实现 |
| 前端 | `web-agent.md` | uni-app x 页面实现 |
| 测试 | `qa-agent.md` | 从验收标准产出用例并执行 |
| 审查 | `review-agent.md` | 对照 spec 与合规红线挑问题 |
| 验收 | `acceptance-agent.md` | 逐条核对，出通过/驳回 |

## 通用约束（所有角色）

- 先读 `compliance-redline.md`（最高优先级）与 `methodology.md`（方法论底座）
- 中文交付、三问自检、不写总结文档
- **门禁绿灯才算完成**
