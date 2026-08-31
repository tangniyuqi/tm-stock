# AI选股器查询记录功能测试指南

## 功能说明
当用户在AI选股器中进行查询时，系统会自动记录查询条件：
- 首次查询：创建新记录，times=1
- 重复查询：不创建新记录，将现有记录的times字段+1

## 测试步骤

### 1. 准备工作
确保后端服务已启动，数据库表 `addon_quant_screener_record` 已创建。

### 2. 首次查询测试
1. 登录系统
2. 进入"AI选股器"页面
3. 输入查询条件，例如："涨停"
4. 点击"查询"按钮
5. 查询成功后，检查数据库：
```sql
SELECT * FROM addon_quant_screener_record 
WHERE prompt = '涨停' 
ORDER BY id DESC LIMIT 1;
```
预期结果：
- 应该有一条新记录
- times 字段值为 1
- member_id 为当前登录用户ID

### 3. 重复查询测试
1. 在同一页面，再次输入相同的查询条件："涨停"
2. 点击"查询"按钮
3. 查询成功后，再次检查数据库：
```sql
SELECT * FROM addon_quant_screener_record 
WHERE prompt = '涨停' 
ORDER BY id DESC LIMIT 1;
```
预期结果：
- 记录数量没有增加（仍然是1条）
- times 字段值变为 2
- updated_at 时间已更新

### 4. 多次重复查询测试
1. 继续使用相同条件"涨停"查询3-5次
2. 每次查询后检查数据库
3. 预期结果：times 字段持续增加（3, 4, 5...）

### 5. 不同条件查询测试
1. 输入新的查询条件，例如："量比大于2"
2. 点击"查询"按钮
3. 检查数据库：
```sql
SELECT * FROM addon_quant_screener_record 
WHERE member_id = ? -- 当前用户ID
ORDER BY id DESC LIMIT 2;
```
预期结果：
- 应该有2条记录
- 一条是"涨停"（times > 1）
- 一条是"量比大于2"（times = 1）

### 6. 多用户隔离测试
1. 使用另一个账号登录
2. 输入相同的查询条件："涨停"
3. 查询成功后检查数据库：
```sql
SELECT * FROM addon_quant_screener_record 
WHERE prompt = '涨停'
ORDER BY member_id, id;
```
预期结果：
- 应该有2条"涨停"记录
- 每条记录的member_id不同
- 每个用户的记录独立计数

### 7. 日志检查
查看后端日志，应该能看到类似以下内容：

首次查询：
```
INFO 使用RecordQuery逻辑 member_id=1 prompt=涨停
INFO 创建新的查询记录 member_id=1 prompt=涨停
```

重复查询：
```
INFO 使用RecordQuery逻辑 member_id=1 prompt=涨停
INFO 更新查询次数 record_id=123 current_times=1
```

## 常见问题排查

### 问题1：每次查询都创建新记录
可能原因：
- RecordQuery方法未被调用
- 查询条件字符串不完全匹配（空格、标点等）

排查方法：
1. 检查后端日志是否有"使用RecordQuery逻辑"
2. 检查数据库中prompt字段的实际值
3. 确认API层的条件判断是否生效

### 问题2：times字段没有增加
可能原因：
- 数据库更新语句失败
- 事务回滚

排查方法：
1. 检查后端日志是否有"更新查询次数"
2. 检查是否有错误日志
3. 手动执行SQL测试更新语句

### 问题3：前端调用失败
可能原因：
- API路由未注册
- 权限验证失败

排查方法：
1. 打开浏览器开发者工具，查看Network标签
2. 检查API请求是否发送
3. 查看响应状态码和错误信息

## 性能考虑

- 记录操作是异步的，不会阻塞查询响应
- 前端静默失败，记录失败不影响用户体验
- 建议定期清理旧记录或归档数据

## 数据统计示例

```sql
-- 查看最活跃的用户
SELECT member_id, COUNT(*) as query_types, SUM(times) as total_queries
FROM addon_quant_screener_record 
GROUP BY member_id
ORDER BY total_queries DESC
LIMIT 10;

-- 查看最热门的查询条件
SELECT prompt, SUM(times) as total_times
FROM addon_quant_screener_record 
GROUP BY prompt
ORDER BY total_times DESC
LIMIT 20;

-- 查看今天的查询统计
SELECT COUNT(*) as record_count, SUM(times) as total_queries
FROM addon_quant_screener_record 
WHERE DATE(created_at) = CURDATE();
```
