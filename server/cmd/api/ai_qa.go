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

// 生产工厂仍为空；显式本地装配另行传入，不改全局工厂或默认提交权限。
var aiQAPoliciesFactory func(context.Context, *sql.DB) (service.AIQAPolicies, error)
var aiQAProvidersFactory func(context.Context, *sql.DB) (service.AIQAProviders, error)
var aiReportQAFactory func(context.Context, *sql.DB) (service.AIReportQAEvidenceSource, service.AIReportQAConfigProvider, error)

func registerAIQA(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, conversations bool, c config.AIQAConfig) error {
	return registerAIQAWithLocal(ctx, mux, db, read, conversations, c, nil, nil)
}

func registerAIQAWithLocal(ctx context.Context, mux *http.ServeMux, db *sql.DB, read config.AITaskReadConfig, conversations bool, c config.AIQAConfig, local *service.AILocalQAControl, reports *service.AIReportReadService) error {
	if !c.Read && !c.Submit && !c.ReportSubmit {
		return nil
	}
	if ctx == nil || mux == nil || db == nil || !conversations || !c.Read || !read.Enabled || read.Auth.Validate() != nil || c.ReportSubmit && !c.Submit || c.Submit && local == nil && (aiQAPoliciesFactory == nil || aiQAProvidersFactory == nil) || c.ReportSubmit && local != nil && reports == nil {
		return memberauth.ErrInvalidConfig
	}
	verifier := repository.NewAIQASchemaVerifier(db)
	if local != nil {
		id, err := local.InstanceID(ctx)
		if err != nil {
			return memberauth.ErrInvalidConfig
		}
		verifier = repository.NewAILocalQASchemaVerifier(db, id)
	}
	if err := verifier.Verify(ctx); err != nil {
		return fmt.Errorf("问答保护自检失败（内部详情已隐藏）")
	}
	var policies service.AIQAPolicies
	var models service.AIQAProviders
	var err error
	if local != nil {
		if err := local.Ready(ctx); err != nil {
			return fmt.Errorf("本地问答装配未确认（内部详情已隐藏）")
		}
		policies = local
	} else if aiQAPoliciesFactory != nil {
		policies, err = aiQAPoliciesFactory(ctx, db)
		if err != nil {
			return fmt.Errorf("问答策略装配未确认（内部详情已隐藏）")
		}
	}
	if c.Submit {
		if local != nil {
			models = local
		} else {
			models, err = aiQAProvidersFactory(ctx, db)
		}
		if err != nil {
			return fmt.Errorf("问答模型装配未确认（内部详情已隐藏）")
		}
	}
	var reportEvidence service.AIReportQAEvidenceSource
	var reportConfig service.AIReportQAConfigProvider
	// 可信报告工厂只提供证据与无密钥配置；只读启动也可装配，不能由此放开提交。
	if local != nil && reports != nil {
		reportEvidence, err = service.NewAIReportQAEvidence(reports)
		if err != nil {
			return memberauth.ErrInvalidConfig
		}
		reportConfig = local
	} else if aiReportQAFactory != nil || c.ReportSubmit {
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
	if reportEvidence != nil {
		svc, err = service.NewAIQAServiceWithReportAccess(repository.NewAIQARepo(db), policies, models, sessions, c.Submit, c.ReportSubmit, reportEvidence, reportConfig)
	} else {
		svc, err = service.NewAIQAService(repository.NewAIQARepo(db), policies, models, sessions, c.Submit)
	}
	if err != nil {
		return err
	}
	handler.NewAIQAHandler(svc).Register(mux, auth, c.Submit)
	return nil
}
