package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

func TestConversationDossierRepositoryRejectsInvalidAssembly(t *testing.T) {
	called := false
	fn := func(dossier.ConversationTx) error { called = true; return nil }
	var empty *AIConversationDossierRepo
	for _, r := range []*AIConversationDossierRepo{empty, NewAIConversationDossierRepo(nil, sql.LevelReadCommitted), NewAIConversationDossierRepo(&sql.DB{}, sql.LevelSerializable)} {
		if err := r.WithinReferences(context.Background(), false, fn); !errors.Is(err, dossier.ErrInvalidDraft) {
			t.Fatal("无效装配未提前拒绝", err)
		}
	}
	r := NewAIConversationDossierRepo(&sql.DB{}, sql.LevelReadCommitted)
	if err := r.WithinReferences(nil, false, fn); !errors.Is(err, dossier.ErrInvalidDraft) {
		t.Fatal("空上下文未拒绝")
	}
	if err := r.WithinReferences(context.Background(), false, nil); !errors.Is(err, dossier.ErrInvalidDraft) {
		t.Fatal("空回调未拒绝")
	}
	if called {
		t.Fatal("无效装配进入了回调")
	}
}
