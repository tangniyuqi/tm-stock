package task

import (
	"bytes"
	"strings"
	"testing"
)

func TestDossierReportAdmissionProfile(t *testing.T) {
	base := admissionTestPlan()
	legacy, digest, err := base.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"legacy", "single_support", "six_step", "unknown", "missing_support", "budget"} {
		t.Run(mode, func(t *testing.T) {
			p := base.Clone()
			if mode != "legacy" {
				p.DossierSupportDigest = strings.Repeat("c", 64)
			}
			if mode != "legacy" && mode != "single_support" {
				p.ReportProfile = DossierReportProfile
				p.Budget.MaxCalls = 6
			}
			switch mode {
			case "unknown":
				p.ReportProfile = "unknown-profile"
			case "missing_support":
				p.DossierSupportDigest = ""
			case "budget":
				p.Budget.MaxCalls = 5
			}
			raw, hash, err := p.Encode()
			valid := mode == "legacy" || mode == "single_support" || mode == "six_step"
			if !valid {
				if err == nil || raw != nil || hash != "" {
					t.Fatal("错完整编排获准入")
				}
				return
			}
			decoded, err := DecodeAdmission(raw)
			if err != nil || decoded.ReportProfile != p.ReportProfile || mode != "legacy" && hash == digest {
				t.Fatal("编排版本未持久绑定", err)
			}
			if mode == "legacy" && !bytes.Equal(raw, legacy) {
				t.Fatal("空编排版本改变旧记录")
			}
		})
	}
}
