package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

func prepareAITaskCreates(ctx context.Context, db *sql.DB, read config.AITaskReadConfig, enabled bool, control service.AITaskAdmissionControl) (*service.AITaskCreateService, error) {
	return prepareAITaskCreatesWithDeep(ctx, db, read, enabled, false, control)
}

func prepareAITaskCreatesWithDeep(ctx context.Context, db *sql.DB, read config.AITaskReadConfig, enabled, deep bool, control service.AITaskAdmissionControl) (*service.AITaskCreateService, error) {
	if deep && !enabled {
		return nil, memberauth.ErrInvalidConfig
	}
	if !enabled && !read.Enabled {
		return nil, nil
	}
	if !read.Enabled || read.Auth.Validate() != nil || enabled && (control == nil || db == nil || ctx == nil) {
		return nil, memberauth.ErrInvalidConfig
	}
	if enabled {
		checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := checkCtx.Err(); err != nil {
			return nil, err
		}
		if err := control.Ready(checkCtx); err != nil {
			return nil, fmt.Errorf("AI创建准入启动检查失败（详情已隐藏）: %w", task.ErrAdmissionUnavailable)
		}
		if err := checkCtx.Err(); err != nil {
			return nil, err
		}
		if err := repository.NewAIAdmissionSchemaVerifier(db).Verify(checkCtx); err != nil {
			return nil, fmt.Errorf("AI创建结构自检失败（详情已隐藏）: %w", points.ErrInvariant)
		}
		if err := checkCtx.Err(); err != nil {
			return nil, err
		}
	}
	ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		return nil, err
	}
	tasks, err := service.NewAITaskService(repository.NewAITaskRepo(db), ledger)
	if err != nil {
		return nil, err
	}
	svc, err := service.NewAITaskCreateService(tasks, control, enabled)
	if err != nil {
		return nil, err
	}
	if deep {
		if !enabled || control == nil {
			return nil, memberauth.ErrInvalidConfig
		}
		d, ok := control.(service.AITaskDeepAdmission)
		if !ok {
			return nil, memberauth.ErrInvalidConfig
		}
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := d.ReadyDeep(checkCtx); err != nil {
			return nil, fmt.Errorf("AI深度准入启动检查失败（详情已隐藏）: %w", task.ErrAdmissionUnavailable)
		}
		if err := repository.NewAIReportSchemaVerifier(db).VerifyWithDeep(checkCtx); err != nil {
			return nil, fmt.Errorf("AI深度报告结构自检失败（详情已隐藏）: %w", points.ErrInvariant)
		}
		limiter, err := service.NewSQLDeepLimiter(db)
		if err != nil {
			return nil, err
		}
		svc.SetDeepLimiter(limiter)
	}
	return svc, nil
}

func registerAITaskCreates(mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, svc *service.AITaskCreateService, enabled bool) error {
	if svc == nil {
		if enabled {
			return memberauth.ErrInvalidConfig
		}
		return nil
	}
	if !read.Enabled {
		return memberauth.ErrInvalidConfig
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
	handler.NewAITaskCreateHandler(svc).Register(mux, auth, enabled)
	return nil
}
