package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

func TestDossierSevenStepExecutionGates(t *testing.T) {
	old, store, control, modelServer, r, _ := newSupportWorkerFixture(t, 3, supportWorkerReply("supported", "direct_support"))
	registryStore := &registryUnitStore{}
	registry, err := NewAIDossierSupportRegistryService(registryStore, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"registry", "tasks", "full", "audit"} {
		t.Run(mode, func(t *testing.T) {
			reg, tasks, full, audit := registry, task.AdmissionStore(store), AITaskAdmissionControl(registryUnitAdmission{}), control.audit
			switch mode {
			case "registry":
				reg = nil
			case "tasks":
				tasks = nil
			case "full":
				full = nil
			case "audit":
				audit = nil
			}
			if got, err := NewAIDossierComparisonExecutionControl(reg, tasks, full, audit); err == nil || got != nil {
				t.Fatal("缺控制面依赖仍提供比较执行")
			}
		})
	}
	ctl, err := NewAIDossierComparisonExecutionControl(registry, store, registryUnitAdmission{}, control.audit)
	if err != nil {
		t.Fatal(err)
	}
	if p, err := ctl.ExecutionPlan(context.Background(), store.v.UserID, store.v.ID); err == nil || p != nil || registryStore.calls != 0 {
		t.Fatal("旧准入触达比较清单")
	}
	if err := ctl.Recheck(context.Background(), store.v, nil); err == nil {
		t.Fatal("空比较计划核验通过")
	}
	var none *AIDossierComparisonExecutionControl
	if p, err := none.ExecutionPlan(nil, 0, 0); p != nil || err == nil {
		t.Fatal("空执行控制返回计划")
	}
	if none.Start(nil, r, provider.Request{}) == nil || none.Finish(nil, r, provider.Response{}, nil) == nil {
		t.Fatal("空控制转发审计")
	}
	opts := old.core.options
	for _, mode := range []string{"tasks", "store", "model", "control", "owner"} {
		t.Run("worker_"+mode, func(t *testing.T) {
			tasks, st, model, cmp, owner := old.core.tasks, taskAgentStore(store), old.core.model, AIDossierComparisonControl(ctl), old.core.owner
			switch mode {
			case "tasks":
				tasks = nil
			case "store":
				st = nil
			case "model":
				model = nil
			case "control":
				cmp = nil
			case "owner":
				owner = "bad"
			}
			if got, err := NewAIDossierComparisonWorker(tasks, st, model, cmp, owner, opts); err == nil || got != nil {
				t.Fatal("比较执行缺依赖仍装配")
			}
		})
	}
	opts.Enabled = false
	disabled, err := NewAIDossierComparisonWorker(old.core.tasks, store, nil, nil, old.core.owner, opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := disabled.RunOwned(context.Background(), r, nil); !errors.Is(err, finalreport.ErrDisabled) {
		t.Fatal("默认关闭未拒绝", err)
	}
	var empty *AIDossierComparisonWorker
	if _, err := empty.RunOwned(nil, r, nil); err == nil {
		t.Fatal("空worker执行")
	}
	active, err := NewAIDossierComparisonWorker(old.core.tasks, store, old.core.model, ctl, old.core.owner, old.core.options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := active.RunOwned(context.Background(), r, (*dossier.CardComparisonPlan)(nil)); err == nil {
		t.Fatal("旧步位或空计划执行")
	}
	if modelServer.Count() != 0 {
		t.Fatal("前置拒绝仍调用模型")
	}
}

func TestDossierSevenStepControlRefusals(t *testing.T) {
	for _, mode := range []string{"nil_context", "cancelled", "user", "task", "cross_member", "missing_admission", "bad_record", "legacy", "full_denied", "registry_failed"} {
		t.Run(mode, func(t *testing.T) {
			_, store, old, srv, _, _ := newSupportWorkerFixture(t, 3, supportWorkerReply("supported", "direct_support"))
			// 仅构造格式正确的隔离准入；本组在完整控制面或仓储拒绝，不授予模型执行权。
			store.v.AnalysisType = task.DeepAnalysisType
			store.v.Budget.MaxCalls = 7
			store.record.Plan.Scope.AnalysisType = task.DeepAnalysisType
			store.record.Plan.Budget = store.v.Budget
			store.record.Plan.ReportProfile = task.DossierComparisonReportProfile
			store.record.Plan.DossierComparisonDigest = strings.Repeat("e", 64)
			var err error
			_, store.record.Digest, err = store.record.Plan.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if err = store.record.Bind(store.v); err != nil {
				t.Fatal(err)
			}
			registryStore := &registryUnitStore{}
			registry, err := NewAIDossierSupportRegistryService(registryStore, false)
			if err != nil {
				t.Fatal(err)
			}
			full := registryUnitAdmission{}
			ctx := context.Background()
			uid, id := store.v.UserID, store.v.ID
			switch mode {
			case "nil_context":
				ctx = nil
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "user":
				uid = 0
			case "task":
				id = 0
			case "cross_member":
				uid++
			case "missing_admission":
				store.found = false
			case "bad_record":
				store.record.Digest = "bad"
			case "legacy":
				store.record.Plan.ReportProfile = task.DossierReportProfile
				store.record.Plan.DossierComparisonDigest = ""
				_, store.record.Digest, err = store.record.Plan.Encode()
				if err != nil {
					t.Fatal(err)
				}
			case "full_denied":
				full.err = errPointsInjected
			}
			ctl, err := NewAIDossierComparisonExecutionControl(registry, store, full, old.audit)
			if err != nil {
				t.Fatal(err)
			}
			if p, err := ctl.ExecutionPlan(ctx, uid, id); err == nil || p != nil {
				t.Fatal("拒绝边界仍返回执行输入")
			}
			want := 0
			if mode == "registry_failed" {
				want = 1
			}
			if registryStore.calls != want || srv.Count() != 0 {
				t.Fatal("拒绝后越过依赖边界或调用模型", registryStore.calls)
			}
		})
	}
}
