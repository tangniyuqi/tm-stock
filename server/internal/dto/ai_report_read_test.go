package dto

import (
	"encoding/json"
	"testing"
)

func TestAIReportReadClosedJSONContract(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{
		{AIReport{Sections: []AIReportSection{}}, `{"taskId":0,"subjectType":"","subjectId":0,"subjectName":"","generatedAt":0,"disclosure":{"aiGenerated":false,"modelName":"","filingNo":""},"sections":[]}`},
		{AIReportSection{Quotes: []AIReportQuote{}}, `{"key":"","title":"","aiGenerated":false,"quotes":[]}`},
		{AIReportQuote{}, `{"text":"","sourceType":"","sourceUrl":"","collectedAt":0}`},
		{AIReportCompany{Evidence: []AIReportCompanyEvidence{}}, `{"name":"","code":"","node":"","evidence":[]}`},
		{AIReportCompanyEvidence{}, `{"text":"","sourceType":"","sourceUrl":"","collectedAt":0,"trimmed":false}`},
	} {
		b, err := json.Marshal(tc.value)
		if err != nil || string(b) != tc.want {
			t.Fatal(string(b), err)
		}
	}
}
