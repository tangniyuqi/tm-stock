# tm-stock 后端（Go + 标准库 net/http）

## 分层（强制）

```
cmd/api/main.go     只做装配
internal/
  handler/          HTTP 薄层：参数绑定 → 调 service → 返回
  service/          业务逻辑 + 事务边界
  repository/       数据访问
  model/ dto/       领域模型 / 传输结构
  middleware/
pkg/                可复用公共库
configs/            配置（不含密钥）
migrations/         SQL 迁移：YYYYMMDD_<操作>_<描述>.sql
```

依赖方向单向：`handler → service → repository`，禁止反向或跨层。

## 开发

```bash
cd server
go mod tidy
go run ./cmd/api        # 默认 :8080，healthz 可探活
go run ./cmd/schema-check  # 只读核对目标库结构；不执行迁移、不写数据
gofmt -w . && go vet ./... && go test ./... -race
```

## 约定要点

- 时间字段对外统一 **毫秒时间戳（int64）**
- 统一响应：`{ "code": 0, "msg": "", "data": {} }`
- 密钥只从环境变量读（`TM_*`），仓库内只允许占位符
- 详见 [`../.claude/agents/backend-standards-go.md`](../.claude/agents/backend-standards-go.md)

## 🔴 合规

后端返回的**任何文案字段**同样受合规红线约束（禁用词表机器检查）。
一期**不提供个股详情接口**（基本面、财务、K 线、技术指标等）；仅可通过题材路径查询
成分股的客观归属依据，见 [`../.claude/agents/compliance-redline.md`](../.claude/agents/compliance-redline.md)。
