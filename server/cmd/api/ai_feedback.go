package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"net/http"
	"time"
)

func registerAIFeedback(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, enabled bool) error {
	if !enabled {
		return nil
	}
	if !read.Enabled || read.Auth.Validate() != nil || db == nil {
		return memberauth.ErrInvalidConfig
	}
	check, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := repository.NewAIFeedbackSchemaVerifier(db).Verify(check); err != nil {
		return fmt.Errorf("反馈结构自检失败（驱动详情已隐藏）")
	}
	v, err := memberauth.NewVerifier(read.Auth)
	if err != nil {
		return err
	}
	sessions, err := service.NewMemberSessionService(v, repository.NewMemberIdentityRepo(db))
	if err != nil {
		return err
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		return err
	}
	svc, err := service.NewAIFeedbackService(repository.NewAIFeedbackRepo(db))
	if err != nil {
		return err
	}
	handler.NewAIFeedbackHandler(svc).RegisterMember(mux, auth)
	return nil
}
