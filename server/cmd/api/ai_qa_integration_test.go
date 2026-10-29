//go:build integration

package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
	"github.com/tangniyuqi/tm-stock/server/migrations"
)

// 仅测试装配接口；从不提供配置、批准或模型客户端。
type qaStartupPolicy struct{}

func (qaStartupPolicy) ForPolicy(context.Context, string) (qa.Config, error) {
	return qa.Config{}, qa.ErrUnavailable
}

type qaStartupModel struct{}

type qaStartupReport struct{}

func (qaStartupReport) ForQA(context.Context, int64, int64) (qa.ReportEvidence, error) {
	return qa.ReportEvidence{}, qa.ErrUnavailable
}

func (qaStartupReport) ForReport(context.Context, qa.ReportEvidence) (qa.Config, error) {
	return qa.Config{}, qa.ErrUnavailable
}

func (qaStartupModel) ForPlan(context.Context, qa.Proof) ([3]*provider.OpenAICompat, error) {
	return [3]*provider.OpenAICompat{}, qa.ErrUnavailable
}

// 仅复用可丢弃测试库；问答迁移和工厂从不自动进入生产启动。
func checkQAStartup(t *testing.T, ctx context.Context, db *sql.DB, read config.AITaskReadConfig) {
	t.Helper()
	ddl := strings.ReplaceAll(strings.ReplaceAll(migrations.QASchema, "DELIMITER $$", ""), "DELIMITER ;", "")
	ddl = strings.ReplaceAll(ddl, "$$", ";")
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		t.Fatal("问答隔离迁移失败", err)
	}
	originalPolicies, originalModels := aiQAPoliciesFactory, aiQAProvidersFactory
	originalReport := aiReportQAFactory
	defer func() {
		aiQAPoliciesFactory, aiQAProvidersFactory, aiReportQAFactory = originalPolicies, originalModels, originalReport
	}()
	for _, mode := range []string{"off", "read", "report_read", "report_error", "report_nil_evidence", "report_nil_config", "submit_without_factory", "policy_error", "missing_trigger", "cancelled", "submit_factories", "submit_model_error", "submit_nil_policy", "submit_nil_model"} {
		t.Run("qa_"+mode, func(t *testing.T) {
			aiQAPoliciesFactory, aiQAProvidersFactory = nil, nil
			aiReportQAFactory = nil
			factoryCalls := 0
			reportCalls := 0
			if strings.HasPrefix(mode, "report_") {
				aiReportQAFactory = func(context.Context, *sql.DB) (service.AIReportQAEvidenceSource, service.AIReportQAConfigProvider, error) {
					reportCalls++
					switch mode {
					case "report_error":
						return nil, nil, qa.ErrUnavailable
					case "report_nil_evidence":
						return nil, qaStartupReport{}, nil
					case "report_nil_config":
						return qaStartupReport{}, nil, nil
					default:
						return qaStartupReport{}, qaStartupReport{}, nil
					}
				}
			}
			// 只读启动不能构造任何模型；此工厂即使被调用也始终拒绝。
			if mode == "read" || strings.HasPrefix(mode, "report_") {
				aiQAProvidersFactory = func(context.Context, *sql.DB) (service.AIQAProviders, error) {
					factoryCalls++
					return nil, fmt.Errorf("仅测试拒绝装配")
				}
			}
			if mode == "policy_error" {
				aiQAPoliciesFactory = func(context.Context, *sql.DB) (service.AIQAPolicies, error) {
					return nil, fmt.Errorf("仅测试拒绝装配")
				}
			}
			if strings.HasPrefix(mode, "submit_") && mode != "submit_without_factory" {
				aiQAPoliciesFactory = func(context.Context, *sql.DB) (service.AIQAPolicies, error) {
					if mode == "submit_nil_policy" {
						return nil, nil
					}
					return qaStartupPolicy{}, nil
				}
				aiQAProvidersFactory = func(context.Context, *sql.DB) (service.AIQAProviders, error) {
					if mode == "submit_model_error" {
						return nil, qa.ErrUnavailable
					}
					if mode == "submit_nil_model" {
						return nil, nil
					}
					return qaStartupModel{}, nil
				}
			}
			if mode == "missing_trigger" {
				if _, err := db.ExecContext(ctx, "DROP TRIGGER tm_ai_qa_call_no_delete"); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := db.ExecContext(ctx, ddl); err != nil {
						t.Error("恢复隔离问答保护失败", err)
					}
				}()
			}
			callCtx := ctx
			if mode == "cancelled" {
				var stop context.CancelFunc
				callCtx, stop = context.WithCancel(ctx)
				stop()
			}
			c := config.AIQAConfig{Read: mode != "off", Submit: strings.HasPrefix(mode, "submit_")}
			mux := http.NewServeMux()
			err := registerAIQA(callCtx, mux, db, read, true, c)
			expectedError := mode != "off" && mode != "read" && mode != "report_read" && mode != "submit_factories"
			if (err != nil) != expectedError || factoryCalls != 0 || strings.HasPrefix(mode, "report_") && reportCalls != 1 {
				t.Fatal("问答启动保护或只读工厂隔离失败", err, factoryCalls)
			}
			for _, methodPath := range [][2]string{{"GET", "/api/v1/ai/conversations/1/question-receipt"}, {"GET", "/api/v1/ai/conversations/1/questions/1/answer"}, {"POST", "/api/v1/ai/conversations/1/questions"}} {
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, httptest.NewRequest(methodPath[0], methodPath[1], nil))
				want := 404
				if mode == "submit_factories" || (mode == "read" || mode == "report_read") && methodPath[0] == "GET" {
					want = 401
				}
				if w.Code != want {
					t.Fatal("问答启动错误地开放路由", mode, methodPath, w.Code)
				}
			}
		})
	}
	for _, table := range []string{"tm_ai_conversations", "tm_ai_conversation_messages", "tm_ai_qa_requests", "tm_ai_qa_calls", "tm_ai_qa_quotas", "tm_ai_qa_scopes", "tm_ai_qa_limits"} {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil || n != 0 {
			t.Fatal("问答启动探针未全部回滚", table, n, err)
		}
	}
}
