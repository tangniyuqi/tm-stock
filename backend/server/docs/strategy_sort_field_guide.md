# 策略排序字段添加说明

## 修改概述
为策略（Strategy）模块添加 `sort` 字段，支持自定义排序功能。

## 修改内容

### 1. 数据库变更
- 文件：`server/docs/add_strategy_sort_field.sql`
- 执行 SQL 脚本为 `addon_quant_strategy` 表添加 `sort` 字段
- 字段类型：INT(10)，默认值：0
- 添加索引以优化查询性能

### 2. 后端模型变更
- 文件：`server/model/quant/strategy.go`
- 在 `Strategy` 结构体中添加 `Sort` 字段
- 字段定义：`Sort *int json:"sort" form:"sort" gorm:"default:0;comment:排序;column:sort;size:10;"`

### 3. 后端服务层变更
- 文件：`server/service/quant/strategy.go`
- 在 `GetStrategyInfoList` 方法中添加排序逻辑
- 默认按 `sort` 字段倒序排序（DESC）

### 4. 前端变更
- 文件：`web/src/view/quant/strategy/strategy.vue`
- 表格中添加"排序"列，支持自定义排序
- 新增/编辑表单中添加排序输入框（使用 `el-input-number` 组件）
- 详情查看中添加排序字段显示
- `formData` 中添加 `sort: 0` 字段

## 使用说明

### 数据库迁移
执行以下命令应用数据库变更：
```bash
mysql -u [username] -p [database_name] < server/docs/add_strategy_sort_field.sql
```

### 排序规则
- 数值越大，排序越靠前（倒序排列）
- 默认值为 0
- 支持前端表格列排序功能

### 注意事项
- 现有数据的 `sort` 字段会自动设置为默认值 0
- 如需为现有数据设置初始排序值，可取消注释 SQL 脚本中的 UPDATE 语句
- GORM 会在下次启动时自动识别新字段（如果使用 AutoMigrate）

## 测试建议
1. 创建多个策略并设置不同的 `sort` 值
2. 验证列表页按 `sort` 倒序显示
3. 测试前端表格列排序功能
4. 验证新增/编辑/查看功能正常
