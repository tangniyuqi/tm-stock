# 快讯列表与全文搜索 · 设计

## 1. 方案概述

后端定时拉取第三方源 → 去重入库 → 提供列表/搜索接口；前端轮询刷新。
检索一期用 MySQL（`FULLTEXT` 或 LIKE + 索引），**接口契约与检索实现解耦**，
后续换 Meilisearch 前端零改动。

## 2. 页面规格（pages/news）

### 快讯列表页
- **入口**：底部 Tab「快讯」
- **顶部**：搜索框（点击进入搜索态）+ 刷新状态指示
- **列表项**：时间(HH:mm) · 正文(最多 3 行折叠) · 来源标签 · [可选]题材标签
- **交互**：下拉刷新 / 上拉分页 / 点击展开或跳原文 / 新消息横幅点击回顶
- **状态**：
  - 加载：骨架屏 5 条
  - 空态："暂无快讯"
  - 错误：图标 + "加载失败，点击重试"
  - 分页到底："没有更多了"

### 搜索态
- 搜索框聚焦后显示历史搜索（本地存储，最多 10 条，可清空）
- 结果项与列表项一致 + **命中词高亮**
- 排序切换：时间 / 相关度
- 空结果："没有找到相关快讯，换个词试试"

## 3. API 契约（前后端并行的前提）

> 统一响应 `{ "code": 0, "msg": "", "data": {} }`；**时间字段一律毫秒时间戳 int64**。

### `GET /api/v1/flash/list`
| 项 | 内容 |
|---|------|
| 入参 | `page`(int,默认1) `size`(int,默认20,最大50) `sinceId`(int64,可选：只取比它新的) |
| 出参 | `{ list: FlashItem[], total: int64, hasMore: bool }` |
| 鉴权 | 否（免费） |

```
FlashItem {
  id        int64    // 主键
  content   string   // 正文
  source    string   // 来源名称
  sourceUrl string   // 原文链接，可能为空串
  publishAt int64    // 发布时间，毫秒时间戳（源发布时间，非入库时间）
  themeTags string[] // 关联题材名（可选，可能为空数组）
}
```

### `GET /api/v1/flash/search`
| 项 | 内容 |
|---|------|
| 入参 | `kw`(string,必填,1-50字) `page` `size` `sort`(time\|relevance,默认 relevance) |
| 出参 | `{ list: FlashItem[], total: int64, hasMore: bool }`，正文中命中词用 `<em>` 包裹 |
| 鉴权 | 否 |

### 错误码
| 码 | 含义 |
|---|------|
| 0 | 成功 |
| 40001 | 参数错误（如 kw 为空、size 超限） |
| 50001 | 服务内部错误 |
| 50002 | 数据源暂不可用（前端提示"数据更新中"） |

## 4. 数据模型

```sql
-- migrations/20260730_create_flash_news.sql
CREATE TABLE flash_news (
  id          BIGINT       PRIMARY KEY AUTO_INCREMENT,
  content     TEXT         NOT NULL COMMENT '正文',
  source      VARCHAR(64)  NOT NULL COMMENT '来源名称',
  source_url  VARCHAR(512) NOT NULL DEFAULT '' COMMENT '原文链接',
  source_uid  VARCHAR(128) NOT NULL COMMENT '源侧唯一标识，用于去重',
  publish_at  DATETIME     NOT NULL COMMENT '源发布时间',
  created_at  DATETIME     NOT NULL,
  updated_at  DATETIME     NOT NULL,
  UNIQUE KEY uk_source_uid (source, source_uid),
  KEY idx_publish_at (publish_at DESC),
  FULLTEXT KEY ft_content (content) WITH PARSER ngram
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='快讯';
```

**去重**：`(source, source_uid)` 唯一键，重复拉取自动忽略。
**注意**：`publish_at` 存源发布时间；源中断时前端显示"暂无最新"，**不要重复展示旧条目**。

## 5. 关键取舍

| 取舍 | 决定 | 理由 |
|-----|------|------|
| 检索方案 | 一期 MySQL ngram 全文索引 | 数据量小时够用；契约解耦，后续可换 Meilisearch |
| 刷新方式 | 前端轮询（30–60s） | 简单可靠；WebSocket 收益不足以抵消复杂度 |
| 正文存储 | 完整存库 | ⚠️ 依赖授权；未授权源**只存标题级摘要 + 链接** |

## 6. 风险与回退

- **风险 1（阻塞）**：第三方授权未落实 → 回退为仅自采公开源（政策/公告）
- **风险 2**：源不稳定 → 拉取失败不影响读接口；前端显示上次成功数据 + 更新时间
- **风险 3**：轮询压力 → 加 ETag/304 与服务端缓存；必要时降频至 60s
