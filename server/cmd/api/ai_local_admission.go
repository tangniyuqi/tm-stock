package main

import (
	"database/sql"
	"os"

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
