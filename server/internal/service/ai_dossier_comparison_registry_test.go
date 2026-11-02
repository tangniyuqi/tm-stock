package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestDossierComparisonRegistryGates(t *testing.T) {
	store := &registryUnitStore{}
	svc, err := NewAIDossierSupportRegistryService(store, false)
	if err != nil {
		t.Fatal(err)
	}
	var empty *AIDossierSupportRegistryService
	if digest, err := empty.RegisterComparisonInternal(nil, nil); digest != "" || err == nil {
		t.Fatal("空服务登记放行")
	}
	if digest, err := svc.RegisterComparisonInternal(context.Background(), nil); digest != "" || err == nil {
		t.Fatal("关闭登记放行")
	}
	scope := task.AdmissionScope{UserID: 1, AnalysisType: task.DeepAnalysisType, SubjectType: task.Theme, SubjectID: 7}
	if p, err := empty.PrepareComparisonForScope(context.Background(), scope, strings.Repeat("a", 64)); p != nil || err == nil {
		t.Fatal("空服务准备放行")
	}
	if p, err := svc.PrepareComparisonForScope(nil, scope, strings.Repeat("a", 64)); p != nil || err == nil {
		t.Fatal("空上下文准备放行")
	}
	if p, err := svc.PrepareComparisonForScope(context.Background(), task.AdmissionScope{}, "invalid"); p != nil || err == nil {
		t.Fatal("无效范围放行")
	}
	if store.calls != 0 {
		t.Fatal("前置拒绝仍触达仓储")
	}
	if p, err := svc.PrepareComparisonForScope(context.Background(), scope, strings.Repeat("a", 64)); p != nil || !errors.Is(err, errPointsInjected) || store.calls != 1 {
		t.Fatal("仓储失败仍泄露准备结果", err)
	}
}
