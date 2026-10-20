package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 仅测试首次范围门禁；不代表完整会员L1交付或真实多Agent/判定质量证明。
type admissionScopeTestStore struct {
	*reportScript
	record task.AdmissionRecord
}
type admissionScopeTestTx struct {
	finalreport.Tx
	record task.AdmissionRecord
}

func (s *admissionScopeTestStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	return s.reportScript.WithinReportTx(ctx, func(tx finalreport.Tx) error { return fn(admissionScopeTestTx{tx, s.record}) })
}
func (tx admissionScopeTestTx) ReadAdmission(_ context.Context, v task.Task) (task.AdmissionRecord, bool, error) {
	return tx.record, true, tx.record.Bind(v)
}
func TestAIReportAdmissionScopeBinding(t *testing.T) {
	for _, mode := range []string{"same", "subset", "model", "filing", "theme", "materials", "hash", "url", "category", "collected"} {
		t.Run(mode, func(t *testing.T) {
			base, st, request := reportFixture(t)
			// 原夹具是旧内部任务；在本隔离夹具中建立完整、规范且可Bind的会员记录。
			st.v.Budget.MaxCalls = 6
			st.state.Limits = st.v.Budget.Limits()
			origin, _, _ := bindingFixture(t)
			plan := origin.record.Plan.Clone()
			plan.Scope = task.AdmissionScope{UserID: st.v.UserID, AnalysisType: st.v.AnalysisType, SubjectType: st.v.SubjectType, SubjectID: st.v.SubjectID}
			plan.Price = points.Price{Version: st.v.PriceVersion, Points: st.v.PricePoints}
			plan.Budget = st.v.Budget
			plan.SubjectName = st.theme
			plan.ModelName = "测试模型"
			plan.FilingNo = "test-only-filing"
			snap := st.snaps[0]
			plan.Materials = []task.AdmissionMaterial{{SnapshotID: snap.ID, Digest: snap.Hash, Domain: "evidence.example.test", Category: snap.SourceCategory, URL: snap.URL, CollectedAt: snap.FetchedAt}}
			if mode == "model" {
				plan.ModelName = "另一个已批准测试模型"
			}
			if mode == "filing" {
				plan.FilingNo = "other-test-only-filing"
			}
			if mode == "theme" {
				plan.SubjectName = "另一示例题材"
			}
			if mode == "materials" {
				plan.Materials[0].SnapshotID = 999
			}
			if mode == "hash" {
				plan.Materials[0].Digest = strings.Repeat("f", 64)
			}
			if mode == "url" {
				plan.Materials[0].URL = "https://evidence.example.test/other"
			}
			if mode == "category" {
				plan.Materials[0].Category = "announcement"
			}
			if mode == "collected" {
				plan.Materials[0].CollectedAt = plan.Materials[0].CollectedAt.Add(-time.Millisecond)
			}
			if mode == "subset" {
				extra := plan.Materials[0]
				extra.SnapshotID = 999
				plan.Materials = append(plan.Materials, extra)
			}
			_, digest, err := plan.Encode()
			if err != nil {
				t.Fatal("隔离夹具准入无效", err)
			}
			record := task.AdmissionRecord{TaskID: st.v.ID, UserID: st.v.UserID, TaskNo: st.v.No, RequestHash: st.v.Hash, Digest: digest, Plan: plan, CreatedAt: st.v.NotBefore}
			if err := record.Bind(st.v); err != nil {
				t.Fatal("记录未通过既有完整绑定", err)
			}
			store := &admissionScopeTestStore{st, record}
			svc, err := NewAIReportService(store, base.core.ledger, base.policy, true)
			if err != nil {
				t.Fatal(err)
			}
			selection, err := finalreport.NewSelectionStage(base.policy, st.v.No, st.v.SubjectID, st.theme, st.snaps)
			if err != nil {
				t.Fatal(err)
			}
			selectionErr := checkSelectionAdmissionScope(context.Background(), admissionScopeTestTx{st, record}, st.v, selection)
			if (mode == "same" || mode == "subset") != (selectionErr == nil) {
				t.Fatal("转换范围校验不符", mode, selectionErr)
			}
			out, err := svc.Finalize(context.Background(), request)
			if mode == "same" || mode == "subset" {
				if err != nil || out.Status != task.Succeeded {
					t.Fatal("同范围正例失败", out, err)
				}
				// 模拟旧缺口已生成的历史档案：当前存储提供可Bind但与报告不同的原批准范围。
				hold, terminal, calls := st.p.hold, st.pt.terminal, st.reviewCalls
				store.record.Plan.ModelName = "不同的原批准测试模型"
				_, store.record.Digest, err = store.record.Plan.Encode()
				if err != nil {
					t.Fatal(err)
				}
				control := &reportReadControl{st: st}
				reader, err := NewAIReportReadService(svc, control, true)
				if err != nil {
					t.Fatal(err)
				}
				report, err := reader.Read(context.Background(), st.v.UserID, st.v.ID)
				if !errors.Is(err, points.ErrIdempotency) || report.TaskID != 0 || len(report.Sections) != 0 || control.calls != 0 {
					t.Fatal("历史范围不符仍出正文", err)
				}
				if replay, err := svc.Finalize(context.Background(), request); !errors.Is(err, points.ErrIdempotency) || replay != (finalreport.Receipt{}) {
					t.Fatal("历史范围不符仍重放成功", replay, err)
				}
				if st.p.hold != hold || st.pt.terminal != terminal || st.reviewCalls != calls {
					t.Fatal("历史拒读撤销结算或补调用")
				}
				return
			}
			if err == nil || out != (finalreport.Receipt{}) {
				t.Fatalf("报告越过首次准入%s范围仍被保存/结算 status=%d err=%v", mode, out.Status, err)
			}
			if st.commit != nil || st.p.hold.State != points.Holding || st.v.Status != task.Running || st.reviewCalls != 0 {
				t.Fatal("范围拒绝改变报告/冻结或触发判定")
			}
		})
	}
}
