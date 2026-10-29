package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/agents/tradingcn"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/agenttask"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/config"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
	"github.com/tangniyuqi/tm-stock/server/internal/repository"
	"github.com/tangniyuqi/tm-stock/server/internal/service"
)

type localExecutionGuard struct {
	execution                             *config.AILocalExecutionSource
	plans                                 *config.AILocalPlanSource
	planner                               *service.AILocalReportPlanner
	repo                                  *repository.AITaskRepo
	audit                                 *service.AIReportCallAuditService
	modelProfile, modelName, agentProfile string
}

func (g *localExecutionGuard) check(ctx context.Context) error {
	c, err := g.execution.Current(ctx)
	if err != nil {
		return err
	}
	p, err := g.plans.Current(ctx)
	if err != nil {
		return err
	}
	if c.ExpiresAtMS > p.ExpiresAtMS {
		return points.ErrInvalid
	}
	for _, plan := range p.Plans {
		if plan.ModelName != g.modelName || plan.ModelProfileDigest != g.modelProfile || plan.AgentProfileDigest != g.agentProfile || c.WorkerTimeoutMS > plan.Budget.MaxDurationMS || c.LeaseMS > plan.Budget.MaxDurationMS {
			return points.ErrIdempotency
		}
	}
	return ctx.Err()
}
func (g *localExecutionGuard) Plan(ctx context.Context, v task.Task) (service.AIL1ReportPlan, error) {
	if err := g.check(ctx); err != nil {
		return service.AIL1ReportPlan{}, err
	}
	return g.planner.Plan(ctx, v)
}
func (g *localExecutionGuard) Recheck(ctx context.Context, v task.Task, p service.AIL1ReportPlan) error {
	if err := g.check(ctx); err != nil {
		return err
	}
	return g.planner.Recheck(ctx, v, p)
}
func (g *localExecutionGuard) MemberQueueRange(ctx context.Context) (task.MemberQueueRange, error) {
	if err := g.check(ctx); err != nil {
		return task.MemberQueueRange{}, err
	}
	return g.repo.MemberQueueRange(ctx)
}
func (g *localExecutionGuard) MemberQueuePage(ctx context.Context, after, upper int64, limit int) ([]task.MemberQueueCandidate, error) {
	if err := g.check(ctx); err != nil {
		return nil, err
	}
	return g.repo.MemberQueuePage(ctx, after, upper, limit)
}
func (g *localExecutionGuard) Start(ctx context.Context, r task.StageRequest, req provider.Request) error {
	if err := g.check(ctx); err != nil {
		return err
	}
	return g.audit.Start(ctx, r, req)
}
func (g *localExecutionGuard) Finish(ctx context.Context, r task.StageRequest, out provider.Response, callErr error) error {
	// 失效也不能丢掉已返回调用的存证；后续阶段仍会重新核验配置。
	return g.audit.Finish(ctx, r, out, callErr)
}

var errLocalWorkerStop = errors.New("本地worker已收到显式停止")

func runLocalReports(parent context.Context, args []string, input io.ReadCloser, output io.Writer) int {
	stage := 1
	fail := func() int {
		_, _ = fmt.Fprintf(output, "本地报告worker未完成（阶段%d，详情已隐藏）；未知调用保留原键，不自动重发或释放。\n", stage)
		if parent != nil && parent.Err() != nil {
			return 130
		}
		return 1
	}
	if parent == nil || (len(args) != 6 && len(args) != 7) || args[0] != "-config" || args[2] != "-plans" || args[4] != "-execution" || len(args) == 7 && args[6] != "-stop-on-stdin" {
		return fail()
	}
	if parent.Err() != nil {
		return 130
	}
	stopOnInput := len(args) == 7
	if stopOnInput && input == nil {
		return fail()
	}
	runtime, err := config.ReadAILocalRuntime(args[1])
	dsn := os.Getenv("TM_DB_DSN")
	if err != nil || runtime.CheckDSN(dsn) != nil {
		return fail()
	}
	plans, err := config.NewAILocalPlanSource(args[3], runtime)
	if err != nil {
		return fail()
	}
	execution, err := config.NewAILocalExecutionSource(args[5], runtime)
	if err != nil {
		return fail()
	}
	c, err := execution.Current(parent)
	if err != nil {
		return fail()
	}
	cc := c.AgentConfig(os.Getenv("TM_AI_LOCAL_AGENT_BEARER"))
	if cc.Validate() != nil {
		return fail()
	}
	stage = 2
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fail()
	}
	defer db.Close()
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	repo := repository.NewAITaskRepo(db)
	admission, err := service.NewAILocalAdmissionControl(plans, repository.NewAILocalInstanceVerifier(db), repo)
	if err != nil {
		return fail()
	}
	startCtx, startCancel := context.WithTimeout(parent, 90*time.Second)
	defer startCancel()
	if err := admission.Ready(startCtx); err != nil {
		return fail()
	}
	stage = 3
	if err := repository.NewAIReportSchemaVerifier(db).VerifyWithDeep(startCtx); err != nil {
		return fail()
	}
	if err := repository.NewAIReportCallAuditSchemaVerifier(db).Verify(startCtx); err != nil {
		return fail()
	}
	payloads := repository.NewAIAgentRepo(db)
	if err := payloads.VerifySchema(startCtx); err != nil {
		return fail()
	}
	stage = 4
	ledger, err := service.NewPointsService(repository.NewPointsRepo(db))
	if err != nil {
		return fail()
	}
	tasks, err := service.NewAITaskService(repo, ledger)
	if err != nil {
		return fail()
	}
	planner, err := service.NewAILocalReportPlanner(admission)
	if err != nil {
		return fail()
	}
	audit, err := service.NewAIReportCallAuditService(repo)
	if err != nil {
		return fail()
	}
	guard := &localExecutionGuard{execution: execution, plans: plans, planner: planner, repo: repo, audit: audit, agentProfile: cc.Profile().Digest}
	selection, checks, model, err := service.NewAILocalReportModelControls(admission, ledger, payloads, c.ModelConfig(), guard)
	if err != nil {
		return fail()
	}
	guard.modelProfile, guard.modelName = model.DirectProfileDigest(), c.Model.Model
	if err := guard.check(startCtx); err != nil {
		return fail()
	}
	client, err := tradingcn.New(cc)
	if err != nil {
		return fail()
	}
	defer client.Close()
	agent, err := service.NewAIAgentService(payloads, client, cc.Profile())
	if err != nil {
		return fail()
	}
	var ownerID [16]byte
	if _, err := rand.Read(ownerID[:]); err != nil {
		return fail()
	}
	owner := "local_worker_" + hex.EncodeToString(ownerID[:])
	worker, err := service.NewAIL1ReportWorker(tasks, repo, ledger, agent, payloads, model, guard, selection, checks, owner, agenttask.Options{Enabled: true, Lease: c.RecoveryOptions(), PollInterval: time.Duration(c.AgentPollMS) * time.Millisecond})
	if err != nil {
		return fail()
	}
	light, err := service.NewAIL1ReportQueue(worker, guard, true)
	if err != nil {
		return fail()
	}
	deep, err := service.NewAIL2ReportQueue(worker, guard, true)
	if err != nil {
		return fail()
	}
	startCancel()
	stage = 5
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(context.Canceled)
	var readDone chan struct{}
	if stopOnInput {
		readDone = make(chan struct{})
		go func() {
			defer close(readDone)
			line, err := bufio.NewReader(io.LimitReader(input, 16)).ReadString('\n')
			if strings.TrimSpace(line) == "stop" || err == io.EOF && line == "" {
				cancel(errLocalWorkerStop)
			} else {
				cancel(points.ErrInvalid)
			}
		}()
		defer func() { input.Close(); <-readDone }()
	}
	if _, err := io.WriteString(output, "LOCAL_REPORT_WORKER_READY：本地轻深队列已就绪，限显式合成资料及回环协议服务；真实调用与生产批准未开启。\n"); err != nil {
		cancel(err)
		return 1
	}
	err = service.RunAIReportQueues(ctx, light, deep, runtime.BatchSize, time.Duration(runtime.PollIntervalMS)*time.Millisecond)
	cause := context.Cause(ctx)
	cancel(err)
	if errors.Is(cause, errLocalWorkerStop) && errors.Is(err, context.Canceled) {
		if _, err := io.WriteString(output, "本地报告worker循环已退出；在途未知结果仍须按原键核查，不代表远端已停止。\n"); err != nil {
			return 1
		}
		return 0
	}
	return fail()
}
