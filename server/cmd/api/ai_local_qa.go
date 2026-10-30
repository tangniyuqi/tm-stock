package main

import (
	"context"
	"database/sql"
	"os"

	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

// 本地问答独立文件不能代替原资料范围表、预算配额或报告发布批准。
func prepareAILocalQA(ctx context.Context, db *sql.DB, runtime *config.AILocalRuntime, c config.AIQAConfig) (*service.AILocalQAControl, error) {
	path := os.Getenv("TM_AI_LOCAL_QA")
	if path == "" {
		return nil, nil
	}
	if ctx == nil || db == nil || runtime == nil || !c.Read {
		return nil, memberauth.ErrInvalidConfig
	}
	source, err := config.NewAILocalQASource(path, runtime)
	if err != nil {
		return nil, err
	}
	control, err := service.NewAILocalQAControl(source, repository.NewAILocalInstanceVerifier(db), c.Submit)
	if err != nil {
		return nil, err
	}
	if err := control.Ready(ctx); err != nil {
		return nil, memberauth.ErrInvalidConfig
	}
	return control, nil
}
