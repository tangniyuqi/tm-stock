package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type correctionMemory struct {
	*managementMemory
	versions       map[int64]*dossier.BoundDraft
	corrections    map[string]dossier.CorrectionEvent
	noLock         bool
	reviewExpireAt int
}

func (tx *correctionMemoryTx) Now(ctx context.Context) (time.Time, error) {
	if tx.c.reviewExpireAt > 0 {
		tx.clock++
		if tx.clock >= tx.c.reviewExpireAt {
			return tx.m.now.Add(11 * time.Minute), nil
		}
		return tx.m.now, nil
	}
	return tx.managementMemoryTx.Now(ctx)
}

type correctionMemoryTx struct {
	*managementMemoryTx
	c     *correctionMemory
	write bool
}

func (m *correctionMemory) WithinCorrection(ctx context.Context, write bool, fn func(dossier.CorrectionTx) error) error {
	next := *m
	next.corrections = map[string]dossier.CorrectionEvent{}
	for k, v := range m.corrections {
		next.corrections[k] = v
	}
	err := m.managementMemory.WithinManagement(ctx, func(tx dossier.ManagementTx) error {
		mt := tx.(*managementMemoryTx)
		next.managementMemory = mt.m
		return fn(&correctionMemoryTx{mt, &next, write})
	})
	if err == nil || m.fail == "commit_after" {
		m.corrections = next.corrections
	}
	return err
}
func (tx *correctionMemoryTx) Version(_ context.Context, theme, version int64) (dossier.StoredVersion, error) {
	if tx.m.fail == "version" {
		return dossier.StoredVersion{}, errors.New("仅测试版本故障")
	}
	v := tx.c.versions[version]
	if theme != 7 || v == nil {
		return dossier.StoredVersion{}, dossier.ErrNotFound
	}
	return v.Stored(), nil
}
func (tx *correctionMemoryTx) Grant(_ context.Context, user, theme, version int64) (dossier.Approval, error) {
	s := tx.m.permissions[dossier.ManagementObject{Kind: "member_read", UserID: user, ThemeID: theme, Version: version}]
	if s == nil {
		return dossier.Approval{}, dossier.ErrNotFound
	}
	return dossier.Approval{Digest: s.Digest, Reference: s.Reference, Enabled: s.Enabled, Until: time.UnixMilli(s.ValidUntil)}, nil
}
func (tx *correctionMemoryTx) CorrectionReadLock(_ context.Context, p dossier.CorrectionPair) (bool, error) {
	if tx.m.fail == "read_lock" {
		return false, errors.New("仅测试锁故障")
	}
	_, ok := tx.m.heads[dossier.ManagementObject{Kind: "publication", ThemeID: p.ThemeID, Version: p.AfterVersion}]
	return ok && !tx.c.noLock, nil
}
func (tx *correctionMemoryTx) LatestCorrection(_ context.Context, p dossier.CorrectionPair) (dossier.CorrectionEvent, string, error) {
	if tx.m.fail == "latest" {
		return dossier.CorrectionEvent{}, "", errors.New("仅测试当前事件故障")
	}
	var latest dossier.CorrectionEvent
	for _, v := range tx.c.corrections {
		if v.Request.Object == p && v.Revision > latest.Revision {
			latest = v
		}
	}
	if latest.Revision == 0 {
		return latest, "", dossier.ErrNotFound
	}
	_, hash, err := latest.Canonical()
	return latest, hash, err
}
func (tx *correctionMemoryTx) CorrectionEvent(_ context.Context, id string) (dossier.CorrectionEvent, error) {
	if tx.m.fail == "event" {
		return dossier.CorrectionEvent{}, errors.New("仅测试事件读取故障")
	}
	e, ok := tx.c.corrections[id]
	if !ok {
		return e, dossier.ErrNotFound
	}
	return e, nil
}
func (tx *correctionMemoryTx) AppendCorrection(_ context.Context, e dossier.CorrectionEvent) error {
	if !tx.write || tx.m.fail == "append" {
		return errors.New("仅测试审计追加失败")
	}
	raw, digest, err := e.Canonical()
	if err != nil {
		return err
	}
	restored, err := dossier.RestoreCorrectionEvent(raw, digest)
	if err != nil {
		return err
	}
	if _, ok := tx.c.corrections[e.Request.RequestID]; ok {
		return dossier.ErrConflict
	}
	tx.c.corrections[e.Request.RequestID] = restored
	return nil
}

type correctionAuth struct {
	calls, failAt int
	user          int64
}

func (a *correctionAuth) Authenticate(context.Context, string) (int64, error) {
	a.calls++
	if a.failAt > 0 && a.calls >= a.failAt {
		return 0, memberauth.ErrUnauthorized
	}
	return a.user, nil
}
func correctionServiceFixture(t *testing.T) (*AIDossierCorrectionService, *correctionMemory, dossier.CorrectionRequest, *correctionAuth) {
	t.Helper()
	manager, base, _ := managementServiceFixture(t)
	u := dossier.Field{State: "unknown"}
	line := base.snapshot.Sentences[0]
	after, err := dossier.BindDraft(dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 7, Version: 2, CreatedAt: base.now.Add(-500 * time.Millisecond).UnixMilli(), Cards: []dossier.FactCard{{Key: "sample", Section: "definition", Statement: dossier.SentenceRef{SnapshotID: 1, Hash: base.snapshot.Hash, Number: line.Number, StartByte: line.StartByte, EndByte: line.EndByte}, Subject: u, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}, []sources.Snapshot{base.snapshot})
	if err != nil {
		t.Fatal(err)
	}
	m := &correctionMemory{managementMemory: base, versions: map[int64]*dossier.BoundDraft{1: base.bound, 2: after}, corrections: map[string]dossier.CorrectionEvent{}}
	for version, bound := range m.versions {
		state := dossier.ManagementState{Digest: bound.Digest(), Reference: "test-fixture:correction-permission", Enabled: true, ValidUntil: base.now.Add(30 * time.Minute).UnixMilli()}
		pub := state
		grant := state
		m.permissions[dossier.ManagementObject{Kind: "publication", ThemeID: 7, Version: version}] = &pub
		m.permissions[dossier.ManagementObject{Kind: "member_read", UserID: 1, ThemeID: 7, Version: version}] = &grant
	}
	auth := &correctionAuth{user: 1}
	svc, err := NewAIDossierCorrectionService(m, manager, auth)
	if err != nil {
		t.Fatal(err)
	}
	r := dossier.CorrectionRequest{RequestID: "test_correction_service_001", Object: dossier.CorrectionPair{ThemeID: 7, BeforeVersion: 1, AfterVersion: 2}, BeforeDigest: base.bound.Digest(), AfterDigest: after.Digest(), Enabled: true, ValidUntil: base.now.Add(10 * time.Minute).UnixMilli(), Declarations: []dossier.CorrectionDeclaration{{Before: dossier.CorrectionLocator{Key: "sample"}, After: dossier.CorrectionLocator{Key: "sample"}, StatementKey: "sample"}}, ReviewReference: "test-fixture:synthetic-review"}
	return svc, m, r, auth
}
func TestDossierCorrectionServiceLifecycle(t *testing.T) {
	ctx := context.Background()
	t.Run("preview_review_read_replay_withdraw", func(t *testing.T) {
		svc, m, r, _ := correctionServiceFixture(t)
		v, err := svc.Inspect(ctx, 7, r.Object)
		if err != nil || v.Preview == nil || v.Revision != 0 {
			t.Fatal("双版候选预览失败", err)
		}
		out, err := svc.Read(ctx, "test", 1, r.Object)
		if err != nil || out.State != "unverified" || len(out.Items) != 0 {
			t.Fatal("空审核状态失败", err)
		}
		first, err := svc.Apply(ctx, 7, r)
		if err != nil || first.Revision != 1 {
			t.Fatal("审核失败", err)
		}
		out, err = svc.Read(ctx, "test", 1, r.Object)
		if err != nil || out.State != "reviewed_declaration" || len(out.Items) != 1 || out.Items[0].Before.Version != 1 || out.Items[0].Statement.Version != 2 {
			t.Fatal("审核原文投影失败", err)
		}
		if again, er := svc.Apply(ctx, 7, r); er != nil || again != first || len(m.corrections) != 1 {
			t.Fatal("原请求重复提交")
		}
		if receipt, er := svc.Request(ctx, 7, r.RequestID); er != nil || receipt != first {
			t.Fatal("原请求查询失败")
		}
		if _, er := svc.Request(ctx, 8, r.RequestID); !errors.Is(er, dossier.ErrNotFound) {
			t.Fatal("他人可读原收据")
		}
		m.source.Enabled = false
		v, err = svc.Inspect(ctx, 7, r.Object)
		if err != nil || v.Preview != nil {
			t.Fatal("失效来源预览未清空")
		}
		withdraw := r
		withdraw.RequestID = "test_correction_service_002"
		withdraw.ExpectedRevision = 1
		withdraw.Enabled = false
		withdraw.ValidUntil = 0
		withdraw.Declarations = []dossier.CorrectionDeclaration{}
		if _, err = svc.Apply(ctx, 7, withdraw); err != nil {
			t.Fatal("失效来源无法撤回", err)
		}
		m.source.Enabled = true
		if _, err = svc.Apply(ctx, 7, r); err != nil {
			t.Fatal("历史收据无法重读")
		}
		out, err = svc.Read(ctx, "test", 1, r.Object)
		if err != nil || len(out.Items) != 0 || out.State != "unverified" {
			t.Fatal("历史重放恢复已撤回内容")
		}
	})
	t.Run("committed_unknown_receipt", func(t *testing.T) {
		svc, m, r, _ := correctionServiceFixture(t)
		m.fail = "commit_after"
		receipt, err := svc.Apply(ctx, 7, r)
		if err == nil || receipt.RequestID != "" || len(m.corrections) != 1 {
			t.Fatal("提交未知未保留原结果")
		}
		m.fail = ""
		if _, err = svc.Request(ctx, 7, r.RequestID); err != nil {
			t.Fatal("无法找回原结果")
		}
		if _, err = svc.Apply(ctx, 7, r); err != nil || len(m.corrections) != 1 {
			t.Fatal("重放再次落库")
		}
	})
	t.Run("expired_review_is_not_no_correction", func(t *testing.T) {
		svc, m, r, _ := correctionServiceFixture(t)
		if _, err := svc.Apply(ctx, 7, r); err != nil {
			t.Fatal(err)
		}
		m.now = time.UnixMilli(r.ValidUntil)
		v, err := svc.Read(ctx, "test", 1, r.Object)
		if err != nil || v.State != "unverified" || len(v.Items) != 0 {
			t.Fatal("过期审核仍可见")
		}
	})
	t.Run("review_expires_during_read", func(t *testing.T) {
		svc, m, r, _ := correctionServiceFixture(t)
		if _, err := svc.Apply(ctx, 7, r); err != nil {
			t.Fatal(err)
		}
		m.reviewExpireAt = 5
		v, err := svc.Read(ctx, "test", 1, r.Object)
		if err != nil || v.State != "unverified" || len(v.Items) != 0 {
			t.Fatal("原授权仍有效时交付已到期声明", err)
		}
	})
	t.Run("known_fields_keep_original_context", func(t *testing.T) {
		svc, m, r, _ := correctionServiceFixture(t)
		u := dossier.Field{State: "unknown"}
		line := m.snapshot.Sentences[0]
		ref := dossier.SentenceRef{SnapshotID: 1, Hash: m.snapshot.Hash, Number: line.Number, StartByte: line.StartByte, EndByte: line.EndByte}
		for version, old := range m.versions {
			bound, err := dossier.BindDraft(dossier.Draft{Protocol: dossier.DraftVersion, ThemeID: 7, Version: version, CreatedAt: old.Identity().CreatedAt, Cards: []dossier.FactCard{{Key: "sample", Section: "definition", Statement: ref, Subject: dossier.Field{State: "known", Ref: ref}, PublishedDate: u, EffectiveDate: u, Period: u, Scope: u, Stage: u, Basis: u, Unit: u}}}, []sources.Snapshot{m.snapshot})
			if err != nil {
				t.Fatal(err)
			}
			m.versions[version] = bound
			m.permissions[dossier.ManagementObject{Kind: "publication", ThemeID: 7, Version: version}].Digest = bound.Digest()
			m.permissions[dossier.ManagementObject{Kind: "member_read", UserID: 1, ThemeID: 7, Version: version}].Digest = bound.Digest()
		}
		r.BeforeDigest = m.versions[1].Digest()
		r.AfterDigest = m.versions[2].Digest()
		r.Declarations[0].Before.Field = "subject"
		r.Declarations[0].After.Field = "subject"
		if _, err := svc.Apply(ctx, 7, r); err != nil {
			t.Fatal(err)
		}
		out, err := svc.Read(ctx, "test", 1, r.Object)
		if err != nil || len(out.Items) != 1 || out.Items[0].Before.Field != "subject" || out.Items[0].After.Field != "subject" || out.Items[0].Before.Quote.Context != line.Text || out.Items[0].Statement.Field != "" {
			t.Fatal("已知字段丢失原句或声明整句定位", err)
		}
	})
}
func TestDossierCorrectionServiceFailures(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"head", "version", "clock", "event", "latest", "append", "commit_before", "source", "digest", "revision", "unknown_field", "missing_card", "expired", "beyond_publication", "publication_withdrawn", "actor", "missing_withdraw"} {
		t.Run(name, func(t *testing.T) {
			svc, m, r, _ := correctionServiceFixture(t)
			actor := int64(7)
			switch name {
			case "head", "version", "clock", "event", "latest", "append", "commit_before":
				m.fail = name
			case "source":
				m.source.Enabled = false
			case "digest":
				r.AfterDigest = r.BeforeDigest
			case "revision":
				r.ExpectedRevision = 1
			case "unknown_field":
				r.Declarations[0].Before.Field = "stage"
			case "missing_card":
				r.Declarations[0].StatementKey = "missing"
			case "expired":
				r.ValidUntil = m.now.UnixMilli()
			case "beyond_publication":
				r.ValidUntil = m.now.Add(45 * time.Minute).UnixMilli()
			case "publication_withdrawn":
				m.permissions[dossier.ManagementObject{Kind: "publication", ThemeID: 7, Version: 1}].Enabled = false
			case "actor":
				actor = 9
			case "missing_withdraw":
				r.Enabled = false
				r.ValidUntil = 0
				r.Declarations = []dossier.CorrectionDeclaration{}
			}
			if receipt, err := svc.Apply(ctx, actor, r); err == nil || receipt.RequestID != "" || len(m.corrections) != 0 || len(m.heads) != 0 {
				t.Fatal("拒绝或失败留下部分审核", name, err)
			}
		})
	}
	for _, name := range []string{"source", "grant", "publication", "missing_head", "read_lock", "latest", "auth_before", "auth_after", "natural_expiry"} {
		t.Run("read_"+name, func(t *testing.T) {
			svc, m, r, auth := correctionServiceFixture(t)
			if _, err := svc.Apply(ctx, 7, r); err != nil {
				t.Fatal(err)
			}
			auth.calls = 0
			switch name {
			case "source":
				m.source.Enabled = false
			case "grant":
				m.permissions[dossier.ManagementObject{Kind: "member_read", UserID: 1, ThemeID: 7, Version: 1}].Enabled = false
			case "publication":
				m.permissions[dossier.ManagementObject{Kind: "publication", ThemeID: 7, Version: 2}].Enabled = false
			case "missing_head":
				m.noLock = true
			case "read_lock":
				m.fail = "read_lock"
			case "latest":
				m.fail = "latest"
			case "auth_before":
				auth.failAt = 1
			case "auth_after":
				auth.failAt = 6
			case "natural_expiry":
				m.expireAt = 5
			}
			out, err := svc.Read(ctx, "test", 1, r.Object)
			if err == nil || out.Notice != "" || len(out.Items) != 0 {
				t.Fatal("拒读泄漏原文或数量", name, err)
			}
		})
	}
}

func TestDossierCorrectionInspectAndIdentity(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"actor", "pair", "read_lock", "latest", "version", "clock", "source", "source_expiry", "missing_version", "missing_head"} {
		t.Run(name, func(t *testing.T) {
			svc, m, r, _ := correctionServiceFixture(t)
			actor := int64(7)
			unavailable := false
			switch name {
			case "actor":
				actor = 9
			case "pair":
				r.Object.AfterVersion = 1
			case "read_lock", "latest", "version", "clock":
				m.fail = name
			case "source":
				m.source.Enabled = false
				unavailable = true
			case "source_expiry":
				m.expireAt = 3
				unavailable = true
			case "missing_version":
				delete(m.versions, 2)
			case "missing_head":
				if _, err := svc.Apply(ctx, 7, r); err != nil {
					t.Fatal(err)
				}
				m.noLock = true
			}
			v, err := svc.Inspect(ctx, actor, r.Object)
			if unavailable {
				if err != nil || v.Preview != nil || v.SourceState != "unavailable" {
					t.Fatal("失效预览未清空", err)
				}
			} else if err == nil || v.Notice != "" || v.Preview != nil {
				t.Fatal("无权或故障返回候选", err)
			}
		})
	}
	t.Run("receipt_and_identity_guards", func(t *testing.T) {
		svc, m, r, auth := correctionServiceFixture(t)
		if _, err := svc.Request(ctx, 7, "short"); err == nil {
			t.Fatal("非法请求编号通过")
		}
		if _, err := svc.Request(ctx, 9, r.RequestID); err == nil {
			t.Fatal("未授权人可查收据")
		}
		if _, err := svc.Read(ctx, "test", 2, r.Object); err == nil || m.calls != 0 {
			t.Fatal("跨账号触达资料")
		}
		if _, err := svc.Read(ctx, "", 1, r.Object); err == nil {
			t.Fatal("空凭据通过")
		}
		auth.failAt = 1
		if _, err := svc.Read(ctx, "test", 1, r.Object); err == nil || m.calls != 0 {
			t.Fatal("失效凭据触达资料")
		}
		if _, err := NewAIDossierCorrectionService(nil, nil, nil); err == nil {
			t.Fatal("空依赖可装配")
		}
	})
}
