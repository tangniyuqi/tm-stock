package dto

import (
	"encoding/json"
	"testing"
)

func TestTaskEventPageClosedJSON(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{
		{AITaskEvent{}, `{"seq":0,"type":"","status":0,"step":0,"errorCode":"","at":0}`},
		{AITaskEventPage{Items: []AITaskEvent{}, NextAfterSeq: 3, UpperSeq: 3}, `{"items":[],"nextAfterSeq":3,"hasMore":false,"upperSeq":3}`},
	} {
		raw, err := json.Marshal(tc.value)
		if err != nil || string(raw) != tc.want {
			t.Fatal(string(raw), err)
		}
	}
}
