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

// 生产工厂仍为空；读取元数据不创建模型客户端，启用提交须另行可信准入装配。
var aiQAPoliciesFactory func(context.Context, *sql.DB) (service.AIQAPolicies, error)
var aiQAProvidersFactory func(context.Context, *sql.DB) (service.AIQAProviders, error)
var aiReportQAFactory func(context.Context, *sql.DB) (service.AIReportQAEvidenceSource, service.AIReportQAConfigProvider, error)

func registerAIQA(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, conversations bool, c config.AIQAConfig) error {
	if !c.Read && !c.Submit {
		return nil
	}
	if ctx == nil || mux == nil || db == nil || !conversations || !c.Read || !read.Enabled || read.Auth.Validate() != nil || c.Submit && (aiQAPoliciesFactory == nil || aiQAProvidersFactory == nil) {
		return memberauth.ErrInvalidConfig
	}
	if err := repository.NewAIQASchemaVerifier(db).Verify(ctx); err != nil {
		return fmt.Errorf("问答保护自检失败（内部详情已隐藏）")
	}
	var policies service.AIQAPolicies
	var models service.AIQAProviders
	var err error
	if aiQAPoliciesFactory != nil {
		policies, err = aiQAPoliciesFactory(ctx, db)
		if err != nil {
			return fmt.Errorf("问答策略装配未确认（内部详情已隐藏）")
		}
	}
	if c.Submit {
		models, err = aiQAProvidersFactory(ctx, db)
		if err != nil {
			return fmt.Errorf("问答模型装配未确认（内部详情已隐藏）")
		}
	}
	var reportEvidence service.AIReportQAEvidenceSource
	var reportConfig service.AIReportQAConfigProvider
	if c.ReportSubmit {
		if aiReportQAFactory == nil {
			return memberauth.ErrInvalidConfig
		}
		reportEvidence, reportConfig, err = aiReportQAFactory(ctx, db)
		if err != nil {
			return fmt.Errorf("报告追问装配未确认（内部详情已隐藏）")
		}
		if reportEvidence == nil || reportConfig == nil {
			return memberauth.ErrInvalidConfig
		}
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
	var svc *service.AIQAService
	if c.ReportSubmit {
		svc, err = service.NewAIQAServiceWithReports(repository.NewAIQARepo(db), policies, models, sessions, c.Submit, reportEvidence, reportConfig)
	} else {
		svc, err = service.NewAIQAService(repository.NewAIQARepo(db), policies, models, sessions, c.Submit)
	}
	if err != nil {
		return err
	}
	handler.NewAIQAHandler(svc).Register(mux, auth, c.Submit)
	return nil
}
