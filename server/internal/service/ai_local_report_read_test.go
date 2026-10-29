package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

type localPublicationStoreFixture struct {
	*reportScript
	record task.AdmissionRecord
	found  bool
}

func (s *localPublicationStoreFixture) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	return s.reportScript.WithinReportTx(ctx, func(finalreport.Tx) error { return fn(s) })
}
func (s *localPublicationStoreFixture) ReadAdmission(ctx context.Context, _ task.Task) (task.AdmissionRecord, bool, error) {
	return s.record, s.found, ctx.Err()
}

func localPublicationFixture(t *testing.T) (*AILocalReportReadControl, *localPublicationStoreFixture, AIReportPublication, string) {
	t.Helper()
	admission, st, _, path, _ := localAdmissionFixture(t)
	plan, err := admission.Plan(context.Background(), task.AdmissionScope{UserID: 1, AnalysisType: 1, SubjectType: task.Theme, SubjectID: st.v.SubjectID})
	if err != nil {
		t.Fatal(err)
	}
	at := st.p.now.Add(time.Millisecond)
	st.v.Budget = plan.Budget
	st.v.PriceVersion = plan.Price.Version
	st.v.PricePoints = plan.Price.Points
	st.v.Deadline = at.Add(time.Duration(plan.Budget.MaxDurationMS) * time.Millisecond)
	st.v.Status = task.Succeeded
	execution, err := admission.ExecutionPlan(context.Background(), st.v)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := pendingPolicy(execution.Policy)
	if err != nil {
		t.Fatal(err)
	}
	// 只为控制器元数据正反例；不充当可由完整读取器交付的报告/模型结果。
	st.commit = &finalreport.Commit{TaskID: st.v.ID, UserID: st.v.UserID, TaskNo: st.v.No, Fence: st.v.Fence, EventSeq: st.v.EventSeq, TerminalTxnID: 1, StepNo: st.v.CurrentStep, OperationKey: "local_report_operation", InputDigest: task.StageDigest([]byte("input")), ContentDigest: task.StageDigest([]byte(`{}`)), PolicyDigest: policy.Digest(), Status: task.Succeeded, Entailment: finalreport.Pass, Semantic: finalreport.Pass, Payload: []byte(`{}`), CreatedAt: at.Add(time.Millisecond)}
	store := &localPublicationStoreFixture{reportScript: st, found: true, record: task.AdmissionRecord{TaskID: st.v.ID, UserID: st.v.UserID, TaskNo: st.v.No, RequestHash: st.v.Hash, Digest: "", Plan: plan, CreatedAt: at}}
	_, store.record.Digest, err = plan.Encode()
	if err != nil {
		t.Fatal(err)
	}
	admission.evidence = store
	control, err := NewAILocalReportReadControl(admission)
	if err != nil {
		t.Fatal(err)
	}
	p := AIReportPublication{UserID: st.v.UserID, TaskID: st.v.ID, SubjectID: st.v.SubjectID, GeneratedAt: st.commit.CreatedAt.UnixMilli(), ReportDigest: st.commit.ContentDigest, PolicyDigest: st.commit.PolicyDigest, ModelName: plan.ModelName, FilingNo: plan.FilingNo}
	return control, store, p, path
}

func TestLocalReportPublicationBoundToCurrentPlan(t *testing.T) {
	for _, mode := range []string{"正常", "历史无准入", "非成功", "错原准入", "错任务号", "错报告摘要", "错策略", "错模型", "错公示", "错生成时点", "来源撤回", "文件撤回", "取消"} {
		t.Run(mode, func(t *testing.T) {
			control, st, p, path := localPublicationFixture(t)
			ctx := context.Background()
			switch mode {
			case "历史无准入":
				st.found = false
			case "非成功":
				st.v.Status = task.Cancelled
			case "错原准入":
				st.record.Plan.Price.Points++
			case "错任务号":
				st.commit.TaskNo = "invalid"
			case "错报告摘要":
				p.ReportDigest = task.StageDigest([]byte("other"))
			case "错策略":
				st.commit.PolicyDigest = task.StageDigest([]byte("other"))
				p.PolicyDigest = st.commit.PolicyDigest
			case "错模型":
				p.ModelName = "其它模型"
			case "错公示":
				p.FilingNo = "其它公示"
			case "错生成时点":
				p.GeneratedAt++
			case "来源撤回":
				st.evidenceError = points.ErrNotFound
			case "文件撤回":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "取消":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			if err := control.Recheck(ctx, p); (err == nil) != (mode == "正常") {
				t.Fatal("本地发布控制器边界不符", err)
			}
			if mode == "正常" {
				resolved, err := control.Resolve(ctx, p.UserID, p.TaskID)
				if err != nil || resolved.Policy.Digest() != p.PolicyDigest || resolved.SubjectID != p.SubjectID {
					t.Fatal("按任务解析策略失败", err)
				}
			}
			if mode == "文件撤回" {
				_, err := control.Resolve(ctx, p.UserID, p.TaskID)
				if !errors.Is(err, finalreport.ErrUnavailable) {
					t.Fatal("撤回未隐藏报告", err)
				}
			}
		})
	}
	if _, err := NewAILocalReportReadControl(nil); err == nil {
		t.Fatal("空准入控制器未拒绝")
	}
}
