# ADR-0007：复用现有 `addon_quant_*` 数据表，废弃自建的 theme/stock 表

- **状态**：已采纳
- **日期**：2026-07-30
- **决策人**：董事长
- **取代**：`server/migrations/20260730_create_theme_tables.sql` 中的
  `theme` / `theme_category` / `theme_chain_node` / `stock` / `theme_event` 五张表
- **相关**：[ADR-0003](0003-个股客观事实层可做.md)（依据可溯源）、
  [ADR-0006](0006-G1最小验证路径.md)（一期砍分类下钻）

## 背景

董事长已有一套在跑的系统（库 `go_noooya_com`，MySQL 9.7.0），其中：

| 表 | 内容 | 规模 |
|---|------|------|
| `addon_quant_base_stock` | 股票基础数据，Tushare `stock_basic` 结构 | **5497 条**（全 A 股） |
| `addon_quant_theme` | 题材表，带 `level` / `parent_id` 层级 | 2 条（光刻机、MLCC） |

而 tm-stock 此前自建了 `theme` / `stock` / `theme_stock_mapping` 等表，
**功能完全重复**。两套并存的代价是：题材内容会在两边各自录入，之后**极难合并**——
而题材库正是本项目的护城河，数据分裂等于护城河分裂。

## 决定

**tm-stock 与现有系统共库，以 `addon_quant_*` 为准。**

1. **废弃**自建的 `theme_category` / `theme` / `theme_chain_node` / `stock` / `theme_event`
2. **保留并转正**唯一真正缺失的表：`addon_quant_theme_stock`（题材↔股票多对多）
3. Go 的 model / repository / dto 全部对齐现有字段风格

### 层级语义（本决策一并确定）

`addon_quant_theme` 的 `level` / `parent_id` 用于表达**题材 → 产业链环节**：

```
level 1  题材（光刻机、MLCC）        parent_id = 0
level 2  产业链环节（光源、物镜）     parent_id = 所属题材 id
```

**不用它表达分类导航**（半导体与电子 › 光学光电子）——
因为 [ADR-0006](0006-G1最小验证路径.md) 已把三级分类下钻砍出一期，
这棵树正好可以专用于环节，无需第二棵树。

> 若二期要恢复分类导航，再单独建分类树，不要往这棵树上叠。
> `domain-glossary.md` 说的「两个正交结构」指的就是这件事：
> 分类导航与产业链是两回事，塞进同一棵树迟早出乱子。

## 由此产生的约束

### C 端查询必须带三重过滤

```sql
WHERE ts.deleted_at   IS NULL   -- ① 软删除（现有表全都是软删除）
  AND ts.audit_status = 2       -- ② 仅已审核通过
  AND ts.status       = 1       -- ③ 启用中
```

三个漏任意一个都是**静默泄漏**：不报错，只是把不该露的数据露出去。
→ 必须写死在 repository 的基础查询里，不做成调用方传参。

### 字段命名的两处冲突（已处理）

| 冲突 | 处理 |
|-----|------|
| 现有表的 `status` 是**启用/停用**，与我方的**审核状态**语义不同 | 审核字段命名为 `audit_status`，不复用 `status` |
| 现有表主键是 `id`（bigint unsigned），业务键是 `ts_code` | 关联表按 `stock_id` 关联，另冗余 `ts_code` 供对账；**权威以 stock_id 为准** |

## 已知隐患（属现有系统，待其维护者决定）

1. `addon_quant_base_stock` **无 `ts_code` 唯一键** → 重复导入会产生重复股票，
   关联表将无法确定指向哪一条。
2. `addon_quant_theme` 的 `uk_source_code(source, code)` 在 `code` 为 NULL 时
   **形同虚设**（MySQL 唯一键允许多个 NULL，现有两条数据 `code` 均为 NULL）
   → 题材可重名。

两条的查重 SQL 与建议 ALTER 已附在
`server/migrations/20260730_addon_quant_theme_stock.sql` 文末，**未擅自执行**。

## 后果

**正面**
- 题材数据只有一份，不会分裂
- 直接拿到 5497 只股票的完整基础数据，省掉一整块导入工作
- 现有表已带 `ngram` 全文索引 `(symbol, name, cnspell)`，
  正好支撑 `theme-query` 里"支持拼音首字母搜索"这条验收标准

**负面（正视）**
- tm-stock **不再独立**：schema 由现有系统主导，改字段要协调
- 现有表的两个隐患成了我们的隐患
- Go 层要跟随现有的软删除与操作人字段约定，比自建时啰嗦
- 上述已知隐患的修复不在我方控制内

## 落地清单

- [x] 关联表转正到 `server/migrations/`
- [x] 删除自建五张表的迁移
- [x] model / repository / dto / service 对齐新 schema
- [x] SQL 守护测试改为断言三重过滤
- [x] `verify-migrations.sh` 改为在测试库内先建上游两表再验关联表
- [x] `docs/specs/theme-query/design.md` §3 API 契约与 §4 数据模型同步（v3）

## 尚未处理

- [ ] `docs/specs/theme-query/requirements.md` 与 `tasks.md` 仍按旧表描述
- [ ] `docs/specs/admin-console/*` 的录入界面按旧表字段设计
- [ ] 事件时间线：上游无事件表，`ThemeDetail.events` 当前恒为空数组
