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

// 本地模式只接反馈；不能借用来源审核或模型抽取权限。
func validateLocalFeedbackAdmin(cfg *config.AIAdminConfig, l0 *config.AIL0Config, local *config.AILocalRuntime) error {
	if cfg == nil || l0 == nil || local == nil || l0.Enabled || cfg.Addr != local.Listen || local.CheckDSN(cfg.DSN) != nil || config.ValidateAIAdminReviewers(cfg.Reviewers) != nil {
		return fmt.Errorf("本地反馈管理配置或隔离边界无效（详情已隐藏）")
	}
	return nil
}

func startLocalFeedbackAdmin(parent context.Context, cfg *config.AIAdminConfig, l0 *config.AIL0Config, db *sql.DB, serve func(context.Context, *http.Server) error, local *config.AILocalRuntime) error {
	if parent == nil || db == nil || serve == nil {
		return fmt.Errorf("本地反馈管理依赖不完整")
	}
	if err := validateLocalFeedbackAdmin(cfg, l0, local); err != nil {
		return err
	}
	enabled, err := config.LoadAIFeedbackAdmin()
	if err != nil || !enabled {
		return fmt.Errorf("本地反馈管理须显式开启原反馈开关")
	}
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	if err := repository.NewAILocalInstanceVerifier(db).Verify(ctx, local.InstanceID); err != nil {
		return fmt.Errorf("本地反馈管理隔离实例未确认（详情已隐藏）")
	}
	if err := repository.NewAIFeedbackSchemaVerifier(db).Verify(ctx); err != nil {
		return fmt.Errorf("本地反馈管理结构自检未通过（详情已隐藏）")
	}
	auth, err := middleware.NewAIAdminAuth(cfg.Reviewers)
	if err != nil {
		return err
	}
	svc, err := service.NewAIFeedbackService(repository.NewAIFeedbackRepo(db))
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	handler.NewAIFeedbackHandler(svc).RegisterInternal(mux)
	// 仅在上述保护全部通过后才监听；鉴权仍由原中间件绑定操作人。
	srv := &http.Server{Addr: local.Listen, Handler: auth(mux), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 90 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	return serve(parent, srv)
}
