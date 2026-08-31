# 实现计划: A股股票题材宝典

## 概述

本实现计划将 A股股票题材宝典的技术设计转化为可执行的开发任务。系统采用标准的三层架构（API -> Service -> Model），基于 gin-vue-admin 框架，使用 Go 语言和 GORM ORM 实现题材的全生命周期管理和题材与股票之间的多对多关联关系管理。

实现将按照从底层到上层的顺序进行：首先创建数据模型层，然后实现业务逻辑层，最后实现 API 层并配置路由。

## 任务清单

- [ ] 1. 创建数据模型和请求参数结构体
  - [ ] 1.1 创建题材数据模型 (model/quant/theme.go)
    - 定义 Theme 结构体，包含所有字段（Name, Description, Tags, CreatedBy, UpdatedBy, DeletedBy）
    - 实现 TableName() 方法返回 "addon_quant_theme"
    - 定义与 BaseStock 的多对多关联关系
    - _需求: 1.1, 1.5, 7.1, 7.2_

  - [ ] 1.2 创建题材-股票关联模型 (model/quant/theme_stock.go)
    - 定义 ThemeStock 结构体，包含 ThemeID, StockID, CreatedAt 字段
    - 实现 TableName() 方法返回 "addon_quant_theme_stock"
    - 设置联合主键 (ThemeID, StockID)
    - _需求: 3.1, 7.4_

  - [ ] 1.3 创建请求参数结构体 (model/quant/request/theme.go)
    - 定义 ThemeSearch 结构体（Name, Tags, OrderKey, OrderDesc, PageInfo）
    - 定义 ThemeStockRequest 结构体（ThemeID, StockIDs，带 binding 验证标签）
    - 定义 ThemeStockSearch 结构体（ThemeID, PageInfo）
    - 定义 StockThemeSearch 结构体（StockID, PageInfo）
    - _需求: 2.1, 2.3, 2.4, 2.5, 3.1, 4.1, 5.1_

- [ ] 2. 实现 Service 层业务逻辑
  - [ ] 2.1 实现题材创建服务 (service/quant/theme.go - CreateTheme)
    - 验证题材名称非空且长度在 1-100 字符
    - 检查题材名称唯一性
    - 创建题材记录并处理数据库唯一约束冲突错误
    - _需求: 1.1, 1.2, 1.3_

  - [ ] 2.2 实现题材更新服务 (service/quant/theme.go - UpdateTheme)
    - 验证题材存在性
    - 验证题材名称唯一性（排除自身）
    - 更新题材信息并记录更新人和更新时间
    - _需求: 1.4, 1.5_

  - [ ] 2.3 实现题材删除服务 (service/quant/theme.go - DeleteTheme)
    - 使用数据库事务保证原子性
    - 软删除题材记录（更新 deleted_by 和 deleted_at）
    - 物理删除该题材的所有股票关联关系
    - _需求: 1.6, 1.7, 7.6_

  - [ ] 2.4 实现题材批量删除服务 (service/quant/theme.go - DeleteThemeByIds)
    - 使用数据库事务保证原子性
    - 批量软删除题材记录
    - 批量删除这些题材的所有股票关联关系
    - _需求: 6.1, 6.2_

  - [ ] 2.5 实现题材查询服务 (service/quant/theme.go - GetTheme)
    - 根据 ID 查询题材详细信息
    - 题材不存在时返回明确错误
    - _需求: 1.8_

  - [ ] 2.6 实现题材列表分页查询服务 (service/quant/theme.go - GetThemeInfoList)
    - 支持按名称模糊搜索（LIKE 查询）
    - 支持按标签筛选（LIKE 查询）
    - 支持按 created_at 或 updated_at 排序（升序/降序）
    - 实现分页逻辑，返回分页结果和总数
    - _需求: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6_

  - [ ] 2.7 实现添加题材-股票关联服务 (service/quant/theme.go - AddStocks)
    - 验证题材存在性
    - 验证所有股票存在性
    - 检查关联关系是否已存在，避免重复添加（使用联合主键自动去重或手动检查）
    - 批量创建关联记录
    - _需求: 3.1, 3.2, 3.3, 6.3_

  - [ ] 2.8 实现删除题材-股票关联服务 (service/quant/theme.go - RemoveStocks)
    - 验证题材存在性
    - 批量删除指定的关联关系
    - 不影响题材和股票本身
    - _需求: 3.4, 3.5, 3.6, 6.4_

  - [ ] 2.9 实现查询题材关联股票服务 (service/quant/theme.go - GetThemeStocks)
    - 验证题材存在性
    - 通过关联表 JOIN 查询股票信息
    - 返回股票的基础信息（代码、名称、市场等）
    - 支持分页查询
    - _需求: 4.1, 4.2, 4.3, 4.4, 4.5_

  - [ ] 2.10 实现查询股票关联题材服务 (service/quant/theme.go - GetStockThemes)
    - 验证股票存在性
    - 通过关联表 JOIN 查询题材信息
    - 返回题材的基础信息（名称、描述、标签等）
    - 支持分页查询
    - _需求: 5.1, 5.2, 5.3, 5.4, 5.5_

- [ ] 3. Checkpoint - 验证 Service 层逻辑
  - 确保所有 Service 方法编译通过
  - 验证数据验证逻辑完整
  - 确认事务处理正确
  - 如有疑问请向用户确认

- [ ] 4. 实现 API 层处理器
  - [ ] 4.1 实现创建题材 API (api/v1/quant/theme.go - CreateTheme)
    - 绑定 JSON 请求体到 Theme 结构体
    - 获取当前用户 ID 并设置 CreatedBy 字段
    - 调用 Service 层 CreateTheme 方法
    - 返回标准成功/失败响应
    - _需求: 8.1, 8.11, 8.12_

  - [ ] 4.2 实现更新题材 API (api/v1/quant/theme.go - UpdateTheme)
    - 绑定 JSON 请求体到 Theme 结构体
    - 获取当前用户 ID 并设置 UpdatedBy 字段
    - 调用 Service 层 UpdateTheme 方法
    - 返回标准成功/失败响应
    - _需求: 8.2, 8.11, 8.12_

  - [ ] 4.3 实现删除题材 API (api/v1/quant/theme.go - DeleteTheme)
    - 从 query 参数获取题材 ID
    - 获取当前用户 ID
    - 调用 Service 层 DeleteTheme 方法
    - 返回标准成功/失败响应
    - _需求: 8.3, 8.11, 8.12_

  - [ ] 4.4 实现批量删除题材 API (api/v1/quant/theme.go - DeleteThemeByIds)
    - 从 query 参数获取题材 ID 数组
    - 获取当前用户 ID
    - 调用 Service 层 DeleteThemeByIds 方法
    - 返回标准成功/失败响应
    - _需求: 8.4, 8.11, 8.12_

  - [ ] 4.5 实现查询题材详情 API (api/v1/quant/theme.go - FindTheme)
    - 从 query 参数获取题材 ID
    - 调用 Service 层 GetTheme 方法
    - 返回标准成功/失败响应（成功时包含 data）
    - _需求: 8.5, 8.11, 8.12_

  - [ ] 4.6 实现分页查询题材列表 API (api/v1/quant/theme.go - GetThemeList)
    - 绑定 query 参数到 ThemeSearch 结构体
    - 调用 Service 层 GetThemeInfoList 方法
    - 构造 PageResult 响应（包含 list 和 total）
    - 返回标准成功/失败响应
    - _需求: 8.6, 8.11, 8.12_

  - [ ] 4.7 实现添加题材-股票关联 API (api/v1/quant/theme.go - AddStocks)
    - 绑定 JSON 请求体到 ThemeStockRequest 结构体
    - 调用 Service 层 AddStocks 方法
    - 返回标准成功/失败响应
    - _需求: 8.7, 8.11, 8.12_

  - [ ] 4.8 实现删除题材-股票关联 API (api/v1/quant/theme.go - RemoveStocks)
    - 绑定 JSON 请求体到 ThemeStockRequest 结构体
    - 调用 Service 层 RemoveStocks 方法
    - 返回标准成功/失败响应
    - _需求: 8.8, 8.11, 8.12_

  - [ ] 4.9 实现查询题材关联股票 API (api/v1/quant/theme.go - GetThemeStocks)
    - 绑定 query 参数到 ThemeStockSearch 结构体
    - 调用 Service 层 GetThemeStocks 方法
    - 构造 PageResult 响应
    - 返回标准成功/失败响应
    - _需求: 8.9, 8.11, 8.12_

  - [ ] 4.10 实现查询股票关联题材 API (api/v1/quant/theme.go - GetStockThemes)
    - 绑定 query 参数到 StockThemeSearch 结构体
    - 调用 Service 层 GetStockThemes 方法
    - 构造 PageResult 响应
    - 返回标准成功/失败响应
    - _需求: 8.10, 8.11, 8.12_

- [ ] 5. 注册模块和配置路由
  - [ ] 5.1 更新 Service 层 enter.go 注册 ThemeService
    - 在 service/quant/enter.go 的 ServiceGroup 中添加 ThemeService 字段
    - _需求: 所有 Service 层需求_

  - [ ] 5.2 更新 API 层 enter.go 注册 ThemeApi 和 Service
    - 在 api/v1/quant/enter.go 的 ApiGroup 中添加 ThemeApi 字段
    - 在 var 区域添加 themeService 变量初始化
    - _需求: 所有 API 层需求_

  - [ ] 5.3 创建题材路由配置 (router/quant/theme.go)
    - 创建 ThemeRouter 结构体
    - 实现 InitThemeRouter 方法，配置所有 10 个路由
    - 为变更操作添加 OperationRecord 中间件（POST, PUT, DELETE）
    - 为所有操作添加 Auth 中间件
    - _需求: 8.1-8.10_

  - [ ] 5.4 更新路由 enter.go 注册 ThemeRouter
    - 在 router/quant/enter.go 的 RouterGroup 中添加 ThemeRouter 字段
    - _需求: 所有路由需求_

  - [ ] 5.5 在主路由初始化中注册题材路由组
    - 在 initialize/router.go 中调用 InitThemeRouter
    - 确保路由前缀为 /quant/theme
    - _需求: 所有路由需求_

- [ ] 6. Checkpoint - 验证 API 和路由配置
  - 确保所有代码编译通过
  - 验证路由注册正确
  - 确认中间件配置完整
  - 如有疑问请向用户确认

- [ ] 7. 数据库迁移和初始化
  - [ ] 7.1 创建数据库迁移脚本
    - 使用 GORM AutoMigrate 创建 Theme 和 ThemeStock 表
    - 添加题材名称唯一索引 (idx_theme_name)
    - 添加软删除索引 (idx_theme_deleted_at)
    - 添加关联表的 stock_id 索引 (idx_stock_id)
    - 添加外键约束（如果数据库支持）
    - _需求: 1.3, 7.4, 7.6_

  - [ ] 7.2 测试数据库迁移
    - 运行迁移脚本验证表创建成功
    - 验证索引和约束正确创建
    - 确认表结构符合设计文档
    - _需求: 所有数据模型需求_

- [ ]\* 8. 编写单元测试
  - [ ]\* 8.1 编写 Service 层单元测试 (service/quant/theme_test.go)
    - 测试 CreateTheme 的正常流程和错误场景（空名称、名称过长、重复名称）
    - 测试 UpdateTheme 的正常流程和错误场景（题材不存在、名称冲突）
    - 测试 DeleteTheme 的级联删除逻辑
    - 测试 DeleteThemeByIds 的批量删除逻辑
    - 测试 GetTheme 的查询逻辑（存在和不存在场景）
    - 测试 GetThemeInfoList 的分页、搜索、筛选、排序功能
    - 测试 AddStocks 的关联创建和去重逻辑
    - 测试 RemoveStocks 的关联删除逻辑
    - 测试 GetThemeStocks 和 GetStockThemes 的双向查询
    - _需求: 所有 Service 层需求_

  - [ ]\* 8.2 编写 API 层单元测试 (api/v1/quant/theme_test.go)
    - 使用 httptest 模拟 HTTP 请求
    - 测试所有 10 个 API 接口的正常流程
    - 测试参数验证错误场景
    - 测试响应格式正确性（code, msg, data）
    - _需求: 8.1-8.12_

- [ ]\* 9. 编写属性测试
  - [ ]\* 9.1 设置属性测试框架
    - 引入 gopter 或 rapid 属性测试库
    - 配置每个属性测试运行至少 100 次迭代
    - 创建测试数据生成器（genThemeName, genThemeTags, genStockIDs）
    - _需求: 所有正确性属性_

  - [ ]\* 9.2 实现属性 1-6 的测试
    - **属性 1: 题材创建和查询往返一致性** - 验证创建后查询数据一致
    - **属性 2: 题材名称唯一性约束** - 验证同名题材创建失败
    - **属性 3: 题材更新完整性** - 验证更新后数据正确且时间戳更新
    - **属性 4: 题材软删除及级联删除** - 验证软删除和关联删除
    - **属性 5: 批量删除题材的完整性** - 验证批量删除的完整性
    - **属性 6: 分页查询的一致性** - 验证分页结果完整无重复
    - _需求: 1.1-1.8, 6.1-6.2, 7.6_

  - [ ]\* 9.3 实现属性 7-12 的测试
    - **属性 7: 模糊搜索的准确性** - 验证搜索结果包含关键词
    - **属性 8: 标签筛选的准确性** - 验证筛选结果包含标签
    - **属性 9: 排序功能的正确性** - 验证排序正确性
    - **属性 10: 题材-股票关联的创建和查询一致性** - 验证双向查询一致
    - **属性 11: 关联关系的幂等性** - 验证重复添加不产生重复记录
    - **属性 12: 删除关联的隔离性** - 验证删除关联不影响题材和股票
    - _需求: 2.3-2.5, 3.1-3.6, 4.1, 5.1_

  - [ ]\* 9.4 实现属性 13-18 的测试
    - **属性 13: 批量删除关联的完整性** - 验证批量删除的完整性
    - **属性 14: 题材关联股票分页查询的一致性** - 验证分页结果完整
    - **属性 15: 股票关联题材分页查询的一致性** - 验证分页结果完整
    - **属性 16: 审计字段的完整性** - 验证审计字段正确设置
    - **属性 17: API 成功响应格式的一致性** - 验证成功响应格式
    - **属性 18: API 错误响应格式的一致性** - 验证错误响应格式
    - _需求: 3.5, 4.2, 5.2, 7.1-7.2, 8.11-8.12_

- [ ] 10. 最终验证和文档完善
  - [ ] 10.1 集成测试验证
    - 启动完整应用，验证所有 API 接口可访问
    - 测试完整的业务流程（创建题材 -> 添加关联 -> 查询 -> 更新 -> 删除）
    - 验证数据库约束和事务正确工作
    - 测试并发场景下的数据一致性
    - _需求: 所有需求_

  - [ ] 10.2 代码质量检查
    - 运行 golint 和 go vet 检查代码规范
    - 确保所有错误都有适当的日志记录
    - 验证所有 API 使用统一的响应格式
    - 检查代码注释完整性
    - _需求: 所有需求_

  - [ ] 10.3 性能测试和优化
    - 测试大数据量下的分页查询性能
    - 验证批量操作的性能
    - 检查 N+1 查询问题
    - 确认数据库索引正确使用
    - _需求: 2.1, 4.2, 5.2, 6.1-6.4_

  - [ ] 10.4 文档完善
    - 添加代码注释（所有 public 方法）
    - 编写 API 使用示例
    - 更新项目 README（如果需要）
    - 记录已知限制和未来改进方向
    - _需求: 所有需求_

## 注意事项

### 任务标记说明

- 标记 `*` 的任务为**可选任务**（主要是测试相关任务），可根据项目进度和优先级跳过
- 未标记的任务为**核心实现任务**，必须完成以实现功能

### 实现建议

1. **按顺序执行**: 任务按照依赖关系排列，建议按顺序执行以避免依赖问题
2. **增量验证**: 在每个 Checkpoint 暂停验证，确保当前阶段正确后再继续
3. **参考现有代码**: 参考 quant 模块中的其他实现（如 BaseStock、News 等）保持代码风格一致
4. **错误处理**: 所有 Service 方法都应该返回清晰的错误信息，便于调试和用户理解
5. **事务管理**: 涉及多表操作的场景（删除题材、批量操作）必须使用事务
6. **日志记录**: 在关键操作点添加日志（使用 global.GVA_LOG），包括 Info 和 Error 级别
7. **参数验证**: 在 Service 层进行业务验证，在 API 层使用 binding 标签进行基础验证
8. **测试数据**: 单元测试和属性测试都需要准备测试数据库和基础股票数据

### 技术要点

- **ORM 框架**: GORM v2
- **Web 框架**: Gin
- **响应格式**: 使用 response.Response 结构（code, msg, data）
- **分页结构**: 使用 response.PageResult 结构（list, total, page, pageSize）
- **中间件**: JWT 认证、操作记录
- **软删除**: 使用 GORM 的 DeletedAt 字段
- **外键约束**: 建议在迁移脚本中添加，确保引用完整性

### 需求覆盖

所有任务都明确标注了对应的需求编号（_需求: X.Y_），确保需求文档的所有验收标准都被实现任务覆盖。实现过程中如发现需求不明确或有冲突，应及时向用户确认。

## 任务依赖图

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2", "1.3"] },
    { "id": 1, "tasks": ["2.1", "2.2", "2.3", "2.4", "2.5", "2.6"] },
    { "id": 2, "tasks": ["2.7", "2.8", "2.9", "2.10"] },
    { "id": 3, "tasks": ["4.1", "4.2", "4.3", "4.4", "4.5", "4.6"] },
    { "id": 4, "tasks": ["4.7", "4.8", "4.9", "4.10"] },
    { "id": 5, "tasks": ["5.1", "5.2"] },
    { "id": 6, "tasks": ["5.3"] },
    { "id": 7, "tasks": ["5.4", "5.5"] },
    { "id": 8, "tasks": ["7.1"] },
    { "id": 9, "tasks": ["7.2"] },
    { "id": 10, "tasks": ["8.1", "8.2"] },
    { "id": 11, "tasks": ["9.1"] },
    { "id": 12, "tasks": ["9.2", "9.3", "9.4"] },
    { "id": 13, "tasks": ["10.1", "10.2", "10.3", "10.4"] }
  ]
}
```
