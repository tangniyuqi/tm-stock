package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func registerAITaskStreams(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, enabled bool) error {
	if !enabled {
		return nil
	}
	if ctx == nil || mux == nil || db == nil || !read.Enabled || read.Auth.Validate() != nil {
		return memberauth.ErrInvalidConfig
	}
	if err := repository.VerifyAITaskStreamReadSchema(ctx, db); err != nil {
		return fmt.Errorf("任务进度只读结构检查失败（驱动详情已隐藏）")
	}
	verifier, err := memberauth.NewVerifier(read.Auth)
	if err != nil {
		return err
	}
	sessions, err := service.NewMemberSessionService(verifier, repository.NewMemberIdentityRepo(db))
	if err != nil {
		return err
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		return err
	}
	reads, err := service.NewAITaskReadService(repository.NewAITaskRepo(db))
	if err != nil {
		return err
	}
	streams, err := service.NewAITaskStreamService(reads, sessions)
	if err != nil {
		return err
	}
	handler.NewAITaskStreamHandler(streams).Register(mux, auth)
	return nil
}
