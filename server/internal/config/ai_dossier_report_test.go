package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestDossierReportLocalPlanProfile(t *testing.T) {
	for _, mode := range []string{"legacy", "six_step", "missing_profile", "unknown_profile", "missing_component", "bad_component", "budget", "null_profile", "null_component", "alias", "unknown_field", "required_zero_missing"} {
		t.Run(mode, func(t *testing.T) {
			runtime, file := localPlansFixture()
			if mode != "legacy" {
				file.Plans[0].ReportProfile = task.DossierReportProfile
				file.Plans[0].DossierSupportDigest = strings.Repeat("d", 64)
			}
			switch mode {
			case "missing_profile":
				file.Plans[0].ReportProfile = ""
			case "unknown_profile":
				file.Plans[0].ReportProfile = "unknown"
			case "missing_component":
				file.Plans[0].DossierSupportDigest = ""
			case "bad_component":
				file.Plans[0].DossierSupportDigest = "bad"
			case "budget":
				file.Plans[0].Budget.MaxCalls = 5
			}
			raw, err := json.Marshal(file)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "null_profile":
				raw = []byte(strings.Replace(string(raw), `"report_profile":"dossier-report-v1"`, `"report_profile":null`, 1))
			case "null_component":
				raw = []byte(strings.Replace(string(raw), `"dossier_support_digest":"`+strings.Repeat("d", 64)+`"`, `"dossier_support_digest":null`, 1))
			case "alias":
				raw = []byte(strings.Replace(string(raw), `"report_profile"`, `"Report_Profile"`, 1))
			case "unknown_field":
				raw = []byte(strings.Replace(string(raw), `"report_profile"`, `"extra":"x","report_profile"`, 1))
			case "required_zero_missing":
				raw = []byte(strings.Replace(string(raw), `"agent_pages":0,`, "", 1))
			}
			path := filepath.Join(t.TempDir(), "plans.json")
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			source, err := NewAILocalPlanSource(path, runtime)
			valid := mode == "legacy" || mode == "six_step"
			if (err == nil) != valid || !valid && source != nil {
				t.Fatal("新可选字段放宽旧必填或准入不完整", err)
			}
		})
	}
}
