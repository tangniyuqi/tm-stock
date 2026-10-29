package service

import (
	"context"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 仅供同一本地合成计划下已终检、已结算的本人报告；不调用模型或写账。
// 不替代原读取器对检查点、报告原文及结算明细的完整核验。
type AILocalReportReadControl struct{ admission *AILocalAdmissionControl }

func NewAILocalReportReadControl(admission *AILocalAdmissionControl) (*AILocalReportReadControl, error) {
	if admission == nil || admission.source == nil || admission.evidence == nil || admission.identity == nil {
		return nil, points.ErrInvalid
	}
	return &AILocalReportReadControl{admission}, nil
}

type localPublicationProof struct {
	task        task.Task
	plan        task.AdmissionPlan
	publication AIReportPublication
}

func (c *AILocalReportReadControl) load(ctx context.Context, uid, id int64) (localPublicationProof, error) {
	var out localPublicationProof
	if c == nil || c.admission == nil || ctx == nil || uid <= 0 || id <= 0 {
		return out, points.ErrInvalid
	}
	err := c.admission.evidence.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		v, err := lockOwnedTask(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		if v.Status != task.Succeeded {
			return finalreport.ErrUnavailable
		}
		r, found, err := reportAdmission(ctx, tx, v)
		if err != nil {
			return err
		}
		if !found {
			return finalreport.ErrUnavailable
		}
		commit, err := tx.ReportCommit(ctx, uid, id)
		if err != nil {
			return err
		}
		if commit.Validate() != nil || commit.Status != task.Succeeded || commit.TaskID != id || commit.UserID != uid || commit.TaskNo != v.No || commit.Fence != v.Fence || commit.StepNo != v.CurrentStep || commit.EventSeq != v.EventSeq || commit.CreatedAt.Before(r.CreatedAt) {
			return points.ErrInvariant
		}
		out = localPublicationProof{task: v, plan: r.Plan.Clone(), publication: AIReportPublication{UserID: uid, TaskID: id, SubjectID: v.SubjectID, GeneratedAt: commit.CreatedAt.UnixMilli(), ReportDigest: commit.ContentDigest, PolicyDigest: commit.PolicyDigest, ModelName: r.Plan.ModelName, FilingNo: r.Plan.FilingNo}}
		return nil
	})
	return out, err
}

func localReadDenial(err error) error {
	if errors.Is(err, task.ErrAdmissionUnavailable) || errors.Is(err, points.ErrIdempotency) {
		return finalreport.ErrUnavailable
	}
	return err
}

func (c *AILocalReportReadControl) policy(ctx context.Context, p localPublicationProof) (*finalreport.Policy, error) {
	// 在任务事务结束后确认配置及资料，不能在任务/账户锁内等待规划器。
	if err := c.admission.Recheck(ctx, p.plan); err != nil {
		return nil, localReadDenial(err)
	}
	execution, err := c.admission.ExecutionPlan(ctx, p.task)
	if err != nil {
		return nil, localReadDenial(err)
	}
	digest, err := execution.AdmissionDigest()
	if err != nil {
		return nil, err
	}
	if digest != p.plan.ExecutionDigest {
		return nil, finalreport.ErrUnavailable
	}
	policy, err := pendingPolicy(execution.Policy)
	if err != nil {
		return nil, err
	}
	if policy.Digest() != p.publication.PolicyDigest || execution.Model.Model != p.publication.ModelName || execution.Model.FilingNo != p.publication.FilingNo {
		return nil, finalreport.ErrUnavailable
	}
	return policy, nil
}

func (c *AILocalReportReadControl) Resolve(parent context.Context, uid, id int64) (AIReportReadPolicy, error) {
	if parent == nil {
		return AIReportReadPolicy{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	p, err := c.load(ctx, uid, id)
	if err != nil {
		return AIReportReadPolicy{}, err
	}
	policy, err := c.policy(ctx, p)
	if err != nil {
		return AIReportReadPolicy{}, err
	}
	if err := ctx.Err(); err != nil {
		return AIReportReadPolicy{}, err
	}
	return AIReportReadPolicy{UserID: uid, TaskID: id, SubjectID: p.task.SubjectID, AnalysisType: p.task.AnalysisType, Policy: policy}, nil
}

func (c *AILocalReportReadControl) Recheck(parent context.Context, p AIReportPublication) error {
	if parent == nil {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	proof, err := c.load(ctx, p.UserID, p.TaskID)
	if err != nil {
		return err
	}
	if proof.publication != p {
		return finalreport.ErrUnavailable
	}
	if _, err := c.policy(ctx, proof); err != nil {
		return err
	}
	return ctx.Err()
}
