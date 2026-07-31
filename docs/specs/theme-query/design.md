# 题材查询 · 设计

- **版本**：v3（2026-07-30）：v2 按 [ADR-0003](../../adr/0003-个股客观事实层可做.md) 重写；
  v3 按 [ADR-0007](../../adr/0007-复用现有addon_quant数据表.md) 改为复用现有 addon_quant_* 表

## 1. 方案概述

题材库为**人工维护 + 结构化存储**（护城河，不是自动抓取能替代的）。
前端分类下钻 → 详情页折叠环节 → 展开看成分股 → 点个股看**归属依据**。
付费墙在**服务端**判定（前端遮罩只是表现，不是安全边界）。

**行情**与**题材结构**分离：结构数据变化慢（可长缓存），行情 15 分钟延时（短缓存）。

## 2. 页面规格

### 2.1 题材库页（`pages/segment/index`）
- 左侧：一级分类竖排（数据驱动，不硬编码）
- 右侧：二级分组 + 其下三级题材 chip
- 顶部：题材搜索入口
- 状态：加载骨架 / 空态 / 错误可重试

### 2.2 题材详情页（`pages/segment/detail`）

对应设计稿第三张图：

```
┌─────────────────────────────┐
│ ← 题材                       │
├─────────────────────────────┤
│ OCS 光交换                   │  ← 顶层（免费）
│ 半导体与电子 › 光学光电子      │
│ 一句话通俗解读…               │
│ 更新于 07-30 · 来源 XXX       │
├─────────────────────────────┤
│ 行情延时15分钟 · 数据时点 14:45│  ← 强制明示
├─────────────────────────────┤
│ ▼ 光源      +5.5% ⓘ          │  ← 中层（付费）环节折叠
│    ┌──────┐┌──────┐┌──────┐ │
│    │茂莱光学││波长光电││福晶科技│ │  ← 成分股卡片
│    │ +2.1% ││ -0.8% ││ +1.3% │ │
│    └──────┘└──────┘└──────┘ │
│ ▶ 物镜      +4.5% ⓘ          │
├─────────────────────────────┤
│ 关联事件时间线                 │  ← 底层（付费）
├─────────────────────────────┤
│ 免责声明（固定）               │
└─────────────────────────────┘
```

- **环节涨跌幅旁的 ⓘ** → 展开聚合口径说明（如"该环节已收录公司等权平均"）
- **点击成分股卡片** → 弹出「归属依据」浮层，**不跳转**
- **未订阅**：中层及以下高斯模糊遮罩 + "订阅后查看" + "今日试吃剩余 N 次"

### 2.3 归属依据浮层（合规核心组件）

```
┌── 茂莱光学 为什么在「光源」环节 ──┐
│ 来源类型：2025 年年度报告          │
│ 原文摘录：                        │
│   "公司光学镜头及光源模组业务       │
│    收入占比 34.2%…"               │
│ 原文链接：[巨潮资讯网 →]           │
│ 采集时点：2026-06-12               │
└──────────────────────────────┘
```

- 四项**缺一不可**；缺任一项的映射**不入库、不返回**
- 浮层内**不得**出现任何评价性措辞

## 3. API 契约

> 统一 `{code,msg,data}`；时间字段**毫秒时间戳 int64**。
> **v3（2026-07-30）**：按 ADR-0007 对齐 `addon_quant_*` 表结构。
> 分类树接口已移除——ADR-0006 砍掉三级分类下钻，一期只做题材列表 + 搜索。

### `GET /api/v1/theme/search?kw=&limit=`
按题材名搜索（只搜 `level<=1` 的题材本身，不搜产业链环节）。鉴权：否
```
ThemeBrief { id int64, name string, description string }
```

### `GET /api/v1/theme/{id}`
鉴权：**是**（付费内容服务端判定）
```
ThemeDetail {
  id            int64
  name          string
  summary       string      // 题材说明（客观）
  updatedAt     int64
  quoteEnabled  bool        // 一期 false（ADR-0006 砍掉涨跌幅）
  quoteDelayMin int         // 启用时固定 15
  quoteAt       int64       // 行情【数据时点】，不是刷新时间
  quoteIsMock   bool        // true 时前端必须显著标注「示例数据」
  locked        bool        // 无权限时 true，且 chainNodes 为 null
  trialLeft     int
  chainNodes    ChainNode[] | null
  events        ThemeEvent[] | null   // 上游暂无事件表，当前恒为空
}

ChainNode {
  id int64, name string, description string
  changePct  float64 | null   // 环节涨跌幅；无数据为 null（前端显示「—」）
  caliber    string           // 聚合口径说明，如「已收录成分股等权平均」
  stocks     StockItem[]
}

StockItem {
  code        string          // 6 位 symbol，给用户看，如 "688502"
  tsCode      string          // 唯一标识，如 "688502.SH"，调依据/行情接口用
  name        string
  market      string          // 主板 / 创业板 / 科创板…
  changePct   float64 | null  // 延时行情；无数据为 null
  hasEvidence bool            // 恒为 true —— 无依据的映射在服务端已过滤
}
```

> 分组说明：`chainNodes` 的第一组可能是**题材自身**（当该题材直接挂了股票、
> 未拆环节时）。前端不要假设每组都是产业链环节。

### `GET /api/v1/theme/{themeId}/stock/{tsCode}/evidence`
归属依据浮层数据。鉴权：**是**（与详情页同权限，不能绕付费墙）
```
Evidence {
  tsCode        string
  stockCode     string   // 6 位 symbol
  stockName     string
  chainNodeName string
  sourceType    string   // 中文文案：公告/年报/招股书/官方产业目录/互动易问答
  sourceExcerpt string   // 原文摘录（必填，非空）
  sourceUrl     string   // 原文链接（必填，非空）
  collectedAt   int64    // 采集时点
}
```

> `sourceType` 返回**中文文案**而非枚举数字：这段字要直接显示给用户，
> 让前端各自维护一份「1=公告 2=年报」映射表，两边迟早不同步。

### 🔴 契约层面的合规约束

1. **不得存在**个股详情类接口（返回基本面、财务、K线、技术指标）。
2. `StockItem` **不得**包含评价字段（`score` / `isLeader` / `purity` /
   `recommendLevel` / `importance` 等）。
3. `StockItem` **不得**包含 `highlight` / `featured` / `top` 类字段——
   高亮是纯前端的用户选中态。
4. 服务端组装 `chainNodes[].stocks` 时**必须过滤**证据不全 / 未审核 / 已停用的映射。
5. 排序只用客观字段；服务端默认按 `tsCode` 升序。

> 1–3 由 `scripts/check-architecture.sh` 规则 7/8/9 机器守护；
> 4 由 `service/theme_test.go` 与 `repository/theme_sql_test.go` 双层断言守护。

### 错误码
| 码 | 含义 |
|---|------|
| 40100 | 未登录 |
| 40301 | 未订阅且试吃已用完（`ErrAccessDenied`） |
| 40401 | 题材不存在（`ErrThemeNotFound`） |
| 40402 | 依据不存在（`ErrEvidenceNotFound`；证据不全同样归此类） |

## 4. 数据模型

> **v3（2026-07-30）**：按 [ADR-0007](../../adr/0007-复用现有addon_quant数据表.md)，
> 与现有系统**共库**，自建的 `theme` / `theme_category` / `theme_chain_node` /
> `stock` / `theme_event` 五张表**已废弃删除**。

### 表归属

| 表 | 归属 | 内容 |
|---|------|------|
| `addon_quant_theme` | **现有系统**，本项目只读 | 题材树：`level=1` 题材，`level=2` 产业链环节 |
| `addon_quant_base_stock` | **现有系统**，本项目只读 | 5497 只 A 股，Tushare `stock_basic` 结构 |
| `addon_quant_theme_stock` | **本项目负责** | 题材↔股票归属映射（唯一真正缺失的表） |

建表脚本：`server/migrations/20260730_addon_quant_theme_stock.sql`
（上游两表的脚本不在本仓库；测试用的最小复刻在 `scripts/dev/verify-migrations.sh` 内联）

### 层级语义

```
level 1  题材（光刻机、MLCC）        parent_id = 0
level 2  产业链环节（光源、物镜）      parent_id = 所属题材 id
```

归属映射的 `theme_id` **可指向任意层级**：题材不分环节就挂 level 1，
分环节就挂 level 2。因此详情页要按「题材自身 + 其所有子节点」一并查询。

**不用这棵树表达分类导航**——ADR-0006 已把三级分类下钻砍出一期。

### 🔴 C 端查询必须带的过滤（写进 repository 基础查询，不做成传参）

```sql
WHERE ts.deleted_at    IS NULL   -- ① 软删除：现有表全是软删除
  AND ts.audit_status  = 2       -- ② 仅已审核通过
  AND ts.status        = 1       -- ③ 启用中
  AND ts.source_type   > 0       -- ④ 证据三项非空
  AND ts.source_excerpt <> ''
  AND ts.source_url     <> ''
-- JOIN 上游两表时同样要过滤各自的 deleted_at
```

任意一条漏掉都是**静默泄漏**：不报错，只是把不该露的数据露出去。
→ 已收敛为 `repository` 的 `mappingBaseWhere` / `upstreamJoin` 两个常量，
并由 `theme_sql_test.go` 断言守护（不需要数据库，CI 每次都跑）。

### 关键约束（已在 MySQL 8.0.45 实测）

| 约束 | 机制 | 实测结果 |
|-----|------|---------|
| 证据非空 | `CHECK chk_theme_stock_evidence` | 空串 → `ERROR 3819` |
| 证据非 NULL | `NOT NULL` | NULL → `ERROR 1048` |
| 同环节不重复挂同一股票 | `UNIQUE (theme_id, stock_id, alive)` | 重复 → `ERROR 1062` |
| 软删后可重新添加 | 生成列 `alive`（存活=1，删除=NULL） | ✅ 可重加 |
| 误插入不对外可见 | `audit_status DEFAULT 0`（草稿） | ✅ C 端查到 0 条 |

> ⚠️ 生成列**不能用非确定性函数**：初稿写 `IFNULL(UNIX_TIMESTAMP(deleted_at),0)`
> 直接建表失败（`ERROR 3763`）。改用 `IF(deleted_at IS NULL, 1, NULL)`。

### 现有表的两个已知隐患（属现有系统，待其维护者决定）

1. `addon_quant_base_stock` **无 `ts_code` 唯一键** → 重复导入产生重复股票
2. `addon_quant_theme` 的 `uk_source_code(source, code)` 在 `code` 为 NULL 时
   **形同虚设** → 题材可重名

查重 SQL 与建议 ALTER 见迁移脚本文末，**未擅自执行**。

## 5. 行情接入设计

- **来源**：授权的延时行情源（15 分钟），见 `docs/上线前置授权清单.md` §2
- **缓存**：Redis，key `quote:{code}`，TTL 60s；批量拉取，不按个股逐个请求
- **降级**：拉取失败时 `changePct = null`，前端显示"—"，**不显示 0、不显示旧值**
- **开关**：`quote.enabled=false` 时全站不展示涨跌幅（授权未落地时的默认状态）
- **模拟数据**：`quote.mock=true` 时接口置 `quoteIsMock=true`，前端**必须**显著标注

> 与「数据真实铁律」一致：**宁可留空，不显示假数据**。

## 6. 付费墙与试吃

- **服务端判定**：`/theme/{id}` 内部检查订阅 → 未订阅则查当日试吃计数
- 试吃按 **userId + GMT+8 自然日**记账，**服务端存储**
- 同一题材当日重复访问**只扣一次**
- 用尽返回 40301
- **依据接口同权限**——不能通过直接调 evidence 接口绕过付费墙

## 7. 关键取舍

| 取舍 | 决定 | 理由 |
|-----|------|------|
| 个股点击行为 | **弹依据浮层**，不跳详情页 | 一箭三雕：满足合规可溯源、给用户真价值、避开详情页红线 |
| 依据字段位置 | 建在**映射表**上，不是个股表上 | 依据是"为什么归到这个环节"，与环节绑定而非与公司绑定 |
| 无依据数据 | **服务端过滤，不返回** | 返回空依据等于承认"我们也不知道为什么" |
| 行情缺失 | 显示"—" | 显示 0 会被读成"平盘"，是假数据 |
| 默认排序 | 涨跌幅降序 + UI 明示 + 可切换 | 客观字段；⚠️ 仍属灰区，待律师确认 |
| 题材库来源 | 人工维护为主 | 护城河；自动抓取质量不可控且易蹭概念 |

## 8. 风险与回退

- **风险 1（最大）**：**依据采集是重人工**——每条映射都要找原文摘录。
  直接决定题材库能做多大多快。回退：先窄后宽，只精修少量热门题材。
- **风险 2**：行情授权未落地 → `quote.enabled=false` 整体关闭涨跌幅，页面仍可用
- **风险 3**：律师意见收紧 → 中层/依据浮层做成**可整体下线的开关**，
  退回"只到环节层"形态（即 v1 形态）而不是重写
- **风险 4**：成分股映射被质疑"蹭概念" → 依据浮层就是答辩材料；
  这也是为什么依据必须是**原文摘录**而不是"我们判断"
