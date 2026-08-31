# Requirements Document - A股股票题材宝典

## Introduction

A股股票题材宝典是一个题材管理系统，用于管理A股市场的各种投资题材及其关联的股票。该系统支持题材的增删改查操作，以及题材与股票之间的多对多关联关系管理，帮助用户更好地理解和跟踪不同题材板块的股票。

## Glossary

- **题材宝典系统（Theme_Encyclopedia_System）**: 管理A股市场投资题材及其关联股票的系统
- **题材（Theme）**: A股市场中的投资概念或板块分类，如"新能源汽车"、"人工智能"等
- **股票（Stock）**: A股市场中的上市公司股票
- **题材-股票关联（Theme_Stock_Relation）**: 题材与股票之间的关联关系
- **用户（User）**: 使用题材宝典系统的操作者
- **有效题材**: 包含必要信息（名称、描述）且未被删除的题材记录

## Requirements

### 需求 1: 题材基础管理

**用户故事**: 作为系统用户，我希望能够管理题材的基础信息，以便组织和分类不同的投资概念。

#### 验收标准

1. THE Theme_Encyclopedia_System SHALL 支持创建新题材，包含题材名称、描述、标签等基础信息
2. WHEN 用户提交题材创建请求，THE Theme_Encyclopedia_System SHALL 验证题材名称不为空且长度在1-100字符之间
3. WHEN 用户提交题材创建请求，THE Theme_Encyclopedia_System SHALL 验证题材名称在系统中唯一
4. THE Theme_Encyclopedia_System SHALL 支持更新已存在题材的基础信息
5. WHEN 用户更新题材信息时，THE Theme_Encyclopedia_System SHALL 记录更新时间和操作人
6. THE Theme_Encyclopedia_System SHALL 支持删除题材记录
7. WHEN 删除题材时，THE Theme_Encyclopedia_System SHALL 同时删除该题材与所有股票的关联关系
8. THE Theme_Encyclopedia_System SHALL 支持通过题材ID查询单个题材详情

### 需求 2: 题材列表查询

**用户故事**: 作为系统用户，我希望能够浏览和搜索题材列表，以便快速找到感兴趣的投资题材。

#### 验收标准

1. THE Theme_Encyclopedia_System SHALL 支持分页查询题材列表
2. WHEN 用户请求题材列表时，THE Theme_Encyclopedia_System SHALL 返回题材的基础信息（ID、名称、描述、创建时间、更新时间）
3. THE Theme_Encyclopedia_System SHALL 支持按题材名称进行模糊搜索
4. THE Theme_Encyclopedia_System SHALL 支持按题材标签进行筛选
5. THE Theme_Encyclopedia_System SHALL 支持按创建时间或更新时间排序题材列表
6. WHEN 题材列表为空时，THE Theme_Encyclopedia_System SHALL 返回空列表和总数为0

### 需求 3: 题材与股票关联管理

**用户故事**: 作为系统用户，我希望能够为题材关联相关的股票，以便建立题材与股票之间的映射关系。

#### 验收标准

1. THE Theme_Encyclopedia_System SHALL 支持为指定题材添加一只或多只关联股票
2. WHEN 用户添加题材-股票关联时，THE Theme_Encyclopedia_System SHALL 验证题材ID和股票ID均存在
3. WHEN 用户添加题材-股票关联时，THE Theme_Encyclopedia_System SHALL 检查关联关系是否已存在，避免重复添加
4. THE Theme_Encyclopedia_System SHALL 支持删除题材与股票之间的关联关系
5. THE Theme_Encyclopedia_System SHALL 支持批量删除题材的多个股票关联
6. WHEN 删除关联关系时，THE Theme_Encyclopedia_System SHALL 仅删除关联记录，不影响题材和股票本身

### 需求 4: 题材关联股票查询

**用户故事**: 作为系统用户，我希望能够查看某个题材下关联的所有股票，以便分析该题材板块的构成。

#### 验收标准

1. WHEN 用户查询指定题材的关联股票时，THE Theme_Encyclopedia_System SHALL 返回该题材下所有关联的股票列表
2. THE Theme_Encyclopedia_System SHALL 支持分页查询题材关联的股票列表
3. WHEN 查询题材关联股票时，THE Theme_Encyclopedia_System SHALL 返回股票的基础信息（股票代码、股票名称、市场）
4. WHEN 题材不存在时，THE Theme_Encyclopedia_System SHALL 返回错误提示
5. WHEN 题材存在但未关联任何股票时，THE Theme_Encyclopedia_System SHALL 返回空列表和总数为0

### 需求 5: 股票关联题材查询

**用户故事**: 作为系统用户，我希望能够查看某只股票属于哪些题材，以便了解该股票的概念属性。

#### 验收标准

1. WHEN 用户查询指定股票的关联题材时，THE Theme_Encyclopedia_System SHALL 返回该股票所属的所有题材列表
2. THE Theme_Encyclopedia_System SHALL 支持分页查询股票关联的题材列表
3. WHEN 查询股票关联题材时，THE Theme_Encyclopedia_System SHALL 返回题材的基础信息（题材名称、描述、标签）
4. WHEN 股票不存在时，THE Theme_Encyclopedia_System SHALL 返回错误提示
5. WHEN 股票存在但未关联任何题材时，THE Theme_Encyclopedia_System SHALL 返回空列表和总数为0

### 需求 6: 批量操作支持

**用户故事**: 作为系统用户，我希望能够批量管理题材和关联关系，以便提高操作效率。

#### 验收标准

1. THE Theme_Encyclopedia_System SHALL 支持批量删除多个题材
2. WHEN 批量删除题材时，THE Theme_Encyclopedia_System SHALL 同时删除这些题材的所有股票关联关系
3. THE Theme_Encyclopedia_System SHALL 支持批量为题材添加多只股票
4. THE Theme_Encyclopedia_System SHALL 支持批量删除题材的多个股票关联
5. WHEN 批量操作部分失败时，THE Theme_Encyclopedia_System SHALL 返回详细的成功和失败信息

### 需求 7: 数据完整性和权限控制

**用户故事**: 作为系统用户，我希望系统能够保证数据的完整性和安全性，以便保护数据不被误操作。

#### 验收标准

1. THE Theme_Encyclopedia_System SHALL 记录每个题材的创建人和创建时间
2. THE Theme_Encyclopedia_System SHALL 记录每个题材的最后更新人和更新时间
3. WHEN 用户创建或修改题材时，THE Theme_Encyclopedia_System SHALL 验证用户已登录并具有操作权限
4. THE Theme_Encyclopedia_System SHALL 确保题材与股票的关联关系在数据库层面具有外键约束
5. WHEN 尝试关联不存在的股票ID时，THE Theme_Encyclopedia_System SHALL 返回明确的错误提示
6. THE Theme_Encyclopedia_System SHALL 支持软删除机制，保留已删除题材的历史记录

### 需求 8: API接口规范

**用户故事**: 作为系统开发者，我希望题材宝典提供标准的RESTful API接口，以便前端和其他系统集成调用。

#### 验收标准

1. THE Theme_Encyclopedia_System SHALL 提供创建题材的POST接口 `/theme/createTheme`
2. THE Theme_Encyclopedia_System SHALL 提供更新题材的PUT接口 `/theme/updateTheme`
3. THE Theme_Encyclopedia_System SHALL 提供删除题材的DELETE接口 `/theme/deleteTheme`
4. THE Theme_Encyclopedia_System SHALL 提供批量删除题材的DELETE接口 `/theme/deleteThemeByIds`
5. THE Theme_Encyclopedia_System SHALL 提供查询题材详情的GET接口 `/theme/findTheme`
6. THE Theme_Encyclopedia_System SHALL 提供分页查询题材列表的GET接口 `/theme/getThemeList`
7. THE Theme_Encyclopedia_System SHALL 提供添加题材-股票关联的POST接口 `/theme/addStocks`
8. THE Theme_Encyclopedia_System SHALL 提供删除题材-股票关联的DELETE接口 `/theme/removeStocks`
9. THE Theme_Encyclopedia_System SHALL 提供查询题材关联股票的GET接口 `/theme/getThemeStocks`
10. THE Theme_Encyclopedia_System SHALL 提供查询股票关联题材的GET接口 `/theme/getStockThemes`
11. WHEN API调用成功时，THE Theme_Encyclopedia_System SHALL 返回标准的成功响应格式（包含code、data、msg）
12. WHEN API调用失败时，THE Theme_Encyclopedia_System SHALL 返回标准的错误响应格式（包含code、msg和详细错误信息）
