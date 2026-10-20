package dto

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestAITaskCreationDTOHasOnlyPublicImmutableFields(t *testing.T) {
	raw, err := json.Marshal(AITaskCreation{TaskID: 9, PricePoints: 7, PriceVersion: "test-v1"})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"pricePoints", "priceVersion", "taskId"}) {
		t.Fatal(string(raw))
	}
}
