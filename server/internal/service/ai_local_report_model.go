package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// AIReportCallAudit 必须持久保存原键的调用前后记录；不能用日志或空实现代替。
// 接口本身不证明实现已获批准，正式启动仍须核验实际审计仓储。
type AIReportCallAudit interface {
	Start(context.Context, task.StageRequest, provider.Request) error
	Finish(context.Context, task.StageRequest, provider.Response, error) error
}

type localReportModelControl struct {
	admission *AILocalAdmissionControl
	tasks     *AITaskService
	ledger    *PointsService
	store     l1ReportStore
	payloads  agents.PayloadReader
	model     *provider.OpenAICompat
	audit     AIReportCallAudit
}

type AILocalReportSelectionControl struct{ core *localReportModelControl }
type AILocalReportCheckControl struct{ core *localReportModelControl }

// 仅装配无密钥的字面回环协议服务，不启动请求、队列或真实模型。
func NewAILocalReportModelControls(admission *AILocalAdmissionControl, ledger *PointsService, payloads agents.PayloadReader, cfg provider.Config, audit AIReportCallAudit) (*AILocalReportSelectionControl, *AILocalReportCheckControl, *provider.OpenAICompat, error) {
	return newAILocalReportModelControls(admission, ledger, payloads, cfg, audit, "")
}

// 临时会话仅用于显式本地父桥认证，不赋予真实模型批准；原准入、档案和调用存证照常核验。
// 供应商Key仍禁止从cfg传入，调用者须在独立受控父桥保管真实凭据。
func NewAILocalReportModelControlsWithSession(admission *AILocalAdmissionControl, ledger *PointsService, payloads agents.PayloadReader, cfg provider.Config, audit AIReportCallAudit, session string) (*AILocalReportSelectionControl, *AILocalReportCheckControl, *provider.OpenAICompat, error) {
	if session == "" {
		return nil, nil, nil, points.ErrInvalid
	}
	if _, err := config.ParseAILocalModelSession(session); err != nil {
		return nil, nil, nil, points.ErrInvalid
	}
	return newAILocalReportModelControls(admission, ledger, payloads, cfg, audit, session)
}

func newAILocalReportModelControls(admission *AILocalAdmissionControl, ledger *PointsService, payloads agents.PayloadReader, cfg provider.Config, audit AIReportCallAudit, session string) (*AILocalReportSelectionControl, *AILocalReportCheckControl, *provider.OpenAICompat, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u == nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (strings.TrimRight(u.Path, "/") != "" && strings.TrimRight(u.Path, "/") != "/v1") || cfg.APIKey != "" || cfg.JSONSchemaFallback || cfg.ReasoningEffort != "" || admission == nil || ledger == nil || payloads == nil || audit == nil {
		return nil, nil, nil, points.ErrInvalid
	}
	store, ok := admission.evidence.(l1ReportStore)
	if !ok || admission.source == nil || admission.identity == nil {
		return nil, nil, nil, points.ErrInvalid
	}
	taskStore, ok := admission.evidence.(task.Store)
	if !ok {
		return nil, nil, nil, points.ErrInvalid
	}
	tasks, err := NewAITaskService(taskStore, ledger)
	if err != nil {
		return nil, nil, nil, err
	}
	// 只在实际Provider内持有临时父桥会话，不进入配置文件/日志/身份摘要。
	cfg.APIKey = session
	model, err := provider.NewDirect(cfg, true)
	if err != nil {
		return nil, nil, nil, points.ErrInvalid
	}
	core := &localReportModelControl{admission, tasks, ledger, store, payloads, model, audit}
	return &AILocalReportSelectionControl{core}, &AILocalReportCheckControl{core}, model, nil
}

// expected 亲自回取原隔离Agent及已捕获选句，不信任调用方提供的提示或判定正文。
// 只读转换仍经过原准入/检查点/当前资料核验，不执行模型或账务操作。
func (c *localReportModelControl) expected(ctx context.Context, v task.Task, step int) (finalreport.ModelStep, error) {
	if c == nil || ctx == nil || step < 2 || step > 4 {
		return nil, points.ErrInvalid
	}
	p, err := c.admission.ExecutionPlan(ctx, v)
	if err != nil {
		return nil, err
	}
	// 本地装配不接管无首次准入的历史开发任务。
	if err := c.store.WithinAdmissionTx(ctx, func(tx task.AdmissionReadTx) error {
		current, err := lockOwnedTask(ctx, tx, v.UserID, v.ID)
		if err != nil {
			return err
		}
		_, found, err := reportAdmission(ctx, tx, current)
		if err != nil {
			return err
		}
		if !found {
			return task.ErrAdmissionUnavailable
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := checkAdmittedL1(ctx, c.store, v, p); err != nil {
		return nil, err
	}
	policy, err := p.validate(v, c.model)
	if err != nil {
		return nil, fmt.Errorf("本地执行配置核验失败: %w", err)
	}
	record, result, err := c.payloads.Quarantined(ctx, p.Agent.ActorID, p.Agent.RequestID())
	if err != nil {
		return nil, err
	}
	hint, err := agenttask.NewHint(v, p.Agent, record, result)
	if err != nil {
		return nil, fmt.Errorf("本地隔离回执核验失败: %w", err)
	}
	selection, err := finalreport.NewSelectionStage(policy, v.No, v.SubjectID, p.ThemeName, p.Snapshots)
	if err != nil {
		return nil, err
	}
	sp, err := finalreport.NewSelectionModelPlan(p.Model, selection, v, hint)
	if err != nil || step == 2 {
		return sp, err
	}
	svc, err := NewAIReportService(c.store, c.ledger, policy, true)
	if err != nil {
		return nil, err
	}
	r := l1Stage(v, v.LeaseOwner, "extract", 2, sp.OperationKey(v, 2), sp.InputDigest())
	draft, err := svc.ConvertModel(ctx, r, sp)
	if err != nil {
		return nil, fmt.Errorf("本地原选句回取失败: %w", err)
	}
	proposal, err := policy.Decode(string(draft.Payload))
	if err != nil {
		return nil, err
	}
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
		return nil, err
	}
	if step == 3 {
		return finalreport.NewEntailmentCheck(p.Model, in)
	}
	return finalreport.NewSemanticCheck(p.Model, v.No, visible)
}

func (c *localReportModelControl) recheck(parent context.Context, v task.Task, p finalreport.ModelStep, step int) error {
	if parent == nil || p == nil || !p.Valid() {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	want, err := c.expected(ctx, v, step)
	if err != nil {
		return fmt.Errorf("本地第%d步调用计划复核失败: %w", step, err)
	}
	if !p.MatchesTask(v) || want.InputDigest() != p.InputDigest() || want.Config() != p.Config() {
		return points.ErrIdempotency
	}
	return ctx.Err()
}

func (c *AILocalReportSelectionControl) Recheck(ctx context.Context, v task.Task, p *finalreport.SelectionModelPlan) error {
	if c == nil || c.core == nil {
		return points.ErrInvalid
	}
	return c.core.recheck(ctx, v, p, 2)
}
func (c *AILocalReportCheckControl) Recheck(ctx context.Context, v task.Task, p *finalreport.CheckPlan) error {
	if c == nil || c.core == nil || !p.Valid() {
		return points.ErrInvalid
	}
	step := 3
	if p.Kind() == finalreport.CheckSemantic {
		step = 4
	}
	return c.core.recheck(ctx, v, p, step)
}

func (c *localReportModelControl) start(parent context.Context, r task.StageRequest, req provider.Request) error {
	if parent == nil || r.Validate() != nil || r.StepNo < 2 || r.StepNo > 4 {
		return points.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	v, err := c.tasks.Inspect(ctx, r.UserID, r.TaskID)
	if err != nil {
		return err
	}
	if v.Status != task.Running || v.CancelRequested || v.Fence != r.Fence || v.LeaseOwner != r.Owner {
		return points.ErrIdempotency
	}
	p, err := c.expected(ctx, v, r.StepNo)
	if err != nil {
		return fmt.Errorf("本地第%d步调用前重建失败: %w", r.StepNo, err)
	}
	if p.InputDigest() != r.InputDigest || p.OperationKey(v, r.StepNo) != r.OperationKey || p.StageName() != r.Stage {
		return points.ErrIdempotency
	}
	want := p.Request()
	want.TaskID, want.Step = v.No, r.StepNo
	a, err := json.Marshal(want)
	if err != nil {
		return points.ErrInvalid
	}
	b, err := json.Marshal(req)
	if err != nil || string(a) != string(b) {
		return points.ErrIdempotency
	}
	return c.audit.Start(ctx, r, req)
}

func (c *AILocalReportSelectionControl) Start(ctx context.Context, r task.StageRequest, req provider.Request) error {
	if c == nil || c.core == nil || r.StepNo != 2 || r.Stage != "extract" {
		return points.ErrInvalid
	}
	return c.core.start(ctx, r, req)
}
func (c *AILocalReportCheckControl) Start(ctx context.Context, r task.StageRequest, req provider.Request) error {
	if c == nil || c.core == nil || (r.StepNo != 3 && r.StepNo != 4) || r.Stage != "judge" {
		return points.ErrInvalid
	}
	return c.core.start(ctx, r, req)
}

// 返回后即使资料已撤回也须先持久存证；不能因撤回丢掉未知调用的核对依据。
func (c *AILocalReportSelectionControl) Finish(ctx context.Context, r task.StageRequest, out provider.Response, callErr error) error {
	if c == nil || c.core == nil || ctx == nil || r.Validate() != nil || r.StepNo != 2 || r.Stage != "extract" {
		return points.ErrInvalid
	}
	return c.core.audit.Finish(ctx, r, out, callErr)
}
func (c *AILocalReportCheckControl) Finish(ctx context.Context, r task.StageRequest, out provider.Response, callErr error) error {
	if c == nil || c.core == nil || ctx == nil || r.Validate() != nil || (r.StepNo != 3 && r.StepNo != 4) || r.Stage != "judge" {
		return points.ErrInvalid
	}
	return c.core.audit.Finish(ctx, r, out, callErr)
}
