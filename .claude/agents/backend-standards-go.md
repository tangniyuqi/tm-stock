# Go 后端开发规范

## 目录结构（标准布局）

```
server/
├── cmd/<app>/main.go          # 程序入口，只做装配
├── internal/                  # 私有代码（外部不可导入）
│   ├── handler/               # HTTP 层：参数绑定 + 调 service + 返回
│   ├── service/               # 业务逻辑（核心）
│   ├── repository/            # 数据访问（DB/缓存）
│   ├── model/                 # 领域模型 / DO
│   ├── dto/                   # 请求/响应结构体
│   └── middleware/            # 中间件
├── pkg/                       # 可被外部复用的公共库
├── configs/                   # 配置文件（不含密钥）
└── migrations/                # 数据库迁移脚本
```

## 分层铁律

1. **Handler 是薄层**：只做参数绑定 → 调 Service → 返回响应。
   ❌ 禁止在 Handler 写业务逻辑、❌ 禁止 Handler 直接访问 DB。
2. **Service 承载业务**：事务边界在 Service。
3. **Repository 只管数据**：不含业务判断。
4. 依赖方向单向：`handler → service → repository`，**禁止反向或跨层**。

## 错误处理

- 统一错误类型（如 `pkg/errcode`），**禁止裸 `errors.New` 散落各处**。
- 错误必须 `%w` 包装并携带上下文：`fmt.Errorf("查询用户失败 uid=%d: %w", uid, err)`。
- ❌ 禁止吞错（`_ = err`）；确实要忽略必须写注释说明原因。
- Handler 层统一转换为 API 错误码，**不要把内部错误细节透给前端**。

## 上下文与并发

- 所有跨进程调用（DB/HTTP/MQ）**必须传 `context.Context`** 并设超时。
- goroutine 必须有明确退出路径（ctx 取消 / channel 关闭），**禁止裸 `go func()` 无控制**。
- 共享状态用 mutex 或 channel，**跑 `go test -race` 必须干净**。

## API 约定

- RESTful 路径小写连字符：`/api/v1/theme-list`。
- 统一响应包装：`{ "code": 0, "msg": "", "data": {} }`。
- **时间字段统一用毫秒时间戳（int64）**，禁止返回格式化字符串（避免前端类型地狱）。
- 分页与详情**用不同的 DTO**：列表 DTO 只含主表字段、禁止关联查询。

## 数据库

- 迁移脚本命名：`migrations/YYYYMMDD_<操作>_<描述>.sql`（禁止 `V1__` 版本号前缀）。
- 表名小写下划线复数、字段小写下划线；每张业务表含 `created_at / updated_at`。
- ❌ 禁止在代码里拼 SQL 字符串（用参数化查询 / query builder）。
- 索引与慢查询：新增查询条件必须评估索引。

## 配置与密钥

- 配置从**环境变量 + 配置文件**读取，**代码里禁止硬编码**。
- 密钥只从环境变量读，仓库内**只允许占位符**（见 CLAUDE.md#凭据约定）。

## 测试

- Service 层必须有单测；表驱动测试（table-driven）为默认写法。
- 关键路径覆盖率目标 ≥ 70%（新增代码，ratchet 模式）。
- 跑 `go test ./... -race` 必须绿。

## 提交前

```bash
gofmt -w .
go vet ./...
golangci-lint run
bash scripts/harness-checks.sh
```
