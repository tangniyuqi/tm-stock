package service

import (
	"context"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestReportCheckProtocolLocalAdmission(t *testing.T) {
	for _, protocol := range []string{"", finalreport.PairedCheckProtocol} {
		name := "legacy"
		if protocol != "" {
			name = "paired"
		}
		t.Run(name, func(t *testing.T) {
			c, st, _, _, _ := localAdmissionFixture(t, protocol)
			ctx := context.Background()
			if err := c.Ready(ctx); err != nil {
				t.Fatal(err)
			}
			for _, kind := range []int{1, 2} {
				admission, err := c.Plan(ctx, task.AdmissionScope{UserID: 1, AnalysisType: kind, SubjectType: task.Theme, SubjectID: st.v.SubjectID})
				if err != nil {
					t.Fatal(err)
				}
				v := st.v
				v.AnalysisType, v.Budget, v.PricePoints, v.PriceVersion = kind, admission.Budget, admission.Price.Points, admission.Price.Version
				plan, err := c.ExecutionPlan(ctx, v)
				if err != nil || plan.Model.CheckProtocol != protocol {
					t.Fatal("准入或执行丢失明确判定协议")
				}
				ent, err := finalreport.CheckPolicyVersion(finalreport.CheckEntailment, plan.Model)
				if err != nil || ent != plan.Policy.EntailmentVersion {
					t.Fatal("实际策略未绑定批准协议")
				}
				plan.Model.CheckProtocol = finalreport.PairedCheckProtocol
				if protocol != "" {
					plan.Model.CheckProtocol = ""
				}
				changed, err := plan.AdmissionDigest()
				if err == nil && changed == admission.ExecutionDigest {
					t.Fatal("协议被替换仍可复用原任务摘要")
				}
			}
		})
	}
}
