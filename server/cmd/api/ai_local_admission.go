package main

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 没有本地文件时保留原无控制器行为；本地文件不能被非隔离启动路径借用。
func prepareAILocalAdmission(db *sql.DB, runtime *config.AILocalRuntime) (service.AITaskAdmissionControl, error) {
	path := os.Getenv("TM_AI_LOCAL_PLANS")
	if path == "" {
		return nil, nil
	}
	if db == nil || runtime == nil {
		return nil, memberauth.ErrInvalidConfig
	}
	source, err := config.NewAILocalPlanSource(path, runtime)
	if err != nil {
		return nil, err
	}
	return service.NewAILocalAdmissionControl(source, repository.NewAILocalInstanceVerifier(db), repository.NewAITaskRepo(db))
}

// 同一计划来源解析本人任务的轻/深策略，不因读取开关开启创建或模型。
func prepareAILocalReportReads(parent context.Context, db *sql.DB, read config.AITaskReadConfig, enabled bool, admission service.AITaskAdmissionControl) (*service.AIReportReadService, error) {
	if !enabled {
		return nil, nil
	}
	local, ok := admission.(*service.AILocalAdmissionControl)
	if !ok || local == nil || parent == nil || db == nil || !read.Enabled || read.Auth.Validate() != nil {
		return nil, memberauth.ErrInvalidConfig
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	if err := local.Ready(ctx); err != nil {
		return nil, err
	}
	if err := repository.NewAIReportSchemaVerifier(db).VerifyWithDeep(ctx); err != nil {
		return nil, memberauth.ErrInvalidConfig
	}
	ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		return nil, err
	}
	base, err := service.NewAIReportService(repository.NewAITaskRepo(db), ledger, nil, false)
	if err != nil {
		return nil, err
	}
	control, err := service.NewAILocalReportReadControl(local)
	if err != nil {
		return nil, err
	}
	return service.NewAIReportReadServiceWithPolicySource(base, control, control, true)
}
