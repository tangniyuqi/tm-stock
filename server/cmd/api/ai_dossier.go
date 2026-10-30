package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 仅隔离本地显式启用。结构准备连接独立且及时关闭，不赋予读取连接探针写权限。
func registerAILocalDossiers(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, runtime *config.AILocalRuntime, readerDSN string) error {
	raw := os.Getenv("TM_AI_LOCAL_DOSSIER_READ_ENABLED")
	var enabled bool
	var err error
	if raw != "" {
		enabled, err = strconv.ParseBool(raw)
		if err != nil {
			return memberauth.ErrInvalidConfig
		}
	}
	referencesEnabled, err := config.LoadAILocalConversationDossiers(enabled, read)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	if ctx == nil || mux == nil || db == nil || runtime == nil || !read.Enabled || read.Auth.Validate() != nil || runtime.CheckDSN(readerDSN) != nil {
		return memberauth.ErrInvalidConfig
	}
	prepDSN := os.Getenv("TM_AI_LOCAL_DOSSIER_PREPARE_DSN")
	if runtime.CheckDSN(prepDSN) != nil {
		return memberauth.ErrInvalidConfig
	}
	rc, err := mysql.ParseDSN(readerDSN)
	if err != nil {
		return memberauth.ErrInvalidConfig
	}
	pc, err := mysql.ParseDSN(prepDSN)
	if err != nil || rc.Addr != pc.Addr || rc.DBName != pc.DBName || rc.User == pc.User {
		return memberauth.ErrInvalidConfig
	}
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	prepare, err := sql.Open("mysql", prepDSN)
	if err != nil {
		return memberauth.ErrUnavailable
	}
	prepare.SetMaxOpenConns(1)
	defer prepare.Close()
	if repository.NewAILocalInstanceVerifier(prepare).Verify(checkCtx, runtime.InstanceID) != nil || repository.NewAILocalInstanceVerifier(db).Verify(checkCtx, runtime.InstanceID) != nil {
		return memberauth.ErrInvalidConfig
	}
	if repository.NewAIDossierSchemaVerifier(prepare).Verify(checkCtx) != nil {
		return memberauth.ErrUnavailable
	}
	if referencesEnabled && repository.NewAIConversationDossierSchemaVerifier(prepare).Verify(checkCtx) != nil {
		return memberauth.ErrUnavailable
	}
	if err = prepare.Close(); err != nil {
		return memberauth.ErrUnavailable
	}
	repo := repository.NewAIDossierRepo(db, sql.LevelReadCommitted)
	if repo.VerifyReadAccess(checkCtx) != nil {
		return memberauth.ErrUnavailable
	}
	references := repository.NewAIConversationDossierRepo(db, sql.LevelReadCommitted)
	if referencesEnabled && references.VerifyAccess(checkCtx) != nil {
		return memberauth.ErrUnavailable
	}
	verifier, err := memberauth.NewVerifier(read.Auth)
	if err != nil {
		return memberauth.ErrInvalidConfig
	}
	sessions, err := service.NewMemberSessionService(verifier, repository.NewMemberIdentityRepo(db))
	if err != nil {
		return err
	}
	auth, err := middleware.NewMemberAuth(sessions)
	if err != nil {
		return err
	}
	svc, err := service.NewAIDossierService(repo, sessions, false)
	if err != nil {
		return err
	}
	if err = checkCtx.Err(); err != nil {
		return memberauth.ErrUnavailable
	}
	var saved *service.AIConversationDossierService
	if referencesEnabled {
		saved, err = service.NewAIConversationDossierService(references, sessions)
		if err != nil {
			return err
		}
	}
	handler.NewAIDossierHandler(svc).Register(mux, auth)
	if saved != nil {
		handler.NewAIConversationDossierHandler(saved).Register(mux, auth)
	}
	return nil
}
