package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
)

// AIReportQAConfigProvider 提供受信任的报告问答模型/限额配置；不能从请求或报告正文加载。
type AIReportQAConfigProvider interface {
	ForReport(context.Context, qa.ReportEvidence) (qa.Config, error)
}

// buildReportPlan 只负责把“当前报告证据 + 可信配置”构造成报告追问计划；不执行模型、不写库。
func buildReportPlan(ctx context.Context, id qa.Identity, in conversation.MessageInput, evidence qa.ReportEvidence, provider AIReportQAConfigProvider, now time.Time) (*qa.Plan, error) {
	if ctx == nil || provider == nil || in.Skill != "report_qa" || in.TaskID != evidence.TaskID || !in.Valid() {
		return nil, qa.ErrInvalid
	}
	cfg, err := provider.ForReport(ctx, evidence)
	if err != nil {
		return nil, err
	}
	plan, err := qa.NewReportPlan(id, in, evidence, cfg, now)
	if err != nil {
		return nil, err
	}
	if !plan.Valid() {
		return nil, qa.ErrProtocol
	}
	return plan, nil
}
