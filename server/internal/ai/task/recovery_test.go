package task

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRecoveryOptionsAndPlanGuards(t *testing.T) {
	good := RecoveryOptions{Lease: time.Second, Heartbeat: 100 * time.Millisecond, Timeout: time.Second}
	if good.Validate() != nil {
		t.Fatal("有效恢复期限被拒绝")
	}
	for _, bad := range []RecoveryOptions{{}, {Lease: time.Second, Heartbeat: time.Second, Timeout: time.Second}, {Lease: time.Second, Heartbeat: time.Millisecond, Timeout: time.Second}, {Lease: time.Second + time.Nanosecond, Heartbeat: 100 * time.Millisecond, Timeout: time.Second}, {Lease: time.Second, Heartbeat: 100 * time.Millisecond, Timeout: time.Hour}} {
		if bad.Validate() == nil {
			t.Fatal("无效恢复期限被放行")
		}
	}
	v := Task{ID: 1, UserID: 2, No: "fixed-task", CurrentStep: 0}
	step := RecoveryStep{OperationKey: "recovery_operation_0001", Stage: "extract", InputDigest: strings.Repeat("a", 64), Policy: StagePolicy{Digest: strings.Repeat("b", 64), Check: func(ctx context.Context, _ StageValidationInput) error { return ctx.Err() }}, Recheck: func(ctx context.Context) error { return ctx.Err() }}
	p := RecoveryPlan{TaskID: 1, UserID: 2, TaskNo: v.No, Steps: []RecoveryStep{step}}
	if p.Validate(v) != nil {
		t.Fatal("有效恢复计划被拒绝")
	}
	for _, mutate := range []func(*RecoveryPlan){func(p *RecoveryPlan) { p.TaskID++ }, func(p *RecoveryPlan) { p.UserID++ }, func(p *RecoveryPlan) { p.TaskNo = "other" }, func(p *RecoveryPlan) { p.Steps = nil }, func(p *RecoveryPlan) { p.Steps = append(p.Steps, p.Steps[0]) }, func(p *RecoveryPlan) { p.Steps[0].OperationKey = "short" }, func(p *RecoveryPlan) { p.Steps[0].Stage = "other" }, func(p *RecoveryPlan) { p.Steps[0].InputDigest = "bad" }, func(p *RecoveryPlan) { p.Steps[0].Policy = StagePolicy{} }, func(p *RecoveryPlan) { p.Steps[0].Recheck = nil }} {
		bad := p
		bad.Steps = append([]RecoveryStep(nil), p.Steps...)
		mutate(&bad)
		if bad.Validate(v) == nil {
			t.Fatal("无效/重复恢复计划被放行")
		}
	}
	v.CurrentStep = 2
	if p.Validate(v) == nil {
		t.Fatal("截短恢复前缀被放行")
	}
}
