# AI选股器查询记录功能实现说明

## 功能概述
在用户进行AI选股查询时，自动记录查询条件到数据库。如果同一用户使用相同的查询条件，则增加查询次数而不是创建新记录。

## 实现方式

### 后端实现

#### 1. 数据模型 (server/model/quant/screener_record.go)
```go
type ScreenerRecord struct {
    MemberId  *uint32  // 用户ID
    Prompt    *string  // 查询条件
    Times     *int     // 查询次数
    Status    *int8    // 状态
}
```

#### 2. 服务层 (server/service/quant/screener_record.go)
新增 `RecordQuery` 方法：
- 检查是否存在相同用户+相同prompt的记录
- 如果不存在：创建新记录，times=1
- 如果存在：将该记录的times字段+1

#### 3. API层 (server/api/v1/quant/screener_record.go)
修改 `CreateScreenerRecord` 方法：
- 当请求包含prompt字段时，自动调用RecordQuery逻辑
- 实现去重和计数功能
- 保持向后兼容，不影响其他创建场景

### 前端实现 (web/src/view/quant/universe/index.vue)

#### 1. 导入API
```javascript
import { createScreenerRecord } from '@/api/quant/screenerRecord';
```

#### 2. 记录函数
```javascript
const recordQueryToDatabase = async (promptText) => {
  try {
    await createScreenerRecord({
      prompt: promptText
    });
  } catch (error) {
    console.error('记录查询失败:', error);
  }
};
```

#### 3. 调用时机
在 `handleSearch` 函数中，查询成功后异步调用：
```javascript
if (res.code === 0) {
  // ... 其他逻辑
  recordQueryToDatabase(query.value.trim());
}
```

## 特点

1. **去重机制**：同一用户的相同查询条件只保存一条记录
2. **计数功能**：重复查询时自动增加times字段
3. **异步处理**：前端异步调用，不阻塞用户操作
4. **静默失败**：记录失败不影响用户查询体验
5. **自动关联**：后端自动获取当前登录用户ID

## 数据流程

```
用户输入查询条件
    ↓
前端调用queryWencai API
    ↓
查询成功
    ↓
前端异步调用createScreenerRecord
    ↓
后端检查是否存在相同记录
    ↓
存在：times+1  /  不存在：创建新记录
```

## 注意事项

1. 记录操作是异步的，不会影响查询性能
2. 用户ID由后端自动获取，前端无需传递
3. 记录失败不会影响正常的查询功能
4. 支持多用户隔离，每个用户的记录独立统计
