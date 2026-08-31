# AI选股器查询记录 API 注册说明

## 概述
本文档说明如何将AI选股器查询记录的API接口注册到系统的`sys_apis`表中。

## 方法一：通过初始化代码（推荐用于新系统）

如果是全新的系统初始化，API会自动注册。相关代码已添加到：
- 文件：`server/source/system/api.go`
- 位置：`InitializeData` 方法中的 `entities` 数组

### 已添加的API记录

```go
{ApiGroup: "AI选股器查询记录", Method: "POST", Path: "/quant/screenerRecord/createScreenerRecord", Description: "创建AI选股器查询记录"},
{ApiGroup: "AI选股器查询记录", Method: "DELETE", Path: "/quant/screenerRecord/deleteScreenerRecord", Description: "删除AI选股器查询记录"},
{ApiGroup: "AI选股器查询记录", Method: "DELETE", Path: "/quant/screenerRecord/deleteScreenerRecordByIds", Description: "批量删除AI选股器查询记录"},
{ApiGroup: "AI选股器查询记录", Method: "PUT", Path: "/quant/screenerRecord/updateScreenerRecord", Description: "更新AI选股器查询记录"},
{ApiGroup: "AI选股器查询记录", Method: "GET", Path: "/quant/screenerRecord/findScreenerRecord", Description: "根据ID获取AI选股器查询记录"},
{ApiGroup: "AI选股器查询记录", Method: "GET", Path: "/quant/screenerRecord/getScreenerRecordList", Description: "获取AI选股器查询记录列表"},
```

### 触发初始化

如果需要重新初始化数据库：
1. 删除或清空数据库
2. 重启服务，系统会自动执行初始化脚本

## 方法二：直接执行SQL（推荐用于已有系统）

如果系统已经运行，数据库已经初始化，可以直接执行SQL插入API记录。

### 执行步骤

1. 连接到数据库
2. 执行以下SQL：

```sql
INSERT INTO `sys_apis` (`path`, `description`, `api_group`, `method`) VALUES
('/quant/screenerRecord/createScreenerRecord', '创建AI选股器查询记录', 'AI选股器查询记录', 'POST'),
('/quant/screenerRecord/deleteScreenerRecord', '删除AI选股器查询记录', 'AI选股器查询记录', 'DELETE'),
('/quant/screenerRecord/deleteScreenerRecordByIds', '批量删除AI选股器查询记录', 'AI选股器查询记录', 'DELETE'),
('/quant/screenerRecord/updateScreenerRecord', '更新AI选股器查询记录', 'AI选股器查询记录', 'PUT'),
('/quant/screenerRecord/findScreenerRecord', '根据ID获取AI选股器查询记录', 'AI选股器查询记录', 'GET'),
('/quant/screenerRecord/getScreenerRecordList', '获取AI选股器查询记录列表', 'AI选股器查询记录', 'GET');
```

3. 验证插入结果：

```sql
SELECT * FROM `sys_apis` WHERE `api_group` = 'AI选股器查询记录';
```

### SQL文件位置
完整的SQL脚本已保存在：`server/docs/screener_record_apis.sql`

## 方法三：通过系统管理界面

1. 登录系统管理后台
2. 进入 `系统管理` -> `API管理`
3. 点击 `同步API` 按钮
4. 系统会自动扫描并同步所有新增的API接口

## API权限配置

API注册后，需要为角色分配相应的权限：

1. 进入 `系统管理` -> `角色管理`
2. 选择需要配置的角色
3. 点击 `设置权限`
4. 在API权限列表中找到 `AI选股器查询记录` 分组
5. 勾选需要授权的API接口
6. 保存配置

## 注意事项

1. **避免重复插入**：执行SQL前请先检查是否已存在相同的API记录
2. **权限配置**：新增API默认没有权限，需要手动为角色分配
3. **路由一致性**：确保API路径与路由配置保持一致
4. **API分组**：所有相关API使用统一的分组名称 `AI选股器查询记录`

## 验证

### 1. 检查API是否注册成功

```sql
SELECT id, path, description, api_group, method 
FROM sys_apis 
WHERE api_group = 'AI选股器查询记录'
ORDER BY id;
```

应该返回6条记录。

### 2. 测试API访问

使用Postman或其他工具测试API接口：

```bash
# 获取列表（需要登录token）
GET http://localhost:8888/quant/screenerRecord/getScreenerRecordList?page=1&pageSize=10
Authorization: Bearer {your_token}
```

### 3. 检查权限配置

在系统管理界面查看角色的API权限，确认 `AI选股器查询记录` 分组下的API已正确显示。

## 相关文件

- API初始化代码：`server/source/system/api.go`
- SQL脚本：`server/docs/screener_record_apis.sql`
- API接口实现：`server/api/v1/quant/screener_record.go`
- 路由配置：`server/router/quant/screener_record.go`

## 常见问题

### Q: 执行SQL后API仍然不显示？
A: 尝试清除缓存或重启服务。

### Q: API已注册但无法访问？
A: 检查角色权限配置，确保当前用户的角色已授权相应的API。

### Q: 如何批量授权所有API？
A: 在角色管理中，可以勾选整个API分组来批量授权。
