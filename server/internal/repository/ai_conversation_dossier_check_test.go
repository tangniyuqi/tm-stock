package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

func TestConversationDossierProtectionInvalid(t *testing.T) {
	var empty *AIConversationDossierSchemaVerifier
	for _, v := range []*AIConversationDossierSchemaVerifier{empty, NewAIConversationDossierSchemaVerifier(nil)} {
		if err := v.Verify(context.Background()); !errors.Is(err, dossier.ErrInvalidDraft) {
			t.Fatal("无效准备装配未拒绝")
		}
	}
	if err := NewAIConversationDossierSchemaVerifier(&sql.DB{}).Verify(nil); !errors.Is(err, dossier.ErrInvalidDraft) {
		t.Fatal("空上下文未拒绝")
	}
	var r *AIConversationDossierRepo
	if err := r.VerifyAccess(context.Background()); !errors.Is(err, dossier.ErrInvalidDraft) {
		t.Fatal("空运行装配未拒绝")
	}
	if err := NewAIConversationDossierRepo(&sql.DB{}, sql.LevelSerializable).VerifyAccess(context.Background()); !errors.Is(err, dossier.ErrInvalidDraft) {
		t.Fatal("不支持的隔离级别未拒绝")
	}
}
func TestConversationDossierProtectionPrivacy(t *testing.T) {
	cause := errors.New("仅测试：内部SQL和私有数据")
	err := &AIConversationDossierSchemaError{Stage: 3, cause: cause}
	if strings.Contains(err.Error(), "SQL") || strings.Contains(err.Error(), "私有数据") || !errors.Is(err, cause) {
		t.Fatal("保护错误未隐藏内部数据或丢失错误链")
	}
}
