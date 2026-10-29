package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/handler"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/middleware"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// prepareAIReportReads 只装配读取，不开启终检写入。缺真实发布证明就不准启动。
func prepareAIReportReads(ctx context.Context, db *sql.DB, read config.AITaskReadConfig, enabled bool, p *finalreport.Policy, control service.AIReportReadControl) (*service.AIReportReadService, error) {
	return prepareAIReportReadsWithDeep(ctx, db, read, enabled, p, nil, control)
}

// 深度策略由可信启动装配显式提供；不由会员参数或深度创建开关隐式启用。
func prepareAIReportReadsWithDeep(ctx context.Context, db *sql.DB, read config.AITaskReadConfig, enabled bool, p, deepPolicy *finalreport.Policy, control service.AIReportReadControl) (*service.AIReportReadService, error) {
	if !enabled {
		return nil, nil
	}
	if !read.Enabled || read.Auth.Validate() != nil || !p.Valid() || control == nil {
		return nil, memberauth.ErrInvalidConfig
	}
	if deepPolicy != nil && (!deepPolicy.Valid() || deepPolicy.AnalysisType() != task.DeepAnalysisType || p.AnalysisType() == task.DeepAnalysisType) {
		return nil, memberauth.ErrInvalidConfig
	}
	if db == nil || ctx == nil {
		return nil, points.ErrInvalid
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	verifier := repository.NewAIReportSchemaVerifier(db)
	var err error
	if deepPolicy != nil || p.AnalysisType() == task.DeepAnalysisType {
		err = verifier.VerifyWithDeep(checkCtx)
	} else {
		err = verifier.Verify(checkCtx)
	}
	checkErr := checkCtx.Err()
	cancel()
	if err != nil || checkErr != nil {
		return nil, fmt.Errorf("会员报告读取启动自检失败（驱动详情已隐藏）: %w", points.ErrInvariant)
	}
	ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		return nil, err
	}
	base, err := service.NewAIReportService(repository.NewAITaskRepo(db), ledger, p, false)
	if err != nil {
		return nil, err
	}
	if deepPolicy == nil {
		return service.NewAIReportReadService(base, control, true)
	}
	deep, err := service.NewAIReportService(repository.NewAITaskRepo(db), ledger, deepPolicy, false)
	if err != nil {
		return nil, err
	}
	return service.NewAIReportReadServiceWithDeep(base, deep, control, true)
}
func registerAIReportReads(mux *http.ServeMux, db *sql.DB, cfg config.AITaskReadConfig, reads *service.AIReportReadService) error {
	if reads == nil {
		return nil
	}
	if !cfg.Enabled {
		return memberauth.ErrInvalidConfig
	}
	v, err := memberauth.NewVerifier(cfg.Auth)
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
	handler.NewAIReportReadHandler(reads).Register(mux, auth)
	return nil
}
