package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type companyTestStore struct {
	*admissionScopeTestStore
	records             []finalreport.CompanyRecord
	missing, noReader   bool
	readErr, companyErr error
	companyCalls        int
	hook                func()
}
type companyTestTx struct {
	finalreport.Tx
	store *companyTestStore
}

func (s *companyTestStore) WithinReportTx(ctx context.Context, fn func(finalreport.Tx) error) error {
	return s.reportScript.WithinReportTx(ctx, func(tx finalreport.Tx) error {
		base := admissionScopeTestTx{tx, s.record}
		if s.noReader {
			return fn(base)
		}
		return fn(companyTestTx{base, s})
	})
}
func (tx companyTestTx) ReadAdmission(ctx context.Context, v task.Task) (task.AdmissionRecord, bool, error) {
	if tx.store.readErr != nil {
		return task.AdmissionRecord{}, false, tx.store.readErr
	}
	if tx.store.missing {
		return task.AdmissionRecord{}, false, nil
	}
	return tx.Tx.ReadAdmission(ctx, v)
}
func (tx companyTestTx) ReportCompanies(ctx context.Context, id int64) ([]finalreport.CompanyRecord, error) {
	tx.store.companyCalls++
	if tx.store.hook != nil {
		tx.store.hook()
	}
	return tx.store.records, tx.store.companyErr
}
func companyServiceFixture(t *testing.T) (*AIReportService, *companyTestStore) {
	t.Helper()
	base, st, _ := reportFixture(t)
	st.v.Budget.MaxCalls = 6
	st.state.Limits = st.v.Budget.Limits()
	origin, _, _ := bindingFixture(t)
	a := origin.record.Plan.Clone()
	a.Scope = task.AdmissionScope{UserID: st.v.UserID, AnalysisType: 1, SubjectType: task.Theme, SubjectID: st.v.SubjectID}
	a.Price = points.Price{Version: st.v.PriceVersion, Points: st.v.PricePoints}
	a.Budget = st.v.Budget
	a.SubjectName, a.ModelName, a.FilingNo = st.theme, "测试模型", "test-only-filing"
	snap := st.snaps[0]
	a.Materials = []task.AdmissionMaterial{{SnapshotID: snap.ID, Digest: snap.Hash, Domain: "evidence.example.test", Category: snap.SourceCategory, URL: snap.URL, CollectedAt: snap.FetchedAt}}
	_, digest, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	record := task.AdmissionRecord{TaskID: st.v.ID, UserID: st.v.UserID, TaskNo: st.v.No, RequestHash: st.v.Hash, Digest: digest, Plan: a, CreatedAt: st.v.Deadline.Add(-time.Duration(st.v.Budget.MaxDurationMS) * time.Millisecond)}
	if err := record.Bind(st.v); err != nil {
		t.Fatal(err)
	}
	st.p.now = st.p.now.Add(time.Second)
	r := finalreport.CompanyRecord{ID: 1, NodeID: st.v.SubjectID, StockID: 1, ReviewerID: 11, NodeName: st.theme, Name: "示例题材", Symbol: "600001", TsCode: "600001.SH", MappingCode: "600001.SH", SourceType: 2, AuditStatus: 2, Status: 1, Excerpt: snap.Sentences[3].Text, URL: snap.URL, CollectedAt: snap.FetchedAt, ReviewedAt: st.p.now, Trimmed: true}
	store := &companyTestStore{admissionScopeTestStore: &admissionScopeTestStore{st, record}, records: []finalreport.CompanyRecord{r}}
	svc, err := NewAIReportService(store, base.core.ledger, base.policy, true)
	if err != nil {
		t.Fatal(err)
	}
	return svc, store
}
func TestAIReportCompaniesReadOnlyAndFailClosed(t *testing.T) {
	for _, mode := range []string{"pass", "off", "missing", "read_error", "corrupt", "no_reader", "source", "changed_theme", "company_error", "bad_company", "cancel", "terminal", "status", "deep", "future", "expired", "expires_during_read", "unknown_commit", "context_after_read", "context"} {
		t.Run(mode, func(t *testing.T) {
			s, st := companyServiceFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "off":
				s.enabled = false
			case "missing":
				st.missing = true
			case "read_error":
				st.readErr = errPointsInjected
			case "corrupt":
				st.record.Digest = "bad"
			case "no_reader":
				st.noReader = true
			case "source":
				st.evidenceError = sources.ErrUnauthorized
			case "changed_theme":
				st.theme = "另一示例题材"
			case "company_error":
				st.companyErr = errPointsInjected
			case "bad_company":
				st.records[0].ReviewerID = 0
			case "cancel":
				st.v.CancelRequested = true
			case "terminal":
				st.v.Status = task.Cancelled
			case "status":
				st.v.Status = 0
			case "deep":
				st.v.AnalysisType = 2
			case "future":
				st.v.NotBefore = st.p.now.Add(time.Second)
			case "expired":
				st.p.now = st.v.Deadline
			case "expires_during_read":
				st.hook = func() { st.p.now = st.v.Deadline }
			case "unknown_commit":
				st.failCommitAt = st.txCount + 1
			case "context_after_read":
				st.hook = cancel
			case "context":
				cancel()
			}
			before, hold, events, calls, results := st.v, st.p.hold, st.v.EventSeq, st.reviewCalls, len(st.results)
			d, err := s.PrepareCompanies(ctx, st.v.UserID, st.v.ID)
			if mode == "pass" {
				if err != nil || d == nil || len(d.Rows()) != 1 {
					t.Fatal("公司候选正例", err)
				}
			} else if err == nil || d != nil {
				t.Fatal("错误时泄露公司候选", mode, err)
			}
			if !reflect.DeepEqual(st.v, before) || st.p.hold != hold || st.v.EventSeq != events || st.reviewCalls != calls || len(st.results) != results || st.commit != nil {
				t.Fatal("只读准备改变账务/任务或触发判定")
			}
		})
	}
}
func TestAIReportCompaniesInvalidInputAndPointFaults(t *testing.T) {
	var s *AIReportService
	if d, err := s.PrepareCompanies(context.Background(), 1, 1); !errors.Is(err, points.ErrInvalid) || d != nil {
		t.Fatal(err)
	}
	s, st := companyServiceFixture(t)
	for _, ids := range [][2]int64{{0, 1}, {1, 0}, {-1, 1}} {
		if d, err := s.PrepareCompanies(context.Background(), ids[0], ids[1]); !errors.Is(err, points.ErrInvalid) || d != nil {
			t.Fatal(err)
		}
	}
	for _, ids := range [][2]int64{{999, st.v.ID}, {st.v.UserID, 999}} {
		if d, err := s.PrepareCompanies(context.Background(), ids[0], ids[1]); err == nil || d != nil {
			t.Fatal("越权读取放行")
		}
	}
	for n := 1; n <= 5; n++ {
		s, st := companyServiceFixture(t)
		st.p.calls = 0
		st.p.failAt = n
		d, err := s.PrepareCompanies(context.Background(), st.v.UserID, st.v.ID)
		if err == nil || d != nil {
			t.Fatal("账务读取故障放行", n)
		}
	}
}
