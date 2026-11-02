package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestDossierSevenStepLocalConfig(t *testing.T) {
	for _, mode := range []string{"same", "missing_fields", "missing_comparison", "bad_comparison", "legacy", "six_step", "light", "budget", "null", "alias", "extra"} {
		t.Run(mode, func(t *testing.T) {
			runtime, file := localPlansFixture()
			p := &file.Plans[0]
			p.AnalysisType = task.DeepAnalysisType
			p.MinQuotes = task.DeepMinQuotes
			p.PricePoints = task.DeepPricePoints
			p.ReportProfile = task.DossierComparisonReportProfile
			p.DossierSupportDigest = strings.Repeat("d", 64)
			p.DossierComparisonDigest = strings.Repeat("e", 64)
			p.Budget.MaxCalls = 7
			switch mode {
			case "missing_fields":
				p.DossierSupportDigest = ""
			case "missing_comparison":
				p.DossierComparisonDigest = ""
			case "bad_comparison":
				p.DossierComparisonDigest = "bad"
			case "legacy":
				p.ReportProfile = ""
			case "six_step":
				p.ReportProfile = task.DossierReportProfile
			case "light":
				p.AnalysisType = task.LightAnalysisType
				p.MinQuotes = 0
				p.PricePoints = 7
			case "budget":
				p.Budget.MaxCalls = 6
			}
			raw, err := json.Marshal(file)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "null":
				raw = []byte(strings.Replace(string(raw), `"dossier_comparison_digest":"`+strings.Repeat("e", 64)+`"`, `"dossier_comparison_digest":null`, 1))
			case "alias":
				raw = []byte(strings.Replace(string(raw), `"dossier_comparison_digest"`, `"Dossier_Comparison_Digest"`, 1))
			case "extra":
				raw = []byte(strings.Replace(string(raw), `"dossier_comparison_digest"`, `"extra":"x","dossier_comparison_digest"`, 1))
			}
			path := filepath.Join(t.TempDir(), "plans.json")
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			source, err := NewAILocalPlanSource(path, runtime)
			if (err == nil) != (mode == "same") || mode != "same" && source != nil {
				t.Fatal("七步受控文件闭集或预算被放宽", err)
			}
		})
	}
}
