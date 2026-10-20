package dto

import (
	"encoding/json"
	"testing"
)

func TestAITaskReadClosedJSONContract(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{
		{AITaskProgress{}, `{"id":0,"analysisType":0,"subjectType":"","subjectId":0,"status":0,"currentStep":0,"eventSeq":0,"cancelRequested":false,"errorCode":"","deadlineAt":0,"reportAvailable":false}`},
		{AITaskSummary{}, `{"id":0,"analysisType":0,"subjectType":"","subjectId":0,"status":0,"deadlineAt":0,"reportAvailable":false}`},
		{AITaskHistory{Items: []AITaskSummary{}}, `{"items":[],"nextBeforeId":0,"hasMore":false}`},
	} {
		b, err := json.Marshal(tc.value)
		if err != nil || string(b) != tc.want {
			t.Fatal(string(b), err)
		}
	}
}
