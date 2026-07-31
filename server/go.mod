module github.com/tangniyuqi/tm-stock/server

go 1.24.0

// 依赖在首次 go mod tidy 时补齐：
// require github.com/gin-gonic/gin v1.10.0

require github.com/go-sql-driver/mysql v1.10.0

require filippo.io/edwards25519 v1.2.0 // indirect
