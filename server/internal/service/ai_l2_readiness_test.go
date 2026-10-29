package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestAIL2ReadinessChecksPlanAndCancellation(t *testing.T) {
	p, v, _ := l1PlanFixture(t)
	p.AnalysisType, p.Model.AnalysisType, p.Policy.AnalysisType, p.Policy.MinQuotes = task.DeepAnalysisType, task.DeepAnalysisType, task.DeepAnalysisType, task.DeepMinQuotes
	plan, err := NewAIL2ReportPlan(p, points.Price{Version: "v1", Points: task.DeepPricePoints}, v.Budget, "test-only-approval", "test-only-quality", "test-only-budget", "test-only-price", "test-only-source")
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewAIL2AdmissionControl(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ReadyDeep(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ready := range []func(context.Context) error{c.Ready, c.ReadyDeep} {
		if err := ready(ctx); !errors.Is(err, context.Canceled) {
			t.Fatal("已取消启动仍就绪", err)
		}
	}
	scope := task.AdmissionScope{UserID: v.UserID, AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: p.Agent.SubjectID}
	if _, err := c.Plan(ctx, scope); !errors.Is(err, context.Canceled) {
		t.Fatal("已取消请求仍生成准入", err)
	}
	originalDigest := plan.digest
	plan.digest = ""
	if err := c.ReadyDeep(context.Background()); err == nil {
		t.Fatal("摘要漂移仍就绪")
	}
	plan.digest = originalDigest
	if _, err := NewAIL2AdmissionControl(nil); err == nil {
		t.Fatal("空计划仍装配")
	}
	if err := c.Ready(nil); err == nil {
		t.Fatal("缺上下文仍就绪")
	}
	plan.approvalRef = ""
	if err := c.ReadyDeep(context.Background()); err == nil {
		t.Fatal("缺少批准引用仍就绪")
	}
	if _, err := NewAIL2AdmissionControl(plan); err == nil {
		t.Fatal("无效计划仍装配")
	}
}

func TestAIL2ExecutionChecksCancellationAndTaskBinding(t *testing.T) {
	p, v, _ := l1PlanFixture(t)
	p.AnalysisType, p.Model.AnalysisType, p.Policy.AnalysisType, p.Policy.MinQuotes = task.DeepAnalysisType, task.DeepAnalysisType, task.DeepAnalysisType, task.DeepMinQuotes
	v.AnalysisType = task.DeepAnalysisType
	plan, err := NewAIL2ReportPlan(p, points.Price{Version: "v1", Points: task.DeepPricePoints}, v.Budget, "test-only-approval", "test-only-quality", "test-only-budget", "test-only-price", "test-only-source")
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewAIL2ExecutionPlanner(plan)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	execution, err := c.Plan(ctx, v)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Recheck(ctx, v, execution); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := c.Plan(cancelled, v); !errors.Is(err, context.Canceled) {
		t.Fatal("取消后仍生成执行计划", err)
	}
	if err := c.Recheck(cancelled, v, execution); !errors.Is(err, context.Canceled) {
		t.Fatal("取消后仍允许执行复核", err)
	}
	for _, field := range []string{"user", "task", "number", "subject", "operation"} {
		t.Run(field, func(t *testing.T) {
			wrong := execution
			switch field {
			case "user":
				wrong.Agent.UserID++
			case "task":
				wrong.Agent.TaskID++
			case "number":
				wrong.Agent.TaskNo += "other"
			case "subject":
				wrong.Agent.SubjectID++
			case "operation":
				wrong.Agent.OperationKey = "other-operation"
			}
			if err := c.Recheck(ctx, v, wrong); err == nil {
				t.Fatal("异任务执行身份仍通过复核")
			}
		})
	}
	if _, err := c.Plan(nil, v); err == nil {
		t.Fatal("空上下文仍生成计划")
	}
	plan.approvalRef = ""
	if _, err := NewAIL2ExecutionPlanner(plan); err == nil {
		t.Fatal("失效模板仍装配执行器")
	}
	if err := c.Recheck(ctx, v, execution); err == nil {
		t.Fatal("模板失效后仍通过复核")
	}
}
