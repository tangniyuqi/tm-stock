package service

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type localInstanceCheck interface {
	Verify(context.Context, string) error
}

// AILocalAdmissionControl 仅用于带隔离标记的合成资料计划，不授予模型调用或生产批准。
// 文件、资料当前授权和完整执行身份逐次复核；与已有任务准入/积分事务共用同一契约。
type AILocalAdmissionControl struct {
	source   *config.AILocalPlanSource
	identity localInstanceCheck
	evidence finalreport.Store
}

func NewAILocalAdmissionControl(source *config.AILocalPlanSource, identity localInstanceCheck, evidence finalreport.Store) (*AILocalAdmissionControl, error) {
	if source == nil || identity == nil || evidence == nil {
		return nil, points.ErrInvalid
	}
	return &AILocalAdmissionControl{source, identity, evidence}, nil
}

func (c *AILocalAdmissionControl) current(ctx context.Context) (*config.AILocalPlans, error) {
	if c == nil || ctx == nil {
		return nil, points.ErrInvalid
	}
	p, err := c.source.Current(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.identity.Verify(ctx, p.InstanceID); err != nil {
		return nil, err
	}
	return p, nil
}

func (c *AILocalAdmissionControl) Ready(ctx context.Context) error {
	if ctx == nil {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p, err := c.current(ctx)
	if err != nil {
		return err
	}
	for _, spec := range p.Plans {
		if _, _, err := c.build(ctx, spec, p.Members[0]); err != nil {
			return err
		}
	}
	_, err = c.source.Current(ctx)
	return err
}

func (c *AILocalAdmissionControl) ReadyDeep(ctx context.Context) error {
	if ctx == nil {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p, err := c.current(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, spec := range p.Plans {
		if spec.AnalysisType == task.DeepAnalysisType {
			found = true
			if _, _, err := c.build(ctx, spec, p.Members[0]); err != nil {
				return err
			}
		}
	}
	if !found {
		return task.ErrAdmissionUnavailable
	}
	_, err = c.source.Current(ctx)
	return err
}

func (c *AILocalAdmissionControl) Plan(ctx context.Context, scope task.AdmissionScope) (task.AdmissionPlan, error) {
	if ctx == nil {
		return task.AdmissionPlan{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if scope.Validate() != nil || scope.SubjectType != task.Theme {
		return task.AdmissionPlan{}, task.ErrAdmissionUnavailable
	}
	p, err := c.current(ctx)
	if err != nil {
		return task.AdmissionPlan{}, err
	}
	if !slices.Contains(p.Members, scope.UserID) {
		return task.AdmissionPlan{}, task.ErrAdmissionUnavailable
	}
	for _, spec := range p.Plans {
		if spec.AnalysisType == scope.AnalysisType && spec.SubjectID == scope.SubjectID {
			out, _, err := c.build(ctx, spec, scope.UserID)
			if err != nil {
				return task.AdmissionPlan{}, err
			}
			if _, err := c.source.Current(ctx); err != nil {
				return task.AdmissionPlan{}, err
			}
			return out, nil
		}
	}
	return task.AdmissionPlan{}, task.ErrAdmissionUnavailable
}

func (c *AILocalAdmissionControl) Recheck(ctx context.Context, plan task.AdmissionPlan) error {
	want, err := c.Plan(ctx, plan.Scope)
	if err != nil {
		return err
	}
	a, err := points.RequestHash("local-report-admission-v1", want)
	if err != nil {
		return err
	}
	b, err := points.RequestHash("local-report-admission-v1", plan)
	if err != nil || a != b {
		return points.ErrIdempotency
	}
	return ctx.Err()
}

// ExecutionPlan 只装配同源计划，不执行网络调用；worker另须显式执行授权及调用审计。
// 创建后身份单独绑定，不能因为AdmissionDigest排除任务号就接管其它任务。
func (c *AILocalAdmissionControl) ExecutionPlan(ctx context.Context, v task.Task) (AIL1ReportPlan, error) {
	if ctx == nil {
		return AIL1ReportPlan{}, points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	want, err := c.Plan(ctx, task.AdmissionScope{UserID: v.UserID, AnalysisType: v.AnalysisType, SubjectType: v.SubjectType, SubjectID: v.SubjectID})
	if err != nil {
		return AIL1ReportPlan{}, err
	}
	if v.ID <= 0 || v.No == "" || v.Budget != want.Budget || v.PriceVersion != want.Price.Version || v.PricePoints != want.Price.Points {
		return AIL1ReportPlan{}, points.ErrIdempotency
	}
	p, err := c.current(ctx)
	if err != nil {
		return AIL1ReportPlan{}, err
	}
	for _, spec := range p.Plans {
		if spec.AnalysisType == v.AnalysisType && spec.SubjectID == v.SubjectID {
			admission, out, err := c.build(ctx, spec, v.UserID)
			if err != nil {
				return AIL1ReportPlan{}, err
			}
			// 第二次读取必须仍是首次准入身份，不能接受调用间撤回/替换资料。
			a, err := points.RequestHash("local-report-admission-v1", admission)
			if err != nil {
				return AIL1ReportPlan{}, err
			}
			b, err := points.RequestHash("local-report-admission-v1", want)
			if err != nil {
				return AIL1ReportPlan{}, err
			}
			if a != b {
				return AIL1ReportPlan{}, points.ErrIdempotency
			}
			out.Agent.UserID, out.Agent.TaskID, out.Agent.TaskNo = v.UserID, v.ID, v.No
			out.Agent.OperationKey = out.Agent.RequestID()
			if err := out.Agent.ValidateScope(v); err != nil {
				return AIL1ReportPlan{}, err
			}
			if _, err := c.source.Current(ctx); err != nil {
				return AIL1ReportPlan{}, err
			}
			return out, nil
		}
	}
	return AIL1ReportPlan{}, task.ErrAdmissionUnavailable
}

func (c *AILocalAdmissionControl) build(ctx context.Context, spec config.AILocalPlan, uid int64) (task.AdmissionPlan, AIL1ReportPlan, error) {
	invalid := func(stage string) (task.AdmissionPlan, AIL1ReportPlan, error) {
		return task.AdmissionPlan{}, AIL1ReportPlan{}, fmt.Errorf("本地计划%s核验失败: %w", stage, task.ErrAdmissionUnavailable)
	}
	price := points.Price{Version: spec.PriceVersion, Points: spec.PricePoints}
	if price.Validate() != nil || spec.AnalysisType == 2 && price.Points != task.DeepPricePoints {
		return invalid("定价")
	}
	model := finalreport.CheckConfig{Model: spec.ModelName, FilingNo: spec.FilingNo, Version: spec.ModelVersion, ApprovalRef: spec.ApprovalRef, QualityRef: spec.QualityRef, HardBudgetRef: spec.HardBudgetRef, ProfileDigest: spec.ModelProfileDigest, MaxInputTokens: spec.MaxInputTokens, SelectionMaxTokens: spec.SelectionMaxTokens, CheckMaxTokens: spec.CheckMaxTokens}
	if spec.AnalysisType == 2 {
		model.AnalysisType = 2
	}
	ent, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, model)
	if err != nil {
		return invalid("蕴含策略")
	}
	sem, err := finalreport.CheckPolicyVersion(finalreport.CheckSemantic, model)
	if err != nil {
		return invalid("语义策略")
	}
	policy := finalreport.Config{Ref: spec.PolicyRef, ModelName: model.Model, FilingNo: model.FilingNo, EntailmentVersion: ent, SemanticVersion: sem, AllowedDomains: slices.Clone(spec.AllowedDomains), AnalysisType: model.AnalysisType, MinQuotes: spec.MinQuotes}
	p := AIL1ReportPlan{Model: model, Policy: policy, ThemeName: spec.SubjectName, AnalysisType: model.AnalysisType}
	p.Agent = agenttask.Plan{SubjectType: task.Theme, SubjectID: spec.SubjectID, ActorID: spec.ActorID, Symbol: spec.Symbol, ProfileDigest: spec.AgentProfileDigest, Units: task.Units{Calls: spec.AgentCalls, Tokens: spec.AgentTokens, Pages: spec.AgentPages}}
	if p.preflightBudget(spec.Budget) != nil {
		return invalid("完整预算")
	}
	ids := make([]int64, len(spec.Materials))
	for i, m := range spec.Materials {
		ids[i] = m.SnapshotID
	}
	err = c.evidence.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		snaps, name, err := tx.ReportEvidence(ctx, ids, spec.SubjectID)
		if err != nil {
			return err
		}
		if name != spec.SubjectName || len(snaps) != len(spec.Materials) {
			return task.ErrAdmissionUnavailable
		}
		for i, s := range snaps {
			m := spec.Materials[i]
			u, err := url.Parse(s.URL)
			if err != nil || !slices.Contains(spec.AllowedDomains, u.Hostname()) || !strings.HasSuffix(u.Hostname(), ".example.test") || s.ID != m.SnapshotID || s.Hash != m.Digest || s.LicenseRef != m.LicenseRef {
				return task.ErrAdmissionUnavailable
			}
		}
		p.Snapshots = snaps
		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return task.AdmissionPlan{}, AIL1ReportPlan{}, ctx.Err()
		}
		return invalid("当前证据")
	}
	guard, err := pendingPolicy(policy)
	if err != nil {
		return invalid("终检策略")
	}
	// 固定占位身份仅做结构校验，不参与准入摘要，也不创建任务或授予执行权。
	probe := task.Task{ID: 1, UserID: uid, No: strings.Repeat("a", 32), SubjectType: task.Theme, SubjectID: spec.SubjectID, AnalysisType: spec.AnalysisType, Budget: spec.Budget}
	p.Agent.UserID, p.Agent.TaskID, p.Agent.TaskNo = probe.UserID, probe.ID, probe.No
	p.Agent.OperationKey = p.Agent.RequestID()
	if p.Agent.ValidateScope(probe) != nil {
		return invalid("Agent身份")
	}
	if _, err := finalreport.NewSelectionStage(guard, probe.No, spec.SubjectID, spec.SubjectName, p.Snapshots); err != nil {
		return invalid("选句输入")
	}
	digest, err := p.AdmissionDigest()
	if err != nil {
		return invalid("执行摘要")
	}
	admission := task.AdmissionPlan{Scope: task.AdmissionScope{UserID: uid, AnalysisType: spec.AnalysisType, SubjectType: task.Theme, SubjectID: spec.SubjectID}, Price: price, Budget: spec.Budget, SubjectName: spec.SubjectName, ModelName: model.Model, FilingNo: model.FilingNo, ProfileDigest: model.ProfileDigest, ExecutionDigest: digest, ApprovalRef: spec.ApprovalRef, QualityRef: spec.QualityRef, HardBudgetRef: spec.HardBudgetRef, PriceRef: spec.PriceRef, SourceRef: spec.SourceRef}
	for _, s := range p.Snapshots {
		admission.Materials = append(admission.Materials, task.AdmissionMaterial{SnapshotID: s.ID, Digest: s.Hash, Domain: urlDomain(s.URL), Category: s.SourceCategory, URL: s.URL, CollectedAt: s.FetchedAt})
	}
	if admission.Validate(admission.Scope) != nil {
		return invalid("准入身份")
	}
	return admission, p, nil
}
