package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

func TestAIDossierReadAccessInvalid(t *testing.T) {
	for _, r := range []*AIDossierRepo{nil, {}, {db: &sql.DB{}, isolation: sql.LevelSerializable}} {
		if err := r.VerifyReadAccess(context.Background()); !errors.Is(err, dossier.ErrInvalidDraft) {
			t.Fatal("无效配置未拒绝")
		}
	}
	if err := (&AIDossierRepo{db: &sql.DB{}, isolation: sql.LevelReadCommitted}).VerifyReadAccess(nil); !errors.Is(err, dossier.ErrInvalidDraft) {
		t.Fatal("空上下文未拒绝")
	}
}
