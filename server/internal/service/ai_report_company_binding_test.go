package service

import (
	"context"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 只验证当前读公司绑定辅助核；完整会员执行计划另由真库HTTP主链验证。
func boundCompanyServiceFixture(t *testing.T) (*AIReportService, *companyTestStore) {
	t.Helper()
	s, st := companyServiceFixture(t)
	d, err := s.PrepareCompanies(context.Background(), st.v.UserID, st.v.ID)
	if err != nil {
		t.Fatal(err)
	}
	c := finalreport.Config{Ref: "test-only-policy", ModelName: st.record.Plan.ModelName, FilingNo: st.record.Plan.FilingNo, EntailmentVersion: "test-only-entailment", SemanticVersion: "test-only-semantic", AllowedDomains: []string{"evidence.example.test"}, CompanyDigest: d.ContentDigest()}
	s.policy, err = finalreport.NewCompanyPolicy(c, d, func(context.Context, finalreport.EntailmentInput) (finalreport.Verdict, error) {
		return finalreport.Uncertain, nil
	}, func(context.Context, string) (finalreport.Verdict, error) { return finalreport.Uncertain, nil })
	if err != nil {
		t.Fatal(err)
	}
	st.record.Plan.CompanyDigest = d.ContentDigest()
	_, st.record.Digest, err = st.record.Plan.Encode()
	if err != nil {
		t.Fatal(err)
	}
	st.p.calls = 0
	st.txCount = 0
	if err := st.record.Bind(st.v); err != nil {
		t.Fatal("辅助夹具准入损坏", err)
	}
	if _, err := finalreport.NewCompanyDraft(s.policy, st.record.Plan, st.snaps, st.records, st.p.now); err != nil {
		t.Fatal("辅助夹具公司绑定损坏", err)
	}
	return s, st
}

func TestAIReportCompanyCurrentBinding(t *testing.T) {
	for _, mode := range []string{"pass", "source", "admission", "missing", "corrupt", "reader", "company_error", "company_changed", "theme", "expired", "terminal"} {
		t.Run(mode, func(t *testing.T) {
			s, st := boundCompanyServiceFixture(t)
			switch mode {
			case "source":
				st.evidenceError = sources.ErrUnauthorized
			case "admission":
				st.readErr = errPointsInjected
			case "missing":
				st.missing = true
			case "corrupt":
				st.record.Digest = "bad"
			case "reader":
				st.noReader = true
			case "company_error":
				st.companyErr = errPointsInjected
			case "company_changed":
				st.records[0].ReviewerID++
			case "theme":
				st.theme = "其他题材"
			case "expired":
				st.p.now = st.v.Deadline
			case "terminal":
				st.v.Status = task.Succeeded
				st.p.now = st.v.Deadline
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := st.WithinReportTx(ctx, func(tx finalreport.Tx) error {
				_, _, err := s.reportEvidence(ctx, tx, st.v, []int64{st.snaps[0].ID})
				return err
			})
			if (mode == "pass" || mode == "terminal") != (err == nil) {
				t.Fatal("当前公司绑定判定错误", mode, err)
			}
			if st.commit != nil || st.reviewCalls != 0 {
				t.Fatal("当前读改变报告或调用判定")
			}
		})
	}
}
