# AI选股器查询记录功能

## 功能说明
该模块用于管理AI选股器的查询记录，记录用户使用AI选股器的历史查询信息。

## 数据库表结构
表名：`addon_quant_screener_record`

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int | 主键ID |
| member_id | int | 用户ID |
| prompt | varchar(250) | 提示词 |
| times | int | 查询次数 |
| status | tinyint | 状态（1:正常 0:禁用 -1:删除） |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |
| deleted_at | datetime | 删除时间 |
| created_by | bigint | 创建者 |
| updated_by | bigint | 更新者 |
| deleted_by | bigint | 删除者 |

## 功能特性
- ✅ 查询记录列表（支持分页）
- ✅ 新增查询记录
- ✅ 编辑查询记录
- ✅ 删除查询记录（单个/批量）
- ✅ 查看记录详情
- ✅ 按用户ID、提示词、状态筛选

## 文件结构

### 后端文件
```
server/
├── model/quant/
│   ├── screener_record.go              # 数据模型
│   └── request/screener_record.go      # 请求参数
├── service/quant/
│   └── screener_record.go              # 业务逻辑
├── api/v1/quant/
│   └── screener_record.go              # API接口
└── router/quant/
    └── screener_record.go              # 路由注册
```

### 前端文件
```
web/src/
├── api/quant/
│   └── screenerRecord.js               # API调用
└── view/quant/screenerRecord/
    └── index.vue                       # 页面组件
```

## API接口

### 基础路径
`/quant/screenerRecord`

### 接口列表
- `POST /createScreenerRecord` - 创建记录
- `DELETE /deleteScreenerRecord` - 删除记录
- `DELETE /deleteScreenerRecordByIds` - 批量删除
- `PUT /updateScreenerRecord` - 更新记录
- `GET /findScreenerRecord` - 查询单条记录
- `GET /getScreenerRecordList` - 获取记录列表

## 使用说明

1. 访问路由：`/quant/screenerRecord`（需要在路由配置中添加）
2. 页面支持的操作：
   - 查询：可按用户ID、提示词、状态进行筛选
   - 新增：点击"新增"按钮，填写表单后提交
   - 编辑：点击列表中的"编辑"按钮
   - 删除：支持单个删除和批量删除
   - 查看：点击"查看"按钮查看详细信息

## 注意事项
- 所有接口都需要登录认证
- 删除操作为软删除，数据不会真正从数据库中移除
- 用户ID会自动填充为当前登录用户
