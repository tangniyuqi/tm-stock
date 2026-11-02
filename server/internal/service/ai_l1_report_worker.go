package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// AIL1ReportPlan 只能来自可信规划器，不接会员正文/价格/模型/授权证明。
// 支持同一批准模型的原引文档案与显式公司政策，不冒充完整L2或生产最低交付政策。
type AIL1ReportPlan struct {
	Agent     agenttask.Plan
	Model     finalreport.CheckConfig
	Policy    finalreport.Config
	ThemeName string
	Snapshots []sources.Snapshot
	Companies *finalreport.CompanyDraft `json:"-"`
	// AnalysisType 0 保留既有 L1 编码；2 为深度报告，必须与任务、终检策略同时为 2。
	AnalysisType    int                         `json:",omitempty"`
	ReportProfile   string                      `json:",omitempty"`
	SupportUnits    *task.Units                 `json:",omitempty"`
	Dossier         *dossier.SupportPlan        `json:"-"`
	ComparisonUnits *task.Units                 `json:",omitempty"`
	Comparison      *dossier.CardComparisonPlan `json:"-"`
}

func (p AIL1ReportPlan) String() string {
	return "L1内部编排计划(证据与批准引用已隐藏)"
}
func (p AIL1ReportPlan) GoString() string { return p.String() }

type AIL1ReportPlanner interface {
	Plan(context.Context, task.Task) (AIL1ReportPlan, error)
	Recheck(context.Context, task.Task, AIL1ReportPlan) error
}
type l1ReportStore interface {
	taskAgentStore
	finalreport.Store
	task.AdmissionStore
}
type AIL1ReportReceipt struct {
	Task   task.RecoveryResult `json:"task"`
	Commit finalreport.Receipt `json:"commit"`
}
type AIL1ReportWorker struct {
	tasks      recoveryTasks
	store      l1ReportStore
	ledger     *PointsService
	agent      taskAgentClient
	payloads   agents.PayloadReader
	model      *provider.OpenAICompat
	planner    AIL1ReportPlanner
	selection  AIReportSelectionControl
	checks     AIReportCheckControl
	owner      string
	opts       agenttask.Options
	support    AIDossierSupportControl
	comparison AIDossierComparisonControl
}

func NewAIL1ReportWorker(tasks recoveryTasks, store l1ReportStore, ledger *PointsService, agent taskAgentClient, payloads agents.PayloadReader, model *provider.OpenAICompat, planner AIL1ReportPlanner, selection AIReportSelectionControl, checks AIReportCheckControl, owner string, o agenttask.Options) (*AIL1ReportWorker, error) {
	if tasks == nil || store == nil || ledger == nil || !task.ValidOwner(owner) || o.Validate() != nil || o.Lease.Lease < time.Minute || o.Enabled && (agent == nil || payloads == nil || model == nil || model.DirectProfileDigest() == "" || planner == nil || selection == nil || checks == nil) {
		return nil, points.ErrInvalid
	}
	if o.Enabled {
		planner = admittedL1Planner{planner, store}
	}
	return &AIL1ReportWorker{tasks: tasks, store: store, ledger: ledger, agent: agent, payloads: payloads, model: model, planner: planner, selection: selection, checks: checks, owner: owner, opts: o}, nil
}

// 新六步编排必须显式提供当前核验控制器；旧构造器不隐式开放新能力。
func NewAIDossierReportWorker(tasks recoveryTasks, store l1ReportStore, ledger *PointsService, agent taskAgentClient, payloads agents.PayloadReader, model *provider.OpenAICompat, planner AIL1ReportPlanner, selection AIReportSelectionControl, checks AIReportCheckControl, support AIDossierSupportControl, owner string, o agenttask.Options) (*AIL1ReportWorker, error) {
	if o.Enabled && support == nil {
		return nil, points.ErrInvalid
	}
	w, err := NewAIL1ReportWorker(tasks, store, ledger, agent, payloads, model, planner, selection, checks, owner, o)
	if err != nil {
		return nil, err
	}
	w.support = support
	return w, nil
}

func NewAIDossierComparisonReportWorker(tasks recoveryTasks, store l1ReportStore, ledger *PointsService, agent taskAgentClient, payloads agents.PayloadReader, model *provider.OpenAICompat, planner AIL1ReportPlanner, selection AIReportSelectionControl, checks AIReportCheckControl, support AIDossierSupportControl, comparison AIDossierComparisonControl, owner string, o agenttask.Options) (*AIL1ReportWorker, error) {
	if o.Enabled && comparison == nil {
		return nil, points.ErrInvalid
	}
	w, err := NewAIDossierReportWorker(tasks, store, ledger, agent, payloads, model, planner, selection, checks, support, owner, o)
	if err != nil {
		return nil, err
	}
	w.comparison = comparison
	return w, nil
}

func (p AIL1ReportPlan) pendingPolicy() (*finalreport.Policy, error) {
	if p.ReportProfile == "" {
		if p.SupportUnits != nil || p.Dossier != nil || p.Policy.DossierSupportDigest != "" || p.ComparisonUnits != nil || p.Comparison != nil || p.Policy.DossierComparisonDigest != "" {
			return nil, points.ErrInvalid
		}
		return pendingPolicy(p.Policy, p.Companies)
	}
	if p.ReportProfile != task.DossierReportProfile && p.ReportProfile != task.DossierComparisonReportProfile || !p.Dossier.Valid() || p.Dossier.StepNo() != 4 || p.SupportUnits == nil || *p.SupportUnits != p.Dossier.Units() {
		return nil, points.ErrInvalid
	}
	e := func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
		return finalreport.Uncertain, nil
	}
	s := func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil }
	if p.ReportProfile == task.DossierComparisonReportProfile {
		if !p.Comparison.Valid() || p.ComparisonUnits == nil || *p.ComparisonUnits != p.Comparison.Units() {
			return nil, points.ErrInvalid
		}
		return finalreport.NewDossierComparisonPolicy(p.Policy, p.Dossier, p.Comparison, e, s, p.Companies)
	}
	if p.ComparisonUnits != nil || p.Comparison != nil || p.Policy.DossierComparisonDigest != "" {
		return nil, points.ErrInvalid
	}
	return finalreport.NewDossierPolicy(p.Policy, p.Dossier, e, s, p.Companies)
}

// pendingPolicy 只用于构造/绑定候选，所有未捕获判定均为不明，绝无默认通过。
func pendingPolicy(c finalreport.Config, companies ...*finalreport.CompanyDraft) (*finalreport.Policy, error) {
	e := func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
		return finalreport.Uncertain, nil
	}
	s := func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil }
	if len(companies) > 1 {
		return nil, points.ErrInvalid
	}
	if len(companies) == 1 && companies[0] != nil {
		return finalreport.NewCompanyPolicy(c, companies[0], e, s)
	}
	return finalreport.NewPolicy(c, e, s)
}
func (p AIL1ReportPlan) validate(v task.Task, model *provider.OpenAICompat) (*finalreport.Policy, error) {
	wantType := 1
	if p.AnalysisType == task.DeepAnalysisType {
		wantType = task.DeepAnalysisType
	} else if p.AnalysisType != 0 {
		return nil, points.ErrInvalid
	}
	wantPolicyType := 0
	if wantType == task.DeepAnalysisType {
		wantPolicyType = task.DeepAnalysisType
	}
	if p.Agent.ValidateScope(v) != nil || v.SubjectType != task.Theme || v.AnalysisType != wantType || p.Policy.AnalysisType != wantPolicyType || model == nil || !model.MatchesDirect(p.Model.ProfileDigest, p.Model.Model) || p.Policy.ModelName != p.Model.Model || p.Policy.FilingNo != p.Model.FilingNo {
		return nil, points.ErrInvalid
	}
	ent, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, p.Model)
	if err != nil {
		return nil, err
	}
	sem, err := finalreport.CheckPolicyVersion(finalreport.CheckSemantic, p.Model)
	if err != nil {
		return nil, err
	}
	if ent != p.Policy.EntailmentVersion || sem != p.Policy.SemanticVersion {
		return nil, task.ErrStagePolicy
	}
	if err := p.preflightBudget(v.Budget); err != nil {
		return nil, err
	}
	policy, err := p.pendingPolicy()
	if err != nil {
		return nil, err
	}
	if p.Dossier != nil && (!p.Dossier.MatchesTask(v) || p.Dossier.Config().Model != p.Model.Model || p.Dossier.Config().ProfileDigest != p.Model.ProfileDigest) {
		return nil, points.ErrIdempotency
	}
	if p.Comparison != nil && (!p.Comparison.MatchesTask(v) || p.Comparison.Config().Model.Model != p.Model.Model || p.Comparison.Config().Model.ProfileDigest != p.Model.ProfileDigest) {
		return nil, points.ErrIdempotency
	}
	if _, err := finalreport.NewSelectionStage(policy, v.No, v.SubjectID, p.ThemeName, p.Snapshots); err != nil {
		return nil, err
	}
	return policy, nil
}

// 准入与执行共用完整容量预检，下游各步仍须持久预留，不能等冻结/模型调用之后才发现不足。
func (p AIL1ReportPlan) preflightBudget(budget task.Budget) error {
	if _, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, p.Model); err != nil {
		return err
	}
	units, err := p.Agent.Units.Add(task.Units{Calls: 4, Tokens: 3*p.Model.MaxInputTokens + int64(p.Model.SelectionTokens()) + 2*int64(p.Model.CheckTokens()) + 1})
	if err != nil {
		return task.ErrBudget
	}
	if p.ReportProfile != "" {
		if p.ReportProfile != task.DossierReportProfile && p.ReportProfile != task.DossierComparisonReportProfile || p.SupportUnits == nil || p.SupportUnits.Calls != 1 || p.SupportUnits.Pages != 0 || p.SupportUnits.Validate() != nil || !finalreport.ValidDigest(p.Policy.DossierSupportDigest) {
			return points.ErrInvalid
		}
		units, err = units.Add(*p.SupportUnits)
	} else if p.SupportUnits != nil || p.Policy.DossierSupportDigest != "" {
		return points.ErrInvalid
	}
	if p.ReportProfile == task.DossierComparisonReportProfile {
		if p.AnalysisType != task.DeepAnalysisType || p.ComparisonUnits == nil || p.ComparisonUnits.Calls != 1 || p.ComparisonUnits.Pages != 0 || p.ComparisonUnits.Validate() != nil || !finalreport.ValidDigest(p.Policy.DossierComparisonDigest) {
			return points.ErrInvalid
		}
		if err == nil {
			units, err = units.Add(*p.ComparisonUnits)
		}
	} else if p.ComparisonUnits != nil || p.Policy.DossierComparisonDigest != "" {
		return points.ErrInvalid
	}
	if err != nil || units.Validate() != nil || !units.Fits(budget.Limits()) {
		return task.ErrBudget
	}
	return nil
}

type l1AgentPlanner struct {
	planner AIL1ReportPlanner
	plan    AIL1ReportPlan
}

type admittedDossierControl struct {
	AIDossierSupportControl
	planner AIL1ReportPlanner
	plan    AIL1ReportPlan
}

func (c admittedDossierControl) Recheck(ctx context.Context, v task.Task, p *dossier.SupportPlan) error {
	if err := c.planner.Recheck(ctx, v, c.plan); err != nil {
		return err
	}
	return c.AIDossierSupportControl.Recheck(ctx, v, p)
}

type admittedComparisonControl struct {
	AIDossierComparisonControl
	planner AIL1ReportPlanner
	plan    AIL1ReportPlan
}

func (c admittedComparisonControl) Recheck(ctx context.Context, v task.Task, p *dossier.CardComparisonPlan) error {
	if err := c.planner.Recheck(ctx, v, c.plan); err != nil {
		return err
	}
	return c.AIDossierComparisonControl.Recheck(ctx, v, p)
}

func (l1AgentPlanner) admittedTaskPlan() {}

func (p l1AgentPlanner) Plan(ctx context.Context, _ task.Task) (agenttask.Plan, error) {
	return p.plan.Agent, ctx.Err()
}
func (p l1AgentPlanner) Recheck(ctx context.Context, v task.Task, a agenttask.Plan) error {
	if a != p.plan.Agent {
		return points.ErrIdempotency
	}
	return p.planner.Recheck(ctx, v, p.plan)
}

// Run 驱动固定主链路；没有生产装配/公开调用入口。未知响应或提交保留原键，不补发。
func (w *AIL1ReportWorker) Run(parent context.Context, uid, id int64) (AIL1ReportReceipt, error) {
	if w == nil || uid <= 0 || id <= 0 {
		return AIL1ReportReceipt{}, points.ErrInvalid
	}
	if !w.opts.Enabled {
		return AIL1ReportReceipt{}, finalreport.ErrDisabled
	}
	ctx, cancel := context.WithTimeout(parent, w.opts.Lease.Timeout)
	defer cancel()
	v, err := w.tasks.Inspect(ctx, uid, id)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	if v.Terminal() {
		return AIL1ReportReceipt{Task: recoveryResult(v, 0, "terminal")}, nil
	}
	if expired, err := w.tasks.Expire(ctx, uid, id); err == nil {
		return AIL1ReportReceipt{Task: recoveryResult(expired, 0, "terminal")}, nil
	} else if !errors.Is(err, task.ErrNotReady) {
		return AIL1ReportReceipt{}, err
	}
	if v.CancelRequested {
		if v.LeaseOwner != w.owner {
			v, err = w.tasks.Claim(ctx, uid, id, w.owner, w.opts.Lease.Lease)
			if err != nil {
				return AIL1ReportReceipt{}, err
			}
		}
		if v.Terminal() {
			return AIL1ReportReceipt{Task: recoveryResult(v, 0, "terminal")}, nil
		}
		return w.finishKnown(parent, v, "internal_failure")
	}
	p, err := w.planner.Plan(ctx, v)
	if err != nil {
		if l1KnownRejection(err) {
			return w.rejectKnown(ctx, v)
		}
		return AIL1ReportReceipt{}, err
	}
	if p.ReportProfile != "" && w.support == nil {
		return AIL1ReportReceipt{}, finalreport.ErrDisabled
	}
	if p.ReportProfile == task.DossierComparisonReportProfile && w.comparison == nil {
		return AIL1ReportReceipt{}, finalreport.ErrDisabled
	}
	policy, err := p.validate(v, w.model)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	if err := w.planner.Recheck(ctx, v, p); err != nil {
		if l1KnownRejection(err) {
			return w.rejectKnown(ctx, v)
		}
		return AIL1ReportReceipt{}, err
	}
	v, err = w.ownTask(ctx, v)
	if err != nil {
		if errors.Is(err, task.ErrUnconfirmed) {
			out, readErr := unconfirmedRecovery(ctx, w.tasks, uid, id)
			if readErr != nil {
				return AIL1ReportReceipt{}, readErr
			}
			return AIL1ReportReceipt{Task: out}, nil
		}
		return AIL1ReportReceipt{}, err
	}
	if v.Terminal() {
		return AIL1ReportReceipt{Task: recoveryResult(v, 0, "terminal")}, nil
	}
	out, err := w.runOwned(ctx, v, p, policy)
	if err == nil {
		return out, nil
	}
	if errors.Is(err, task.ErrCancelled) {
		return w.finishKnown(parent, v, "internal_failure")
	}
	if l1KnownRejection(err) {
		return w.rejectKnown(ctx, v)
	}
	return AIL1ReportReceipt{}, err
}
func (w *AIL1ReportWorker) finishKnown(parent context.Context, v task.Task, code string) (AIL1ReportReceipt, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer cancel()
	done, err := w.tasks.FailOrRetry(ctx, v.UserID, v.ID, w.owner, v.Fence, code, 0)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	return AIL1ReportReceipt{Task: recoveryResult(done, 0, "terminal")}, nil
}
func l1Stage(v task.Task, owner, stage string, step int, key, input string) task.StageRequest {
	return task.StageRequest{UserID: v.UserID, TaskID: v.ID, Owner: owner, Fence: v.Fence, Stage: stage, StepNo: step, OperationKey: key, InputDigest: input}
}
func (w *AIL1ReportWorker) runOwned(ctx context.Context, v task.Task, p AIL1ReportPlan, policy *finalreport.Policy) (AIL1ReportReceipt, error) {
	aw, err := NewAITaskAgentWorker(w.tasks, w.store, w.agent, l1AgentPlanner{w.planner, p}, w.owner, w.opts)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	a, err := aw.RunOwned(ctx, v, p.Agent)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	if a.Outcome == "upstream_terminal" {
		return w.finishKnown(ctx, v, "internal_failure")
	}
	if a.Outcome == "capture_pending" {
		// 远端已知而本地原围栏未捕获仍不具备终结证明，统一保留原键待核查。
		a.Outcome = "response_unknown"
	}
	if a.Outcome != "evidence_pending" {
		return AIL1ReportReceipt{Task: a}, nil
	}
	v, err = w.tasks.Inspect(ctx, v.UserID, v.ID)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	record, result, err := w.payloads.Quarantined(ctx, p.Agent.ActorID, p.Agent.RequestID())
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	hint, err := agenttask.NewHint(v, p.Agent, record, result)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	sel, err := finalreport.NewSelectionStage(policy, v.No, v.SubjectID, p.ThemeName, p.Snapshots)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	sp, err := finalreport.NewSelectionModelPlan(p.Model, sel, v, hint)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	options := AIReportCheckOptions{Enabled: true, Lease: w.opts.Lease}
	bound := func(ctx context.Context, current task.Task) error { return checkAdmittedL1(ctx, w.store, current, p) }
	sw, err := NewAIReportSelectionWorker(w.tasks, w.store, w.model, admittedSelectionControl{w.selection, bound}, w.owner, options)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	sr := l1Stage(v, w.owner, "extract", 2, sp.OperationKey(v, 2), sp.InputDigest())
	sc, err := sw.RunOwned(ctx, sr, sp)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	if sc.Task.Outcome == "response_unknown" {
		return AIL1ReportReceipt{Task: sc.Task}, nil
	}
	svc, err := NewAIReportService(w.store, w.ledger, policy, true)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	draft, err := svc.ConvertModel(ctx, sr, sp)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	proposal, err := policy.Decode(string(draft.Payload))
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	// 引文政策只取实际引用；公司政策另含首次公司材料全集，不能漏掉仅支撑公司行的出处。
	var used []sources.Snapshot
	for _, id := range policy.EvidenceIDs(proposal) {
		for _, snap := range p.Snapshots {
			if snap.ID == id {
				used = append(used, snap)
				break
			}
		}
	}
	in, visible, err := policy.Inputs(string(draft.Payload), used)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	ep, err := finalreport.NewEntailmentCheck(p.Model, in)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	cw, err := NewAIReportCheckWorker(w.tasks, w.store, w.model, admittedCheckControl{w.checks, bound}, w.owner, options)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	last := l1Stage(v, w.owner, "judge", 3, ep.OperationKey(v, 3), ep.InputDigest())
	ent, err := cw.RunOwned(ctx, last, ep)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	if ent.Evidence == nil {
		return AIL1ReportReceipt{Task: ent.Task}, nil
	}
	var sem AIReportCheckReceipt
	var support AIDossierSupportReceipt
	var comparison AIDossierComparisonReceipt
	if ent.Evidence.Decision().Verdict == finalreport.Pass {
		semanticStep := 4
		if p.ReportProfile != "" {
			worker, err := NewAIDossierSupportWorker(w.tasks, w.store, w.model, admittedDossierControl{AIDossierSupportControl: w.support, planner: w.planner, plan: p}, w.owner, options)
			if err != nil {
				return AIL1ReportReceipt{}, err
			}
			last = l1Stage(v, w.owner, "judge", 4, p.Dossier.OperationKey(v, 4), p.Dossier.InputDigest())
			support, err = worker.RunOwned(ctx, last, p.Dossier)
			if err != nil {
				return AIL1ReportReceipt{}, err
			}
			if support.Review == nil {
				return AIL1ReportReceipt{Task: support.Task}, nil
			}
			visible, err = finalreport.DossierVisible(visible, p.Dossier, support.Review)
			if err != nil {
				return AIL1ReportReceipt{}, err
			}
			semanticStep = 5
		}
		if p.ReportProfile == task.DossierComparisonReportProfile && support.Review.ResolvedCount() > 0 {
			worker, err := NewAIDossierComparisonWorker(w.tasks, w.store, w.model, admittedComparisonControl{AIDossierComparisonControl: w.comparison, planner: w.planner, plan: p}, w.owner, options)
			if err != nil {
				return AIL1ReportReceipt{}, err
			}
			last = l1Stage(v, w.owner, "judge", 5, p.Comparison.OperationKey(v, 5), p.Comparison.InputDigest())
			comparison, err = worker.RunOwned(ctx, last, p.Comparison)
			if err != nil {
				return AIL1ReportReceipt{}, err
			}
			if comparison.Review == nil {
				return AIL1ReportReceipt{Task: comparison.Task}, nil
			}
			visible, err = finalreport.DossierComparisonVisible(visible, p.Dossier, support.Review, p.Comparison, comparison.Review)
			if err != nil {
				return AIL1ReportReceipt{}, err
			}
			semanticStep = 6
		}
		// 全部资料不足时保留字段存证，直接沿终检失败释放，不制造语义调用。
		if p.ReportProfile == "" || support.Review.ResolvedCount() > 0 && (p.ReportProfile != task.DossierComparisonReportProfile || comparison.Review.ResolvedCount() > 0) {
			mp, err := finalreport.NewSemanticCheck(p.Model, v.No, visible)
			if err != nil {
				return AIL1ReportReceipt{}, err
			}
			last = l1Stage(v, w.owner, "judge", semanticStep, mp.OperationKey(v, semanticStep), mp.InputDigest())
			sem, err = cw.RunOwned(ctx, last, mp)
			if err != nil {
				return AIL1ReportReceipt{}, err
			}
			if sem.Evidence == nil {
				return AIL1ReportReceipt{Task: sem.Task}, nil
			}
		}
	}
	// 语义判定提交后必须刷新任务步号；否则最终阶段可能按旧 current_step 预留失败。
	v, err = w.tasks.Inspect(ctx, v.UserID, v.ID)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	var finalPolicy *finalreport.Policy
	if p.ReportProfile == task.DossierComparisonReportProfile {
		finalPolicy, err = finalreport.NewDossierComparisonCapturedPolicy(p.Policy, p.Dossier, p.Comparison, support.Review, comparison.Review, ent.Evidence, sem.Evidence, p.Companies)
	} else if p.ReportProfile == task.DossierReportProfile {
		finalPolicy, err = finalreport.NewDossierCapturedPolicy(p.Policy, p.Dossier, support.Review, ent.Evidence, sem.Evidence, p.Companies)
	} else {
		finalPolicy, err = finalreport.NewCapturedPolicy(p.Policy, ent.Evidence, sem.Evidence, p.Companies)
	}
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	svc, err = NewAIReportService(w.store, w.ledger, finalPolicy, true)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	finalInput := []any{"l1-final-input-v1", draft.InputDigest, ent.Checkpoint, sem.Checkpoint}
	finalKey := []any{"l1-final-operation-v1", v.UserID, v.ID, v.No}
	if p.ReportProfile == task.DossierReportProfile {
		finalInput = []any{task.DossierReportProfile, draft.InputDigest, ent.Checkpoint, support.Checkpoint, sem.Checkpoint}
		finalKey = []any{task.DossierReportProfile, v.UserID, v.ID, v.No}
	}
	if p.ReportProfile == task.DossierComparisonReportProfile {
		finalInput = []any{task.DossierComparisonReportProfile, draft.InputDigest, ent.Checkpoint, support.Checkpoint, comparison.Checkpoint, sem.Checkpoint}
		finalKey = []any{task.DossierComparisonReportProfile, v.UserID, v.ID, v.No}
	}
	b, err := json.Marshal(finalInput)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	key, err := json.Marshal(finalKey)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	r := l1Stage(v, w.owner, "judge", last.StepNo+1, task.StageDigest(key), task.StageDigest(b))
	keeper := &AIRecoveryWorker{tasks: w.tasks, owner: w.owner, options: w.opts.Lease}
	prepared, err := keeper.withHeartbeatWork(ctx, v, func(ctx context.Context) (task.RecoveryResult, error) {
		if err := w.planner.Recheck(ctx, v, p); err != nil {
			return task.RecoveryResult{}, err
		}
		budget, err := NewAITaskBudgetService(w.store)
		if err != nil {
			return task.RecoveryResult{}, err
		}
		res, err := budget.ReserveStep(ctx, task.ReserveRequest{UserID: v.UserID, TaskID: v.ID, Owner: w.owner, Fence: v.Fence, Key: r.OperationKey, InputDigest: r.InputDigest, Units: task.Units{Calls: 1, Tokens: 1}}, r.StepNo)
		if err != nil {
			return task.RecoveryResult{}, err
		}
		stages, err := NewAITaskStageService(w.store, map[string]task.StagePolicy{"judge": finalPolicy.StagePolicy()})
		if err != nil {
			return task.RecoveryResult{}, err
		}
		_, err = stages.Complete(ctx, r)
		if errors.Is(err, points.ErrNotFound) {
			if res.Reservation.Owner != w.owner || res.Reservation.Fence != v.Fence {
				return recoveryResult(v, r.StepNo, "response_unknown"), nil
			}
			if _, err := stages.Capture(ctx, r, draft.Payload); err != nil {
				return task.RecoveryResult{}, err
			}
			_, err = stages.Complete(ctx, r)
		}
		if err != nil {
			return task.RecoveryResult{}, err
		}
		return recoveryResult(v, r.StepNo, "final_prepared"), nil
	})
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	if prepared.Outcome == "response_unknown" {
		return AIL1ReportReceipt{Task: prepared}, nil
	}
	// 先停止并收回心跳，再续足至少一分钟租约，避免成功终态被自己的心跳误判为失租。
	if err := keeper.beat(ctx, v); err != nil {
		return AIL1ReportReceipt{}, err
	}
	commit, err := svc.Finalize(ctx, finalreport.Request{TaskNo: v.No, Stage: r})
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	v, err = w.tasks.Inspect(ctx, v.UserID, v.ID)
	if err != nil {
		return AIL1ReportReceipt{}, err
	}
	return AIL1ReportReceipt{Task: recoveryResult(v, r.StepNo, "terminal"), Commit: commit}, nil
}
