package quant

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestThemeStockSortField(t *testing.T) {
	var sort int32 = 10
	themeStock := ThemeStock{Sort: &sort}

	data, err := json.Marshal(themeStock)
	if err != nil {
		t.Fatalf("marshal ThemeStock failed: %v", err)
	}

	if !strings.Contains(string(data), `"sort":10`) {
		t.Fatalf("expected sort field to be serialized, got %s", string(data))
	}
}
