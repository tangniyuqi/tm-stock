// Package main 是 tm-stock C 端 API 服务入口。
//
// 职责：只做装配（读配置 → 初始化依赖 → 注册路由 → 启动），不写业务逻辑。
// 依赖方向：main → handler → service → repository（单向）。
//
// 只用标准库 net/http：Go 1.22+ 的 ServeMux 支持方法 + 路径参数模式，
// 本服务端点很少，没必要为此引入 Web 框架（也省下十几个传递依赖）。
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql" // 注册驱动；上层各包都不直接依赖它

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log.Printf("配置: %s", cfg.Redacted()) // Redacted 刻意不含 DSN

	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour) // 避免被 MySQL 的 wait_timeout 掐掉连接

	// 启动时就验证连通性。等第一个请求进来才发现连不上，
	// 只会得到一个 500 和一堆猜测。
	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return err
	}
	log.Println("数据库连接正常")

	repo := repository.NewThemeRepo(db)

	// 行情：一期 Enabled=false（ADR-0006）。QuoteProvider 传 nil，
	// service 在关闭时不会调用它，涨跌幅一律返回 nil，前端显示「—」。
	themeSvc := service.NewThemeService(repo, nil, service.QuoteConfig{
		Enabled:  cfg.QuoteEnabled,
		DelayMin: cfg.QuoteDelayMin,
		IsMock:   cfg.QuoteMock,
	})

	// 🔴 会员与试吃尚未实现（member-center 未开发）。
	// 默认 fail-closed：不给付费内容。开发放行须显式设 TM_DEV_GRANT_PAID=true。
	var access handler.AccessResolver = handler.DenyAllAccess{}
	if cfg.DevGrantPaid {
		access = handler.NewDevGrantAllAccess()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	})
	handler.NewThemeHandler(themeSvc, access).Register(mux)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // 防慢速头攻击
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 优雅退出：容器里 SIGTERM 是常态，直接被杀会掐断处理中的请求。
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("tm-stock API 启动，监听 :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		log.Printf("收到信号 %v，开始优雅退出", sig)
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutCancel()
		return srv.Shutdown(shutCtx)
	}
}
