package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func registerAITaskCancels(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, enabled bool) error {
	if !enabled {
		return nil
	}
	if !read.Enabled {
		return memberauth.ErrInvalidConfig
	}
	v, err := memberauth.NewVerifier(read.Auth)
	if err != nil {
		return err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	err = repository.NewAITaskSchemaVerifier(db).Verify(checkCtx)
	checkErr := checkCtx.Err()
	cancel()
	if err != nil || checkErr != nil {
		// 启动日志也不输出数据库保护名、驱动或连接详情。
		return fmt.Errorf("AI 任务取消启动自检失败（驱动详情已隐藏）: %w", points.ErrInvariant)
	}
	sessions, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(db))
	if err != nil {
		return err
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		return err
	}
	ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		return err
	}
	tasks, err := service.NewAITaskService(repository.NewAITaskRepo(db), ledger)
	if err != nil {
		return err
	}
	svc, err := service.NewAITaskCancelService(tasks)
	if err != nil {
		return err
	}
	handler.NewAITaskCancelHandler(svc).Register(mux, auth)
	return nil
}
