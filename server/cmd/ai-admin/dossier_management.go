package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 专题管理为同一程序的显式独立模式；不挂载反馈、L0或原来源写路由。
func startDossierManagementAdmin(ctx context.Context, cfg *config.AIAdminConfig, management *config.AIDossierManagementConfig, db *sql.DB, serve func(context.Context, *http.Server) error) error {
	if ctx == nil || cfg == nil || management == nil || db == nil || serve == nil {
		return fmt.Errorf("专题管理装配无效")
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Minute)
	auth, err := middleware.NewAIAdminAuth(cfg.Reviewers)
	if err != nil {
		return err
	}
	prepare, err := sql.Open("mysql", management.PrepareDSN)
	if err != nil {
		return fmt.Errorf("专题管理结构准备连接无效")
	}
	checkErr := repository.NewAIDossierManagementSchemaVerifier(prepare).Verify(ctx)
	closeErr := prepare.Close()
	if checkErr != nil || closeErr != nil {
		return fmt.Errorf("专题管理结构保护未通过")
	}
	repo := repository.NewAIDossierManagementRepo(db)
	if err = repo.VerifyAccess(ctx); err != nil {
		return fmt.Errorf("专题管理运行权限未通过")
	}
	svc, err := service.NewAIDossierManagementService(repo, management.Actors)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	handler.NewAIDossierManagementHandler(svc).Register(mux)
	server := &http.Server{Addr: cfg.Addr, Handler: auth(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	return serve(ctx, server)
}
