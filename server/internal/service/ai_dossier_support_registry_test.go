package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/provider"
)

type registryUnitStore struct{ calls int }

func (s *registryUnitStore) WithinSupport(ctx context.Context, _ bool, _ func(dossier.SupportRegistryTx) error) error {
	s.calls++
	if err := ctx.Err(); err != nil {
		return err
	}
	return errPointsInjected
}

type registryUnitAdmission struct{ err error }

func (c registryUnitAdmission) Ready(ctx context.Context) error { return ctx.Err() }
func (c registryUnitAdmission) Plan(ctx context.Context, _ task.AdmissionScope) (task.AdmissionPlan, error) {
	return task.AdmissionPlan{}, ctx.Err()
}
func (c registryUnitAdmission) Recheck(ctx context.Context, _ task.AdmissionPlan) error {
	if c.err != nil {
		return c.err
	}
	return ctx.Err()
}

func TestDossierRegistryConstructionAndGates(t *testing.T) {
	_, st, old, _, r, p := newSupportWorkerFixture(t, 3, supportWorkerReply("partial", "partial_support"))
	store := &registryUnitStore{}
	registry, err := NewAIDossierSupportRegistryService(store, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := NewAIDossierSupportRegistryService(nil, true); got != nil || err == nil {
		t.Fatal("无仓储登记服务")
	}
	if digest, err := registry.RegisterInternal(context.Background(), nil); digest != "" || err == nil || store.calls != 0 {
		t.Fatal("关闭登记仍进库")
	}
	var none *AIDossierSupportRegistryService
	if digest, err := none.RegisterInternal(nil, nil); digest != "" || err == nil {
		t.Fatal("空登记服务")
	}
	for _, mode := range []string{"registry", "zero_registry", "tasks", "full", "audit", "zero_audit"} {
		t.Run(mode, func(t *testing.T) {
			r, ts, full, audit := registry, task.AdmissionStore(st), AITaskAdmissionControl(registryUnitAdmission{}), old.audit
			switch mode {
			case "registry":
				r = nil
			case "zero_registry":
				r = &AIDossierSupportRegistryService{}
			case "tasks":
				ts = nil
			case "full":
				full = nil
			case "audit":
				audit = nil
			case "zero_audit":
				audit = &AIReportCallAuditService{}
			}
			if got, err := NewAIDossierSupportExecutionControl(r, ts, full, audit); got != nil || err == nil {
				t.Fatal("缺依赖仍装配控制器")
			}
		})
	}
	var empty *AIDossierSupportExecutionControl
	if got, err := empty.ExecutionPlan(context.Background(), 1, 1); got != nil || err == nil {
		t.Fatal("空控制器提供计划")
	}
	if empty.Recheck(context.Background(), st.v, p) == nil || empty.Start(context.Background(), r, p.Request()) == nil || empty.Finish(context.Background(), r, provider.Response{}, nil) == nil {
		t.Fatal("空控制器未拒绝")
	}
}

func TestDossierRegistryExecutionRefusal(t *testing.T) {
	for _, mode := range []string{"nil_context", "cancelled", "user", "task", "cross_member", "missing_admission", "bad_record", "legacy", "full_denied", "registry_unavailable", "nil_plan", "wrong_plan"} {
		t.Run(mode, func(t *testing.T) {
			_, st, old, srv, r, p := newSupportWorkerFixture(t, 3, supportWorkerReply("partial", "partial_support"))
			store := &registryUnitStore{}
			registry, err := NewAIDossierSupportRegistryService(store, false)
			if err != nil {
				t.Fatal(err)
			}
			full := registryUnitAdmission{}
			ctx := context.Background()
			uid, id := st.v.UserID, st.v.ID
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
				st.found = false
			case "bad_record":
				st.record.Digest = strings.Repeat("e", 64)
			case "legacy":
				st.record.Plan.DossierSupportDigest = ""
				_, st.record.Digest, _ = st.record.Plan.Encode()
			case "full_denied":
				full.err = errPointsInjected
			case "nil_plan":
				p = nil
			case "wrong_plan":
				st.v.UserID++
			}
			ctl, err := NewAIDossierSupportExecutionControl(registry, st, full, old.audit)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "nil_plan" || mode == "wrong_plan" {
				if ctl.Recheck(ctx, st.v, p) == nil {
					t.Fatal("错计划通过")
				}
			} else if got, err := ctl.ExecutionPlan(ctx, uid, id); got != nil || err == nil || mode == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal("失效准入仍交付计划", err)
			}
			if srv.Count() != 0 || len(st.audits) != 0 || st.p.account.Frozen != 7 {
				t.Fatal("重建拒绝产生调用或结算")
			}
			want := 0
			if mode == "registry_unavailable" {
				want = 1
			}
			if store.calls != want {
				t.Fatal("前置失败仍访问清单", store.calls, want, r.StepNo)
			}
		})
	}
}
