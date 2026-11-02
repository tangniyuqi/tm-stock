package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestReportCallAuditVerifierRequiresDependencies(t *testing.T) {
	if err := NewAIReportCallAuditSchemaVerifier(nil).Verify(context.Background()); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("无数据库未拒绝", err)
	}
	var v *AIReportCallAuditSchemaVerifier
	if err := v.Verify(nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("空上下文未拒绝", err)
	}
	if err := v.VerifyWithDossier(context.Background()); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("专题自检无数据库未拒绝", err)
	}
	if err := NewAIReportCallAuditSchemaVerifier(nil).VerifyWithDossier(nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("专题自检空依赖未拒绝", err)
	}
	var repo *AITaskRepo
	if err := repo.VerifyDossierReportSchema(context.Background()); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("专题自检无仓储未拒绝", err)
	}
}
