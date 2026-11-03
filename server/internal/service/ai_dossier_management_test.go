package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type managementMemory struct {
	bound       *dossier.BoundDraft
	source      sources.Source
	snapshot    sources.Snapshot
	now         time.Time
	fail        string
	expireAt    int
	member      bool
	calls       int
	heads       map[dossier.ManagementObject]dossier.ManagementHead
	events      map[string]dossier.ManagementEvent
	permissions map[dossier.ManagementObject]*dossier.ManagementState
}
type managementMemoryTx struct {
	dossier.Tx
	m     *managementMemory
	clock int
}

func (m *managementMemory) WithinManagement(ctx context.Context, fn func(dossier.ManagementTx) error) error {
	m.calls++
	next := *m
	next.heads = map[dossier.ManagementObject]dossier.ManagementHead{}
	next.events = map[string]dossier.ManagementEvent{}
	next.permissions = map[dossier.ManagementObject]*dossier.ManagementState{}
	for k, v := range m.heads {
		next.heads[k] = v
	}
	for k, v := range m.events {
		next.events[k] = v
	}
	for k, v := range m.permissions {
		copy := *v
		next.permissions[k] = &copy
	}
	if err := fn(&managementMemoryTx{m: &next}); err != nil {
		return err
	}
	if m.fail == "commit_before" {
		return errors.New("仅测试提交前失败")
	}
	*m = next
	if m.fail == "commit_after" {
		return errors.New("仅测试提交确认丢失")
	}
	return ctx.Err()
}
func (tx *managementMemoryTx) Now(context.Context) (time.Time, error) {
	tx.clock++
	if tx.m.fail == "clock" {
		return time.Time{}, errors.New("仅测试时钟失败")
	}
	if tx.m.expireAt > 0 && tx.clock >= tx.m.expireAt {
		return tx.m.now.Add(2 * time.Hour), nil
	}
	return tx.m.now, nil
}
func (tx *managementMemoryTx) Member(_ context.Context, id int64) error {
	if !tx.m.member || id != 1 && id != 2 {
		return dossier.ErrNotFound
	}
	return nil
}
func (tx *managementMemoryTx) Theme(_ context.Context, id int64) error {
	if id != 7 {
		return dossier.ErrNotFound
	}
	return nil
}
func (tx *managementMemoryTx) Version(_ context.Context, theme, version int64) (dossier.StoredVersion, error) {
	if tx.m.fail == "version" {
		return dossier.StoredVersion{}, errors.New("仅测试版本读取失败")
	}
	if theme != 7 || version != 1 {
		return dossier.StoredVersion{}, dossier.ErrNotFound
	}
	return tx.m.bound.Stored(), nil
}
func (tx *managementMemoryTx) Source(_ context.Context, id int64) (sources.Source, error) {
	if id != 1 {
		return sources.Source{}, dossier.ErrNotFound
	}
	return tx.m.source, nil
}
func (tx *managementMemoryTx) Snapshot(_ context.Context, id int64) (sources.Snapshot, error) {
	if id != 1 {
		return sources.Snapshot{}, dossier.ErrNotFound
	}
	return tx.m.snapshot, nil
}
func (tx *managementMemoryTx) Publication(_ context.Context, theme, version int64) (dossier.Approval, error) {
	s := tx.m.permissions[dossier.ManagementObject{Kind: "publication", ThemeID: theme, Version: version}]
	if s == nil {
		return dossier.Approval{}, dossier.ErrNotFound
	}
	return dossier.Approval{Digest: s.Digest, Reference: s.Reference, Enabled: s.Enabled, Until: time.UnixMilli(s.ValidUntil)}, nil
}
func (tx *managementMemoryTx) ManagementHead(_ context.Context, k dossier.ManagementObject, lock bool) (dossier.ManagementHead, error) {
	if tx.m.fail == "head" {
		return dossier.ManagementHead{}, errors.New("仅测试修订读取失败")
	}
	h := tx.m.heads[k]
	if lock {
		tx.m.heads[k] = h
	}
	return h, nil
}
func (tx *managementMemoryTx) ManagementEvent(_ context.Context, id string) (dossier.ManagementEvent, error) {
	if tx.m.fail == "event" {
		return dossier.ManagementEvent{}, errors.New("仅测试原事件读取失败")
	}
	e, ok := tx.m.events[id]
	if !ok {
		return dossier.ManagementEvent{}, dossier.ErrNotFound
	}
	return e, nil
}
func (tx *managementMemoryTx) CurrentPermission(_ context.Context, k dossier.ManagementObject) (*dossier.ManagementState, error) {
	if tx.m.fail == "permission" {
		return nil, errors.New("仅测试许可读取失败")
	}
	return tx.m.permissions[k], nil
}
func (tx *managementMemoryTx) AppendManagementEvent(_ context.Context, e dossier.ManagementEvent) error {
	if tx.m.fail == "append" {
		return errors.New("仅测试审计写入失败")
	}
	if _, ok := tx.m.events[e.Request.RequestID]; ok {
		return dossier.ErrManagementConflict
	}
	tx.m.events[e.Request.RequestID] = e
	return nil
}
func (tx *managementMemoryTx) AdvanceManagementHead(_ context.Context, k dossier.ManagementObject, h dossier.ManagementHead, e dossier.ManagementEvent) error {
	if tx.m.fail == "advance" {
		return errors.New("仅测试修订提交失败")
	}
	if tx.m.heads[k] != h {
		return dossier.ErrManagementConflict
	}
	tx.m.heads[k] = dossier.ManagementHead{Revision: e.Revision, RequestID: e.Request.RequestID}
	return nil
}
func (tx *managementMemoryTx) WritePermission(_ context.Context, k dossier.ManagementObject, _ *dossier.ManagementState, s dossier.ManagementState, _ int64) error {
	if tx.m.fail == "write" {
		return errors.New("仅测试许可写入失败")
	}
	copy := s
	tx.m.permissions[k] = &copy
	return nil
}
func managementServiceFixture(t *testing.T) (*AIDossierManagementService, *managementMemory, dossier.ManagementRequest) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	until := now.Add(time.Hour)
	source := sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryOfficialList, Enabled: true, LicenseTag: "仅合成测试", LicenseRef: "test-fixture:management-unit", LicenseExpires: &until, MinInterval: time.Second}
	snap, err := sources.BuildSnapshot(source, "https://example.test/management", sources.MediaPlain, []byte("仅合成测试：专题管理资料。"), now.Add(-time.Minute), 4096)
	if err != nil {
		t.Fatal(err)
	}
	snap.ID = 1
	u := dossier.Field{State: "unknown"}
	line := snap.Sentences[0]
	bound, err := dossier.BindDraft(dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 7, Version: 1, CreatedAt: now.Add(-time.Second).UnixMilli(), Cards: []dossier.FactCard{{Key: "sample", Section: "definition", Statement: dossier.SentenceRef{SnapshotID: 1, Hash: snap.Hash, Number: line.Number, StartByte: line.StartByte, EndByte: line.EndByte}, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}, []sources.Snapshot{snap})
	if err != nil {
		t.Fatal(err)
	}
	m := &managementMemory{bound: bound, source: source, snapshot: snap, now: now, member: true, heads: map[dossier.ManagementObject]dossier.ManagementHead{}, events: map[string]dossier.ManagementEvent{}, permissions: map[dossier.ManagementObject]*dossier.ManagementState{}}
	request := dossier.ManagementRequest{RequestID: "test_management_unit_001", Object: dossier.ManagementObject{Kind: "publication", ThemeID: 7, Version: 1}, VersionDigest: bound.Digest(), Enabled: true, ValidUntil: now.Add(30 * time.Minute).UnixMilli(), ReviewReference: "test-fixture:management-unit"}
	svc, err := NewAIDossierManagementService(m, []int64{7, 8})
	if err != nil {
		t.Fatal(err)
	}
	return svc, m, request
}
func TestDossierManagementLifecycle(t *testing.T) {
	ctx := context.Background()
	t.Run("publish_replay_revoke", func(t *testing.T) {
		svc, m, r := managementServiceFixture(t)
		preview, err := svc.Inspect(ctx, 7, r.Object)
		if err != nil || preview.Preview == nil || preview.Permission != nil || preview.Managed {
			t.Fatal("未管理候选预览不符")
		}
		first, err := svc.Apply(ctx, 7, r)
		if err != nil || first.Revision != 1 {
			t.Fatal("首次发布失败", err)
		}
		same, err := svc.Apply(ctx, 7, r)
		if err != nil || same != first || len(m.events) != 1 {
			t.Fatal("发布重放产生副作用")
		}
		other := r
		other.Enabled = false
		other.ValidUntil = 0
		if _, err = svc.Apply(ctx, 7, other); !errors.Is(err, dossier.ErrManagementConflict) {
			t.Fatal("同键异体未拒绝")
		}
		if _, err = svc.Apply(ctx, 8, r); !errors.Is(err, dossier.ErrManagementConflict) {
			t.Fatal("跨操作人原请求未隔离")
		}
		if _, err = svc.Request(ctx, 8, r.RequestID); !errors.Is(err, dossier.ErrNotFound) {
			t.Fatal("原请求泄漏给其他操作人")
		}
		m.source.Enabled = false
		revoke := r
		revoke.RequestID = "test_management_unit_002"
		revoke.ExpectedRevision = 1
		revoke.Enabled = false
		revoke.ValidUntil = 0
		if _, err = svc.Apply(ctx, 7, revoke); err != nil || m.permissions[r.Object].Enabled {
			t.Fatal("资料撤回后不能撤回发布", err)
		}
		if receipt, err := svc.Apply(ctx, 7, r); err != nil || receipt != first || m.permissions[r.Object].Enabled {
			t.Fatal("旧成功回执重新启用已撤回许可")
		}
		preview, err = svc.Inspect(ctx, 7, r.Object)
		if err != nil || preview.Preview != nil || preview.SourceState != "unavailable" {
			t.Fatal("失效来源仍展示候选")
		}
		if receipt, err := svc.Request(ctx, 7, r.RequestID); err != nil || receipt != first {
			t.Fatal("原请求无法查证")
		}
	})
	t.Run("member_boundary", func(t *testing.T) {
		svc, m, r := managementServiceFixture(t)
		if _, err := svc.Apply(ctx, 7, r); err != nil {
			t.Fatal(err)
		}
		grant := r
		grant.Object.Kind = "member_read"
		grant.Object.UserID = 2
		grant.RequestID = "test_management_grant_001"
		if _, err := svc.Apply(ctx, 7, grant); err != nil {
			t.Fatal(err)
		}
		revoke := grant
		revoke.RequestID = "test_management_grant_002"
		revoke.ExpectedRevision = 1
		revoke.Enabled = false
		revoke.ValidUntil = 0
		m.member = false
		if _, err := svc.Apply(ctx, 7, revoke); err != nil {
			t.Fatal("失效会员无法撤回", err)
		}
	})
	t.Run("legacy_takeover", func(t *testing.T) {
		svc, m, r := managementServiceFixture(t)
		m.permissions[r.Object] = &dossier.ManagementState{Digest: r.VersionDigest, Reference: "legacy-review", Enabled: true, ValidUntil: r.ValidUntil}
		if _, err := svc.Apply(ctx, 7, r); err != nil || m.events[r.RequestID].Before == nil || m.events[r.RequestID].Before.Reference != "legacy-review" {
			t.Fatal("接管未保存前像", err)
		}
	})
	t.Run("unknown_commit_is_queryable", func(t *testing.T) {
		svc, m, r := managementServiceFixture(t)
		m.fail = "commit_after"
		receipt, err := svc.Apply(ctx, 7, r)
		if err == nil || receipt.RequestID != "" || len(m.events) != 1 {
			t.Fatal("提交未知被当成功或丢弃实际事件")
		}
		m.fail = ""
		confirmed, err := svc.Request(ctx, 7, r.RequestID)
		if err != nil || confirmed.Revision != 1 {
			t.Fatal("未知提交不能按原键查证")
		}
		replay, err := svc.Apply(ctx, 7, r)
		if err != nil || replay != confirmed || len(m.events) != 1 {
			t.Fatal("未知提交被重复处理")
		}
	})
}
func TestDossierManagementFailures(t *testing.T) {
	ctx := context.Background()
	for _, phase := range []string{"event", "version", "head", "permission", "clock", "append", "advance", "write", "commit_before"} {
		t.Run(phase, func(t *testing.T) {
			svc, m, r := managementServiceFixture(t)
			m.fail = phase
			receipt, err := svc.Apply(ctx, 7, r)
			if err == nil || receipt.RequestID != "" || len(m.events) != 0 || len(m.permissions) != 0 || len(m.heads) != 0 {
				t.Fatal("故障留下部分许可、修订或审计")
			}
		})
	}
	for _, kind := range []string{"revision", "digest", "source", "member", "publication", "over_publication", "missing_revoke", "metadata_tamper", "final_expiry"} {
		t.Run(kind, func(t *testing.T) {
			svc, m, r := managementServiceFixture(t)
			switch kind {
			case "revision":
				r.ExpectedRevision = 1
			case "digest":
				r.VersionDigest = strings.Repeat("b", 64)
			case "source":
				m.source.Enabled = false
			case "member":
				m.member = false
				r.Object.Kind = "member_read"
				r.Object.UserID = 2
			case "publication":
				r.Object.Kind = "member_read"
				r.Object.UserID = 2
			case "over_publication":
				m.permissions[r.Object] = &dossier.ManagementState{Digest: r.VersionDigest, Reference: "legacy", Enabled: true, ValidUntil: r.ValidUntil - 1}
				r.Object.Kind = "member_read"
				r.Object.UserID = 2
			case "missing_revoke":
				r.Enabled = false
				r.ValidUntil = 0
			case "metadata_tamper":
				m.permissions[r.Object] = &dossier.ManagementState{Digest: r.VersionDigest, Reference: dossier.ManagedApprovalPrefix + "test_missing_audit", Enabled: true, ValidUntil: r.ValidUntil}
			case "final_expiry":
				m.expireAt = 3
			}
			receipt, err := svc.Apply(ctx, 7, r)
			if err == nil || receipt.RequestID != "" || len(m.events) != 0 {
				t.Fatal("无效批准或当前状态被接受")
			}
		})
	}
	t.Run("authorization_and_cancel", func(t *testing.T) {
		svc, m, r := managementServiceFixture(t)
		if _, err := svc.Apply(ctx, 9, r); err == nil {
			t.Fatal("越权操作人被接受")
		}
		if _, err := svc.Inspect(ctx, 9, r.Object); err == nil {
			t.Fatal("越权预览")
		}
		if _, err := svc.Request(ctx, 9, r.RequestID); err == nil {
			t.Fatal("越权查证")
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := svc.Apply(cancelled, 7, r); err == nil || m.calls != 0 {
			t.Fatal("拒绝请求进入存储")
		}
		if _, err := NewAIDossierManagementService(nil, []int64{7}); err == nil {
			t.Fatal("缺存储未拒绝")
		}
		for _, ids := range [][]int64{nil, {0}, {7, 7}} {
			if _, err := NewAIDossierManagementService(m, ids); err == nil {
				t.Fatal("角色范围未校验")
			}
		}
	})
	t.Run("preview_natural_expiry", func(t *testing.T) {
		svc, m, r := managementServiceFixture(t)
		m.expireAt = 2
		view, err := svc.Inspect(ctx, 7, r.Object)
		if err != nil || view.Preview != nil || view.SourceState != "unavailable" {
			t.Fatal("预览自然过期后仍返回正文")
		}
	})
}
