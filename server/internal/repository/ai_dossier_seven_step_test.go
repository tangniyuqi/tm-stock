package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func TestDossierSevenStepSchemaConstruction(t *testing.T) {
	var repo *AITaskRepo
	if err := repo.VerifyDossierComparisonReportSchema(context.Background()); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("空仓储核验通过", err)
	}
	if err := NewAIReportCallAuditSchemaVerifier(nil).VerifyWithComparison(context.Background()); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("无库比较核验通过", err)
	}
	var verifier *AIReportCallAuditSchemaVerifier
	if err := verifier.VerifyWithComparison(nil); !errors.Is(err, points.ErrInvalid) {
		t.Fatal("空比较核验器通过", err)
	}
}
