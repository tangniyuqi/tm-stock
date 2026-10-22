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

func registerAIConversations(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, enabled bool) error {
	if !enabled {
		return nil
	}
	if ctx == nil || mux == nil || db == nil || !read.Enabled || read.Auth.Validate() != nil {
		return memberauth.ErrInvalidConfig
	}
	if err := repository.NewAIConversationSchemaVerifier(db).Verify(ctx); err != nil {
		return fmt.Errorf("会话保护自检失败（内部详情已隐藏）")
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
	svc, err := service.NewAIConversationService(repository.NewAIConversationRepo(db))
	if err != nil {
		return err
	}
	handler.NewAIConversationHandler(svc).Register(mux, auth)
	return nil
}
