# 题材动态（按日）· 设计

## 1. 方案概述

按日期查询"当天有哪些题材动态"。

**核心设计点：一切按日快照，不实时重算。**
用户回看 7 月 20 日，看到的必须是**7 月 20 日当时**的题材涨跌幅，
而不是拿今天的行情重新算一遍。因此需要一张**每日快照表**，
由定时任务在收盘后落库。

> 这一点如果一开始不做，将来补不回来——**历史数据不会自己长出来**。

## 2. 页面规格（`pages/index/index` 第一个 tab）

```
┌─────────────────────────────┐
│  题材动态  题材轮动  题材库    │  ← tab
├─────────────────────────────┤
│ 上一日   📅 2026-07-25  下一日│  ← 日期切换
├─────────────────────────────┤
│ 排序：题材当日涨跌幅 ⇅         │  ← 必须明示排序依据
├─────────────────────────────┤
│ 1  半导体设备国产化率创历史新高 │
│    [半导体设备]  +3.25%       │
│                  07-25 09:30 │
│ ─────────────────────────── │
│ 2  多部委发文推动新材料产业…    │
│    [新材料]      +2.18%       │
│                  07-25 10:15 │
├─────────────────────────────┤
│ 免责声明（固定）               │
└─────────────────────────────┘
```

**日历浮层**（对应设计稿第四张图）：月份切换、"本月"快捷、已选日期高亮、
**未来日期灰显不可点**。

### 视觉合规约束
- 序号 1/2/3 的彩色强调**可以保留**（题材层排序，风险远低于个股排行）
- 但序号旁**不得**出现"最值得关注""今日最强"等文案
- 排序依据**必须常驻可见**，不能只在设置里

## 3. API 契约

> 统一 `{code,msg,data}`；时间字段**毫秒时间戳 int64**。

### `GET /api/v1/theme-daily/list?date=YYYY-MM-DD&sort=&page=&size=`

- `date` 省略时返回**最近一个有数据的交易日**
- `sort` 白名单：`changePct`（默认，降序）/ `publishAt`；传其他值返回 400
- 鉴权：**否**（免费引流层）

```
DailyResp {
  date        string        // 实际返回的日期（可能与请求不同，如省略时）
  isTradingDay bool
  emptyReason string        // "" | "NON_TRADING_DAY" | "NO_DATA" | "BEFORE_RANGE"
  dataFrom    string        // 数据起始日，前端用于禁用更早日期
  quoteDelayMin int         // 15
  snapshotAt  int64         // 该日快照落库时点
  quoteIsMock bool
  sort        string        // 当前排序依据，前端必须展示
  list        DailyItem[]
  total       int
  hasMore     bool
}

DailyItem {
  id         int64
  title      string         // 事件客观标题
  themeId    int64
  themeName  string
  changePct  float64 | null // 该题材【当日快照】涨跌幅；无数据为 null → 前端显示「—」
  caliber    string         // 涨跌幅聚合口径说明
  publishAt  int64
  source     string
  sourceUrl  string
}
```

### 🔴 契约层面的合规约束

1. `DailyItem` **不得**包含任何个股字段（`stockCode` / `stocks` / `leaders` 等）。
2. `DailyItem` **不得**包含 `score` / `importance` / `heat`（我方打分）类字段。
3. `sort` 只接受客观字段白名单；**不得**支持"按重要性"。
4. `changePct` 必须来自**快照表**，不得由服务端用当日行情实时计算。

### 错误码
| 码 | 含义 |
|---|------|
| 40001 | 排序字段非法 / 日期格式非法 |
| 40002 | 日期早于数据起始日 |

## 4. 数据模型

```sql
-- migrations/20260730_create_theme_daily_tables.sql

CREATE TABLE theme_daily_item (        -- 题材动态条目（事件 ↔ 题材，按日）
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  biz_date DATE NOT NULL COMMENT '归属交易日',
  theme_id BIGINT NOT NULL,
  title VARCHAR(255) NOT NULL COMMENT '事件客观标题',
  source VARCHAR(64) NOT NULL,
  source_url VARCHAR(512) NOT NULL DEFAULT '',
  publish_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
  KEY idx_date (biz_date, publish_at DESC),
  KEY idx_theme (theme_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE theme_daily_quote (       -- 🔑 题材当日涨跌幅【快照】
  biz_date DATE NOT NULL,
  theme_id BIGINT NOT NULL,
  change_pct DECIMAL(8,4) NULL COMMENT 'NULL 表示当日无数据，前端显示「—」',
  caliber VARCHAR(128) NOT NULL COMMENT '聚合口径，如「已收录成分股等权平均」',
  is_mock TINYINT NOT NULL DEFAULT 0 COMMENT '模拟数据标记',
  snapshot_at DATETIME NOT NULL COMMENT '快照落库时点',
  PRIMARY KEY (biz_date, theme_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT '🔑 每日收盘后落库。历史不可重算——回看必须看到当时的值';

CREATE TABLE trading_calendar (        -- 交易日历
  biz_date DATE PRIMARY KEY,
  is_trading_day TINYINT NOT NULL,
  remark VARCHAR(64) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 5. 每日快照任务

| 项 | 设计 |
|---|------|
| 触发 | 每交易日收盘后（如 15:30）定时任务 |
| 动作 | 按题材聚合当日成分股涨跌幅 → 写 `theme_daily_quote` |
| 幂等 | 主键 `(biz_date, theme_id)`，重跑覆盖同日数据 |
| 失败 | 写 `change_pct = NULL`（不写 0），告警；**不允许用前一日值补** |
| 非交易日 | 不落库；查询侧靠 `trading_calendar` 判定 |
| 授权未落地 | `quote.enabled=false` 时 `change_pct=NULL`、`is_mock` 按配置置位 |

> ⚠️ **任务上线越早越好**。数据起始日 = 任务首次成功运行日，
> 在此之前的历史**永远补不回来**（除非另行采购历史行情）。

## 6. 关键取舍

| 取舍 | 决定 | 理由 |
|-----|------|------|
| 历史涨跌幅 | **快照表**，不实时重算 | 回看必须看到当时的值，否则是"用今天改写历史" |
| 无数据 | `NULL` → 显示「—」 | 写 0 会被读成平盘；用前一日值补是假数据 |
| 排序 | 客观字段白名单 + UI 明示 | 我方打分排序 = 变相推荐 |
| 付费墙 | **列表免费**，点题材进详情才收费 | 首屏是引流层；⚠️ 待董事长确认 |
| 非交易日 | 空态，不回填 | 回填 = 假数据 |

## 7. 风险与回退

- **风险 1（最紧迫）**：**快照任务不上线，历史就是空的**。
  建议在题材库尚未做完时就先把快照任务跑起来，先攒数据。
- **风险 2**：事件 → 题材的归属需人工，量大时跟不上 →
  一期先只覆盖少量热门题材，宁可条目少。
- **风险 3**：排序依据的灰区定性 → 做成配置，律师若收紧可切换为纯时间排序。
- **风险 4**：首屏免费 / 付费的边界尚未拍板 → 付费判定放服务端，改口径不用改前端。
