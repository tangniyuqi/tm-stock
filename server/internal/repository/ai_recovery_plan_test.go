package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestRecoveryThemeRepositoryGuards(t *testing.T) {
	var absent *AIExtractionCatalog
	for _, r := range []*AIExtractionCatalog{absent, NewAIExtractionCatalog(nil)} {
		if value, err := r.ThemeForRecovery(context.Background(), 1); !errors.Is(err, points.ErrInvalid) || value != "" {
			t.Fatal(value, err)
		}
	}
}
