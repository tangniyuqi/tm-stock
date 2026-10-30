package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
)

type dossierUnusedStore struct{ calls int }

func (s *dossierUnusedStore) Within(context.Context, bool, func(dossier.Tx) error) error {
	s.calls++
	return errors.New("不应进入存储")
}

type dossierUnusedAuth struct{ calls int }

func (a *dossierUnusedAuth) Authenticate(context.Context, string) (int64, error) {
	a.calls++
	return 0, errors.New("不应鉴权")
}

func TestDossierInvalidRequestsDoNotReachStorage(t *testing.T) {
	store, auth := &dossierUnusedStore{}, &dossierUnusedAuth{}
	if _, e := NewAIDossierService(nil, auth, false); e == nil {
		t.Fatal("缺存储未拒绝")
	}
	if _, e := NewAIDossierService(store, nil, false); e == nil {
		t.Fatal("缺身份核验未拒绝")
	}
	svc, e := NewAIDossierService(store, auth, false)
	if e != nil {
		t.Fatal(e)
	}
	for _, versions := range [][]int64{nil, {0}, {2, 1}, {1, 1}, {1, 2, 3}} {
		if out, e := svc.ReadVersions(context.Background(), "仅测试", 1, 7, versions); !errors.Is(e, dossier.ErrInvalidDraft) || out != nil {
			t.Fatal("非法版本查询未拒绝")
		}
	}
	if out, e := svc.ReadVersions(nil, "仅测试", 1, 7, []int64{1}); e == nil || out != nil {
		t.Fatal("空上下文未拒绝")
	}
	if _, e := svc.StageInternal(context.Background(), dossier.Draft{ThemeID: 7, Version: 1}); e == nil {
		t.Fatal("只读服务启用了候选写入")
	}
	var empty *AIDossierService
	if _, e := empty.StageInternal(context.Background(), dossier.Draft{}); e == nil {
		t.Fatal("空服务未拒绝")
	}
	if _, e := empty.ReadVersions(context.Background(), "仅测试", 1, 7, []int64{1}); e == nil {
		t.Fatal("空服务未拒绝")
	}
	if store.calls != 0 || auth.calls != 0 {
		t.Fatal("非法请求触达了依赖")
	}
}
