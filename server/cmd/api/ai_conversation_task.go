package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func registerAIConversationTasks(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, creator *service.AITaskCreateService, enabled, writes bool) error {
	return registerAIConversationTasksWithLocal(ctx, mux, db, read, creator, enabled, writes, nil)
}
func registerAIConversationTasksWithLocal(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, creator *service.AITaskCreateService, enabled, writes bool, local *config.AILocalRuntime) error {
	if !enabled {
		return nil
	}
	if ctx == nil || mux == nil || db == nil || creator == nil || !read.Enabled || read.Auth.Validate() != nil {
		return memberauth.ErrInvalidConfig
	}
	verifier := repository.NewAIConversationTaskSchemaVerifier(db)
	if local != nil {
		verifier = repository.NewAILocalConversationTaskSchemaVerifier(db, local.InstanceID)
	}
	if err := verifier.Verify(ctx); err != nil {
		var stage *repository.AIConversationSchemaError
		if errors.As(err, &stage) {
			return fmt.Errorf("会话任务保护自检失败：阶段%d（内部详情已隐藏）", stage.Stage)
		}
		return fmt.Errorf("会话任务保护自检失败（内部详情已隐藏）")
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
	svc, err := service.NewAIConversationTaskService(repository.NewAIConversationRepo(db), creator, writes)
	if err != nil {
		return err
	}
	handler.NewAIConversationTaskHandler(svc).Register(mux, auth, writes)
	return nil
}
