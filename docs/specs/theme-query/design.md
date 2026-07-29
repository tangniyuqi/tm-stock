# 题材查询 · 设计

## 1. 方案概述

题材库为**人工维护 + 结构化存储**（这是产品护城河，不是自动抓取能替代的）。
前端分类下钻 → 详情三层展示；付费墙在**服务端**判定（前端遮罩只是表现，不能作为安全边界）。

## 2. 页面规格

### 2.1 题材库页（pages/segment）
- 左侧：一级分类竖排（约 10–12 类，数据驱动，不硬编码）
- 右侧：二级分组 + 其下三级题材 chip
- 顶部：题材搜索入口
- 状态：加载骨架 / 分类为空提示 / 错误可重试

### 2.2 题材详情页
- **顶层**（免费）：题材名（大字）+ 分类路径面包屑 + 一句话解读 + "更新于 X · 来源 Y"
- **中层**（付费）：产业链环节横向流程图（可点击环节看说明）+ 关联事件时间线（时间 + 客观标题 + 原文跳转）
- **底层**（付费）：历史涨跌统计（图表 + 口径说明）
- **未订阅**：中/底层高斯模糊遮罩 + "订阅后查看" + "今日试吃剩余 N 次" + 订阅按钮
- **全页底部**：固定免责声明

## 3. API 契约

> 统一 `{code,msg,data}`；时间字段**毫秒时间戳 int64**。

### `GET /api/v1/theme/categories`
返回完整分类树（一/二/三级）。出参：
```
Category { id int64, name string, level int, parentId int64, children Category[] }
```
鉴权：否

### `GET /api/v1/theme/search?kw=&page=&size=`
出参：`{ list: ThemeBrief[], total, hasMore }`
```
ThemeBrief { id int64, name string, categoryPath string }
```
鉴权：否

### `GET /api/v1/theme/{id}`
出参：
```
ThemeDetail {
  id           int64
  name         string
  categoryPath string
  summary      string     // 一句话通俗解读（客观）
  updatedAt    int64      // 毫秒时间戳
  dataSource   string     // 数据来源说明
  // ↓ 以下字段仅在有权限时返回；无权限时为 null 并置 locked=true
  locked       bool
  chainNodes   ChainNode[] | null
  events       ThemeEvent[] | null
  history      HistoryStat | null
  trialLeft    int        // 今日剩余试吃次数
}
ChainNode  { id int64, name string, description string, order int }
ThemeEvent { id int64, title string, publishAt int64, source string, sourceUrl string }
HistoryStat{ caliber string, points: [{ date int64, changePct float64 }] | null }
```
鉴权：**是**（付费内容服务端判定）

> 🔴 **契约层面的合规约束**：`ThemeDetail` 及其子结构
> **不得包含任何个股字段**（stockCode / stocks / components 等）。
> 架构守护与联调脚本会检查这一点。

### 错误码
| 码 | 含义 |
|---|------|
| 40100 | 未登录 |
| 40301 | 未订阅且试吃已用完 |
| 40401 | 题材不存在 |

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
  data_source VARCHAR(255) NOT NULL DEFAULT '' COMMENT '数据来源说明',
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
  UNIQUE KEY uk_name (name),
  KEY idx_category (category_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE theme_chain_node (       -- 产业链环节（一期漏斗终点，无个股）
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  theme_id BIGINT NOT NULL,
  name VARCHAR(64) NOT NULL,
  description VARCHAR(512) NOT NULL DEFAULT '',
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
  KEY idx_theme (theme_id, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE theme_event (            -- 题材关联事件（只到题材层，不落个股）
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

> 🔴 **一期不建成分股表**（ADR-0002）。schema 层面就不留个股字段，
> 避免"先建着以后再说"演变成事实上线。

## 5. 付费墙与试吃

- **服务端判定**：`/theme/{id}` 内部检查订阅状态 → 未订阅则查当日试吃计数
- 试吃计数按 **userId + GMT+8 自然日**记账，**服务端存储**（前端计数可被绕过）
- 同一题材当日重复访问**只扣一次**（记录已试吃的 themeId 集合）
- 用尽返回 40301，前端展示遮罩与订阅引导

## 6. 关键取舍

| 取舍 | 决定 | 理由 |
|-----|------|------|
| 题材数据来源 | 人工维护为主 | 这是护城河；自动抓取质量不可控且易蹭概念 |
| 付费判定位置 | 服务端 | 前端遮罩不是安全边界 |
| 历史数据 | 一期只做题材层、标口径 | 个股层涉红线；口径不清等于假数据 |

## 7. 风险与回退

- **风险 1（最大）**：题材库内容生产跟不上 → 一期先窄后宽，只精修少量热门大类
- **风险 2**：涨跌数据来源与聚合口径未定 → 未定前**底层历史区整体不上线**，不用假数据凑
- **风险 3**：合规边界变化（律师意见）→ 中层/底层做成可整体下线的开关
