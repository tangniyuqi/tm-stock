# 运营后台 · 设计

## 1. 方案概述

基于 gin-vue-admin v3.0.0 搭建，位于 `server/admin/`，
与 C 端服务 `server/api/` **分服务、共库、共 model**（[ADR-0004](../../adr/0004-后端采用gin-vue-admin.md)）。

**设计主线只有一句**：让"无依据的数据"在任何一条路径上都进不来。

## 2. 三重防线的具体落点

```
         录入表单                保存接口              数据库
            │                      │                    │
  四项必填 ─┤        二次校验 ─────┤     NOT NULL ──────┤
  按钮禁用  │        空串亦拒      │     CHECK <> ''    │
            │                      │                    │
        绕不过前端            绕不过接口           兜住批量导入
                                                  与手工 SQL
```

| 路径 | 被哪一层拦住 |
|-----|------------|
| 运营在表单里漏填 | 表单层（按钮禁用） |
| 用 Postman 直接调接口 | 服务层（400） |
| CSV 批量导入绕过表单校验 | 服务层 + DB |
| 有人手工执行 SQL | **DB CHECK 约束** |

> 只做前端校验等于没做。三层缺一，就存在一条无依据数据进库的路径。

## 3. 禁用词校验的落点（关键设计）

**问题**：代码层的 `check-compliance-words.sh` 只扫仓库文件，
扫不到运营录入的**运行时内容**——而那些内容会原样显示给用户。

**方案**：把词表做成**单一事实源**，两端共用。

```
scripts/compliance-forbidden-words.txt   ← 唯一词表
        │
        ├── check-compliance-words.sh    （构建期：扫代码与 mock）
        └── server/pkg/compliance/       （运行期：扫运营录入内容）
                 └── 启动时加载 + 文件变更热加载
```

**受检字段**：`theme.summary`、`theme_chain_node.description`、
`theme_daily_item.title`、`theme_stock_mapping.source_excerpt`。

**校验时机**：**保存时**（不是展示时）。展示时才拦等于脏数据已入库。

**错误返回**：必须指出**命中词**与**字段名**，否则运营只会反复试错。
```json
{ "code": 40010, "msg": "内容含合规禁用词",
  "data": { "field": "summary", "hits": ["龙头"] } }
```

> ⚠️ 词表只能有一份。若后台维护副本，两边必然漂移——
> 到时候 CI 是绿的，用户看到的却是违规文案。

## 4. 审核流状态机

```
        录入            提交审核         审核通过
  ┌────┐      ┌──────┐          ┌──────┐        ┌────────┐
  │ 无 │─────>│ 草稿 │─────────>│ 待审 │───────>│ 已通过 │──> C 端可见
  └────┘      └──────┘          └──────┘        └────────┘
                  ^                  │
                  │      驳回(填原因) │
                  └──────────────────┘
```

- **C 端查询恒定附加 `status = 'APPROVED'`**——写在 repository 层的基础查询里，
  不靠每个 service 记得加（漏一处就泄漏）
- 录入人 ≠ 审核人：接口层校验 `creator != current_user`
- 状态流转全部落审计日志

## 5. 数据模型（在 theme-query 基础上扩展）

```sql
-- migrations/20260730_admin_console.sql

-- 1) 给映射表加审核状态（theme-query 已建表，此处 ALTER）
ALTER TABLE theme_stock_mapping
  ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'DRAFT'
      COMMENT 'DRAFT|PENDING|APPROVED|REJECTED',
  ADD COLUMN creator VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN reject_reason VARCHAR(255) NOT NULL DEFAULT '',
  ADD KEY idx_status (status);

-- 2) 审计日志（所有写操作）
CREATE TABLE admin_audit_log (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  operator     VARCHAR(64)  NOT NULL COMMENT '操作人',
  action       VARCHAR(32)  NOT NULL COMMENT 'CREATE|UPDATE|DELETE|APPROVE|REJECT',
  entity       VARCHAR(64)  NOT NULL COMMENT '对象类型',
  entity_id    VARCHAR(64)  NOT NULL,
  before_json  TEXT         NULL COMMENT '变更前',
  after_json   TEXT         NULL COMMENT '变更后',
  ip           VARCHAR(64)  NOT NULL DEFAULT '',
  created_at   DATETIME     NOT NULL,
  KEY idx_entity (entity, entity_id),
  KEY idx_operator_time (operator, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT '🔴 监管问询时的举证材料，不是可选审计功能';

-- 3) 证券基础表数据来源（只存标识，见 theme-query design §4）
--    stock 表已在 theme-query 定义，此处不重复
```

> `stock` 表**仍然禁止**扩展基本面字段——后台也不例外。
> 后台能存的，迟早会有接口把它带到 C 端。

## 6. 批量导入设计

- 格式：CSV，UTF-8 with BOM（Excel 兼容）
- 列：`题材名, 环节名, 股票代码, 来源类型, 原文摘录, 原文链接, 采集时点`
- **逐行校验，不合格行不入库**，返回：
  ```json
  { "total": 100, "success": 97,
    "failed": [ {"row": 12, "reason": "原文摘录为空"},
                {"row": 45, "reason": "股票代码 999999 未收录"},
                {"row": 78, "reason": "摘录含禁用词：龙头"} ] }
  ```
- **不做"部分成功即回滚全部"**——运营宁可 97 条先进去，剩 3 条改了再补
- 导入产生的记录同样进 `DRAFT` 状态，仍需审核

## 7. 关键取舍

| 取舍 | 决定 | 理由 |
|-----|------|------|
| 校验层数 | **三层（表单/接口/DB）** | 少一层就多一条绕过路径 |
| 词表位置 | **单一文件，两端共用** | 副本必然漂移，导致 CI 绿而线上违规 |
| 校验时机 | **保存时**，非展示时 | 展示时拦 = 脏数据已入库 |
| C 端过滤 | 写在 **repository 基础查询**里 | 靠每个 service 记得加，漏一处就泄漏 |
| 批量导入失败 | **部分成功**，报行号 | 全量回滚会让运营反复重试整个文件 |
| 评价类字段 | **后台也不给** | 不提供打分工具，就不会有打分泄漏 |
| 审计日志 | 存变更前后完整 JSON | 只存"改过"没有举证价值 |

## 8. 风险与回退

- **风险 1（最大）**：**依据采集是重人工**。表单越严，录入越慢。
  缓解：来源类型下拉 + 代码自动带名称 + CSV 批量；但**不能为了快而放宽四项必填**。
- **风险 2**：BSL 授权未购 → 后台只能内网开发使用，**不得公网可访问**。
- **风险 3**：运营为过校验而胡填摘录（如复制一段无关原文）→
  靠**审核环节 + 抽检**，机器拦不住"填了但填得敷衍"。
  建议主管每周抽检 N 条，抽检结果计入运营考核。
- **风险 4**：词表更新后存量数据可能已含新增禁用词 →
  提供**存量扫描工具**（离线跑一遍全库，输出待整改清单），不做自动改写。
