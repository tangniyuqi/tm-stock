// 内部 AI 审核服务独立进程；不注册公开 AI 路由，不自动运行迁移。
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/pipeline"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.LoadAIAdmin()
	if err != nil {
		return err
	}
	l0Config, err := config.LoadAIL0()
	if err != nil {
		return err
	}
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return fmt.Errorf("内部审核数据库配置无效")
	}
	// 关闭失败不影响已完成的事务；不把驱动错误或 DSN 打入日志。
	defer func() { _ = db.Close() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return startAdmin(ctx, cfg, l0Config, db, serveAdmin)
}

// startAdmin 接受已打开的数据库，便于用隔离测试驱动验证启动与路由装配。
func startAdmin(ctx context.Context, cfg *config.AIAdminConfig, l0Config *config.AIL0Config, db *sql.DB, serve func(context.Context, *http.Server) error) error {
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err := db.PingContext(pingCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("内部审核数据库不可用")
	}
	auth, err := middleware.NewAIAdminAuth(cfg.Reviewers)
	if err != nil {
		return err
	}
	svc, err := service.NewAIReviewService(repository.NewAIReviewRepo(db))
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	handler.NewAIReviewHandler(svc).Register(mux)
	inspection, err := service.NewAIInspectionService(repository.NewAIInspectionRepo(db))
	if err != nil {
		return err
	}
	handler.NewAIInspectionHandler(inspection).Register(mux)
	sourceRepo := repository.NewAISourceRepo(db)
	startupCtx, startupCancel := context.WithTimeout(ctx, 10*time.Second)
	defer startupCancel()
	sourceService, err := sources.NewService(startupCtx, sourceRepo, sources.NewFetcher(), l0Config.Enabled)
	if err != nil {
		return fmt.Errorf("内部来源启动校验失败")
	}
	library, err := service.NewAILibraryService(sourceService, sourceRepo)
	if err != nil {
		return err
	}
	runStore := repository.NewAIL0RunRepo(db)
	runner, err := buildL0Runner(*l0Config, runStore, repository.NewAIExtractionCatalog(db))
	if err != nil {
		return err
	}
	// Go 的带类型 nil 指针转接口后不为 nil；关闭时必须保留真正的空接口。
	var l0Service *service.AIL0Service
	if runner == nil {
		l0Service, err = service.NewAIL0Service(runStore, sourceRepo, nil, pipeline.RunPolicy{})
	} else {
		l0Service, err = service.NewAIL0Service(runStore, sourceRepo, runner, pipeline.RunPolicy{Provider: l0Config.Provider.Name, Model: l0Config.Provider.Model, ApprovalRef: l0Config.ApprovalRef})
	}
	if err != nil {
		return err
	}
	handler.NewAILibraryHandler(library, l0Service).Register(mux)
	srv := &http.Server{Addr: cfg.Addr, Handler: auth(mux), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 90 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	return serve(ctx, srv)
}

func serveAdmin(ctx context.Context, srv *http.Server) error {
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	log.Printf("内部 AI 审核监听 %s，公开 AI 功能保持关闭", srv.Addr)
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("内部审核监听失败")
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("内部审核未能优雅退出")
		}
		return nil
	}
}
