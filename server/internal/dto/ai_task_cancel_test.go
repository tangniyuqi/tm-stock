package dto

import (
	"encoding/json"
	"testing"
)

func TestCancellationDTOClosedFields(t *testing.T) {
	raw, err := json.Marshal(AITaskCancellation{ID: 1, Status: 2, EventSeq: 3, CancelRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 6 {
		t.Fatal("取消字段数量改变")
	}
	for _, key := range []string{"id", "status", "eventSeq", "cancelRequested", "errorCode", "reportAvailable"} {
		if _, ok := fields[key]; !ok {
			t.Fatal("缺少取消字段", key)
		}
	}
	if string(fields["reportAvailable"]) != "false" {
		t.Fatal("伪造报告可用")
	}
}
