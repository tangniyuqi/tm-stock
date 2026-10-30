package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

func TestAIDossierSchemaInvalid(t *testing.T) {
	for _, verifier := range []*AIDossierSchemaVerifier{nil, NewAIDossierSchemaVerifier(nil)} {
		if !errors.Is(verifier.Verify(context.Background()), dossier.ErrInvalidDraft) {
			t.Fatal("无效装配未拒绝")
		}
	}
	err := &AIDossierSchemaError{Stage: 3, cause: errors.New("仅测试私有详情")}
	if strings.Contains(err.Error(), "私有详情") || errors.Unwrap(err) != err.cause {
		t.Fatal("诊断错误契约无效")
	}
}
