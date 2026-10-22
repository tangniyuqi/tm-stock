package service

import (
	"context"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
)

// AIReportQAEvidenceSource 每次按会员与任务重建报告证据；实现必须重新读取当前可读报告。
type AIReportQAEvidenceSource interface {
	ForQA(context.Context, int64, int64) (qa.ReportEvidence, error)
}

// AIReportQAAccess 把报告证据装配为问答控制的报告范围授权；摘要不符或不可读一律失败关闭。
type AIReportQAAccess struct {
	source AIReportQAEvidenceSource
	now    func() time.Time
}

func NewAIReportQAAccess(source AIReportQAEvidenceSource) (*AIReportQAAccess, error) {
	if source == nil {
		return nil, qa.ErrInvalid
	}
	return &AIReportQAAccess{source: source, now: time.Now}, nil
}

func (a *AIReportQAAccess) Scope(ctx context.Context, proof qa.Proof) (qa.Scope, error) {
	if a == nil || a.source == nil || ctx == nil || proof.Identity.UserID <= 0 {
		return qa.Scope{}, qa.ErrInvalid
	}
	taskID, digest, ok := qa.ParseReportApproval(proof.ScopeApproval)
	if !ok {
		return qa.Scope{}, qa.ErrInvalid
	}
	evidence, err := a.source.ForQA(ctx, proof.Identity.UserID, taskID)
	if err != nil {
		return qa.Scope{}, err
	}
	if evidence.TaskID != taskID || evidence.Digest != digest {
		return qa.Scope{}, qa.ErrUnavailable
	}
	scope, err := qa.BuildReportScopeUntil(evidence, a.now().UTC(), proof.ValidUntil)
	if err != nil {
		return qa.Scope{}, err
	}
	if scope.ApprovalRef != proof.ScopeApproval {
		return qa.Scope{}, qa.ErrUnavailable
	}
	return scope, nil
}
