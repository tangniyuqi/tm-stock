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
}
