# 题材查询 · 设计

- **版本**：v2（2026-07-30），按 [ADR-0003](../../adr/0003-个股客观事实层可做.md) 重写

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

### `GET /api/v1/theme/categories`
返回完整分类树。鉴权：否
```
Category { id int64, name string, level int, parentId int64, children Category[] }
```

### `GET /api/v1/theme/search?kw=&page=&size=`
出参 `{ list: ThemeBrief[], total, hasMore }`。鉴权：否
```
ThemeBrief { id int64, name string, categoryPath string }
```

### `GET /api/v1/theme/{id}`
鉴权：**是**（付费内容服务端判定）
```
ThemeDetail {
  id           int64
  name         string
  categoryPath string
  summary      string      // 一句话通俗解读（客观）
  updatedAt    int64
  dataSource   string
  quoteDelayMin int         // 行情延时分钟数，固定 15
  quoteAt      int64        // 行情数据时点（不是刷新时间）
  quoteIsMock  bool         // true 时前端必须显著标注「示例数据」
  locked       bool         // 无权限时 true，且下列字段为 null
  trialLeft    int
  chainNodes   ChainNode[] | null
  events       ThemeEvent[] | null
}

ChainNode {
  id int64, name string, description string, order int
  changePct    float64 | null   // 环节涨跌幅；无数据为 null（前端显示「—」）
  caliber      string           // 聚合口径说明，如「已收录公司等权平均」
  stocks       StockItem[]
}

StockItem {
  code      string            // 如 "688502"
  name      string
  market    string            // SH / SZ / BJ
  changePct float64 | null    // 延时行情；无数据为 null
  hasEvidence bool            // 恒为 true —— 无依据的映射在服务端已过滤
}

ThemeEvent { id int64, title string, publishAt int64, source string, sourceUrl string }
```

### `GET /api/v1/theme/{themeId}/stock/{code}/evidence`
归属依据浮层数据。鉴权：**是**（与详情页同权限）
```
Evidence {
  stockCode    string
  stockName    string
  chainNodeName string
  sourceType   string   // 公告 / 年报 / 招股书 / 官方目录
  sourceExcerpt string  // 原文摘录（必填，非空）
  sourceUrl    string   // 原文链接（必填，非空）
  collectedAt  int64    // 采集时点
}
```

### 🔴 契约层面的合规约束

1. **不得存在**个股详情类接口（`/stock/{code}` 返回基本面、财务、K线、技术指标）。
2. `StockItem` **不得**包含任何评价字段（`score` / `relevance` / `isLeader` /
   `purity` / `recommend` 等）。
3. `StockItem` **不得**包含 `highlight` / `featured` / `top` 类字段——
   高亮是纯前端的用户选中态。
4. 服务端在组装 `chainNodes[].stocks` 时**必须过滤掉证据字段不完整的映射**。
5. 排序参数只接受白名单：`changePct` / `code`；传其他值返回 400。

> 以上 1–3 由 `scripts/check-architecture.sh` 与联调脚本检查。

### 错误码
| 码 | 含义 |
|---|------|
| 40100 | 未登录 |
| 40301 | 未订阅且试吃已用完 |
| 40401 | 题材不存在 |
| 40402 | 依据不存在（该映射无证据，理论上不应出现） |
| 40001 | 排序字段非法 |

## 4. 数据模型

```sql
-- migrations/20260730_create_theme_tables.sql

CREATE TABLE theme_category (         -- 分类（一/二/三级）
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(64) NOT NULL,
  level TINYINT NOT NULL COMMENT '1|2|3',
  parent_id BIGINT NOT NULL DEFAULT 0,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
  KEY idx_parent (parent_id, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE theme (                  -- 题材
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(128) NOT NULL,
  category_id BIGINT NOT NULL COMMENT '三级分类 id',
  summary TEXT NOT NULL COMMENT '一句话通俗解读（客观）',
  data_source VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
  UNIQUE KEY uk_name (name),
  KEY idx_category (category_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE theme_chain_node (       -- 产业链环节
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  theme_id BIGINT NOT NULL,
  name VARCHAR(64) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
  KEY idx_theme (theme_id, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE stock (                  -- 个股基础信息（只存标识，不存基本面）
  code VARCHAR(16) PRIMARY KEY COMMENT '如 688502',
  name VARCHAR(64) NOT NULL,
  market VARCHAR(8) NOT NULL COMMENT 'SH|SZ|BJ',
  delisted TINYINT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT '🔴 一期禁止在此表扩展基本面/财务字段（红线，非排期）';

CREATE TABLE theme_stock_mapping (    -- 🧭 合规命门：题材↔个股映射，必须带依据
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  theme_id BIGINT NOT NULL,
  chain_node_id BIGINT NOT NULL,
  stock_code VARCHAR(16) NOT NULL,
  -- ↓ 证据字段：四项全部 NOT NULL 且不允许空串（DB 层 + 服务层双重约束）
  source_type    VARCHAR(32)  NOT NULL COMMENT '公告|年报|招股书|官方目录',
  source_excerpt VARCHAR(1024) NOT NULL COMMENT '原文摘录',
  source_url     VARCHAR(512) NOT NULL COMMENT '原文链接',
  collected_at   DATETIME     NOT NULL COMMENT '采集时点',
  -- ↓ 审核留痕
  reviewer    VARCHAR(64) NOT NULL DEFAULT '',
  reviewed_at DATETIME NULL,
  sort_order  INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
  UNIQUE KEY uk_node_stock (chain_node_id, stock_code),
  KEY idx_theme (theme_id),
  CONSTRAINT chk_evidence CHECK (
    source_type <> '' AND source_excerpt <> '' AND source_url <> ''
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT '🔴 无依据禁止入库——依据是本项目排除「品种选择」认定的核心抓手';

CREATE TABLE theme_event (            -- 题材关联事件
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  theme_id BIGINT NOT NULL,
  title VARCHAR(255) NOT NULL,
  source VARCHAR(64) NOT NULL,
  source_url VARCHAR(512) NOT NULL DEFAULT '',
  publish_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
  KEY idx_theme_time (theme_id, publish_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

> 🔴 **`stock` 表禁止扩展基本面字段**。"先建着以后再说"正是 v1 想防的事——
> 只是当时防错了对象：该防的不是"有没有个股表"，而是"有没有滑向投资分析"。

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
