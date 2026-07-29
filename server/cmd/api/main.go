// Package main 是 tm-stock 后端服务入口。
// 职责：只做装配（读配置 → 初始化依赖 → 注册路由 → 启动），不写业务逻辑。
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("TM_APP_PORT")
	if port == "" {
		port = "8080"
	}

	// TODO: 替换为 Gin 路由装配（见 .claude/agents/backend-standards-go.md 分层约定）
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	})

	log.Printf("tm-stock 后端启动，监听 :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
