package task

import (
	"bytes"
	"strings"
	"testing"
)

func TestDossierSevenStepAdmission(t *testing.T) {
	legacy := admissionTestPlan()
	old, _, err := legacy.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(old, []byte("DossierComparisonDigest")) {
		t.Fatal("空比较字段改变旧编码")
	}
	base := legacy.Clone()
	base.Scope.AnalysisType = DeepAnalysisType
	base.ReportProfile = DossierComparisonReportProfile
	base.DossierSupportDigest = strings.Repeat("c", 64)
	base.DossierComparisonDigest = strings.Repeat("d", 64)
	base.Budget.MaxCalls = 7
	for _, tc := range []struct {
		name   string
		change func(*AdmissionPlan)
		valid  bool
	}{
		{"七步深度", func(*AdmissionPlan) {}, true},
		{"轻量不能比较", func(p *AdmissionPlan) { p.Scope.AnalysisType = LightAnalysisType }, false},
		{"缺字段组件", func(p *AdmissionPlan) { p.DossierSupportDigest = "" }, false},
		{"缺比较组件", func(p *AdmissionPlan) { p.DossierComparisonDigest = "" }, false},
		{"比较组件格式", func(p *AdmissionPlan) { p.DossierComparisonDigest = "invalid" }, false},
		{"五步不解释比较", func(p *AdmissionPlan) { p.ReportProfile = "" }, false},
		{"六步不解释比较", func(p *AdmissionPlan) { p.ReportProfile = DossierReportProfile }, false},
		{"六步预算不足", func(p *AdmissionPlan) { p.Budget.MaxCalls = 6 }, false},
		{"未知编排", func(p *AdmissionPlan) { p.ReportProfile = "test-only-unknown" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := base.Clone()
			tc.change(&p)
			raw, digest, err := p.Encode()
			if (err == nil) != tc.valid {
				t.Fatal("七步准入边界错误", err)
			}
			if !tc.valid {
				if raw != nil || digest != "" {
					t.Fatal("无效准入返回材料")
				}
				return
			}
			decoded, err := DecodeAdmission(raw)
			if err != nil || decoded.DossierComparisonDigest != p.DossierComparisonDigest || decoded.ReportProfile != p.ReportProfile {
				t.Fatal("新组件未持久绑定", err)
			}
		})
	}
}
