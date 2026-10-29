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
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func registerPointsBalance(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, enabled bool) error {
	if !enabled {
		return nil
	}
	if ctx == nil || mux == nil || db == nil || !read.Enabled {
		return memberauth.ErrInvalidConfig
	}
	v, err := memberauth.NewVerifier(read.Auth)
	if err != nil {
		return err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := repository.NewPointsSchemaVerifier(db).Verify(checkCtx); err != nil {
		return fmt.Errorf("积分余额启动自检失败（内部详情已隐藏）")
	}
	sessions, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(db))
	if err != nil {
		return err
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		return err
	}
	svc, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		return err
	}
	handler.NewPointsBalanceHandler(svc).Register(mux, auth)
	return nil
}
