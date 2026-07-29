# 自主任务队列（L4）

无人值守/批处理执行的**外部化任务队列**。

## 为什么队列要外部化

**不要让 AI"自己记住任务"**——session 会断、context 会满。任务状态必须落在**仓库文件**里，
执行器每次"短 session 启动 → 取一个任务 → 做完 → 更新状态 → 退出"，靠文件实现 durable state。

## 任务文件格式

一个任务一个文件：`tasks/<优先级数字>-<kebab-id>.md`，头部 YAML frontmatter 机器可读，正文给 AI 看。
模板见 [_TEMPLATE.md](_TEMPLATE.md)。

## 状态流转

```
pending ──执行器拾取──> in_progress ──复验绿 + 提 PR──> done
                            │
                            └──复验红 / 超 max_retry──> failed（等人介入）
依赖未完成 ──> blocked
```

## 关键字段

| 字段 | 说明 |
|-----|------|
| `status` | pending / in_progress / done / failed / blocked |
| `priority` | 数字越小越先执行 |
| `max_retry` | 超过即 failed，不再重试（防死循环烧钱） |
| `max_runtime_sec` | 单次运行墙钟上限 |
| `branch` | 执行器在此 feature 分支上工作 |
| `verify_level` | **独立复验级别**：build / test / harness / web / custom / none |
| `verify_command` | verify_level=custom 时必填，从仓库根目录执行 |
| `deps` | 依赖的任务 id，未完成则 blocked |

## 🔴 红线

1. 执行器**只推 feature 分支 + 开 PR，绝不自动 merge 到主干**。
2. **人 review PR 才是闭环终点**，不是"AI 说做完了"。
3. 任务粒度控制在 **30 分钟 ~ 2 小时**；"重构整个系统"这类必须先拆。
4. **独立复验**：AI 自述完成不算数，`verify_level` 跑绿才算。
