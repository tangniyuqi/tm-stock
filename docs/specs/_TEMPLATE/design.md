# <需求名> · 设计

## 1. 方案概述

<一段话说清怎么做，以及为什么是这个方案>

## 2. 页面规格（前端）

### 页面：<名称>
- **入口**：
- **内容与字段**：
- **交互**：
- **状态**：加载骨架 / 空态 / 错误重试 / 无权限
- **移动端要点**：375px 表现、点击区域

## 3. API 契约（前后端并行的前提）

### `GET /api/v1/<resource>`
| 项 | 内容 |
|---|------|
| 入参 | `page`(int) `size`(int) |
| 出参 | `{ code, msg, data: { list: [], total: 0 } }` |
| 时间字段 | **毫秒时间戳 int64** |
| 错误码 | 40001=参数错误 / 40100=未登录 |

## 4. 数据模型

```sql
CREATE TABLE <table> (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
```
索引说明：

## 5. 关键取舍

<有争议的地方；重大决策另开 ADR>

## 6. 风险与回退

- 风险：
- 回退方式：<feature flag / 灰度 / 可关闭>
