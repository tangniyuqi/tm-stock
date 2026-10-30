package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/memberauth"
)

type referenceAuthFunc func(context.Context, string) (int64, error)

func (f referenceAuthFunc) Authenticate(c context.Context, s string) (int64, error) { return f(c, s) }

type referenceTestDossierTx struct {
	dossier.Tx
	now          time.Time
	publications int
}

func (t *referenceTestDossierTx) Now(ctx context.Context) (time.Time, error) { return t.now, ctx.Err() }
func (t *referenceTestDossierTx) Member(ctx context.Context, _ int64) error  { return ctx.Err() }
func (t *referenceTestDossierTx) Publication(context.Context, int64, int64) (dossier.Approval, error) {
	t.publications++
	return dossier.Approval{}, dossier.ErrNotFound
}

type referenceTestStore struct {
	owner  conversation.Record
	rows   map[int64]dossier.ConversationReference
	core   referenceTestDossierTx
	calls  int
	commit error
	extra  bool
}
type referenceTestTx struct {
	*referenceTestStore
	rows  map[int64]dossier.ConversationReference
	write bool
}

func (s *referenceTestStore) WithinReferences(ctx context.Context, w bool, fn func(dossier.ConversationTx) error) error {
	s.calls++
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 10*time.Second {
		return dossier.ErrInvalidDraft
	}
	rows := map[int64]dossier.ConversationReference{}
	for k, v := range s.rows {
		rows[k] = v
	}
	if err := fn(&referenceTestTx{s, rows, w}); err != nil {
		return err
	}
	if s.commit == nil {
		s.rows = rows
	}
	return s.commit
}
func (t *referenceTestTx) Dossiers() dossier.Tx { return &t.core }
func (t *referenceTestTx) Conversation(_ context.Context, u, c int64, lock bool) (conversation.Record, error) {
	if u != t.owner.UserID || c != t.owner.ID || lock && !t.write {
		return conversation.Record{}, conversation.ErrNotFound
	}
	return t.owner, nil
}
func (t *referenceTestTx) Reference(_ context.Context, u, c, id int64) (dossier.ConversationReference, error) {
	r, ok := t.rows[id]
	if !ok || r.UserID != u || r.ConversationID != c {
		return dossier.ConversationReference{}, dossier.ErrNotFound
	}
	return r, nil
}
func (t *referenceTestTx) ReferenceBySelection(_ context.Context, u, c int64, s dossier.Selection) (dossier.ConversationReference, error) {
	for _, r := range t.rows {
		if r.UserID == u && r.ConversationID == c && r.Selection == s {
			return r, nil
		}
	}
	return dossier.ConversationReference{}, dossier.ErrNotFound
}
func (t *referenceTestTx) References(_ context.Context, u, c, b int64, limit int) ([]dossier.ConversationReference, error) {
	out := []dossier.ConversationReference{}
	for _, r := range t.rows {
		if r.UserID == u && r.ConversationID == c && (b == 0 || r.ID < b) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if len(out) > limit && !t.extra {
		out = out[:limit]
	}
	return out, nil
}
func (t *referenceTestTx) ReferenceCount(context.Context, int64, int64) (int, error) {
	return len(t.rows), nil
}
func (t *referenceTestTx) InsertReference(_ context.Context, r dossier.ConversationReference) (dossier.ConversationReference, error) {
	if !t.write {
		return r, dossier.ErrInvalidDraft
	}
	r.ID = int64(len(t.rows) + 1)
	t.rows[r.ID] = r
	return r, nil
}
func referenceFixture(t *testing.T) (*AIConversationDossierService, *referenceTestStore) {
	t.Helper()
	now := time.UnixMilli(1792281600000)
	m := &referenceTestStore{owner: conversation.Record{ID: 3, UserID: 1, Key: "test_reference_owner", RequestHash: conversation.CreationHash("仅测试"), Title: "仅测试", Version: 1, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}, rows: map[int64]dossier.ConversationReference{}, core: referenceTestDossierTx{now: now}}
	s, e := NewAIConversationDossierService(m, referenceAuthFunc(func(context.Context, string) (int64, error) { return 1, nil }))
	if e != nil {
		t.Fatal(e)
	}
	return s, m
}
func seedReferences(m *referenceTestStore, n int) {
	for i := 1; i <= n; i++ {
		m.rows[int64(i)] = dossier.ConversationReference{ID: int64(i), UserID: 1, ConversationID: 3, Selection: dossier.Selection{ThemeID: 7, Version: int64(i)}, CreatedAt: m.core.now}
	}
}

func TestConversationDossierInvalidAndIdentity(t *testing.T) {
	s, m := referenceFixture(t)
	ctx := context.Background()
	if _, e := NewAIConversationDossierService(nil, s.auth); e == nil {
		t.Fatal("缺存储未拒绝")
	}
	if _, e := NewAIConversationDossierService(m, nil); e == nil {
		t.Fatal("缺鉴权未拒绝")
	}
	if _, e := s.Save(ctx, "test", 1, 3, 0, dossier.Selection{ThemeID: 7, Version: 1}); e == nil {
		t.Fatal("非法会话版本未拒绝")
	}
	if _, e := s.Save(ctx, "test", 1, 3, 1, dossier.Selection{ThemeID: 7, Version: 2, CompareVersion: 1}); e == nil {
		t.Fatal("逆序未拒绝")
	}
	if _, e := s.List(ctx, "test", 1, 3, -1); e == nil {
		t.Fatal("非法游标未拒绝")
	}
	if _, e := s.Read(ctx, "test", 1, 3, 0); e == nil {
		t.Fatal("非法引用未拒绝")
	}
	if _, e := s.Receipt(ctx, "test", 1, 3, dossier.Selection{}); e == nil {
		t.Fatal("非法收据未拒绝")
	}
	if _, e := s.List(nil, "test", 1, 3, 0); e == nil {
		t.Fatal("空上下文未拒绝")
	}
	if m.calls != 0 {
		t.Fatal("非法输入进入存储")
	}
	if _, e := s.List(ctx, "test", 2, 3, 0); !errors.Is(e, memberauth.ErrUnauthorized) || m.calls != 0 {
		t.Fatal("身份不符未提前拒绝")
	}
	if _, e := s.List(ctx, "test", 1, 999, 0); !errors.Is(e, conversation.ErrNotFound) {
		t.Fatal("不存在会话未拒绝")
	}
}

func TestConversationDossierReceiptsPagingAndUnknown(t *testing.T) {
	s, m := referenceFixture(t)
	seedReferences(m, 12)
	ctx := context.Background()
	p, e := s.List(ctx, "test", 1, 3, 0)
	if e != nil || len(p.Items) != 10 || !p.HasMore || p.NextBeforeID != 3 || p.Notice != dto.AIConversationDossierNotice {
		t.Fatal("引用分页不符", e)
	}
	p, e = s.List(ctx, "test", 1, 3, p.NextBeforeID)
	if e != nil || len(p.Items) != 2 || p.HasMore || p.NextBeforeID != 0 {
		t.Fatal("引用末页不符", e)
	}
	m.owner.Archived = true
	m.owner.Version = 2
	r, e := s.Save(ctx, "test", 1, 3, 1, dossier.Selection{ThemeID: 7, Version: 1})
	if e != nil || r.ID != 1 || len(m.rows) != 12 || m.core.publications != 0 {
		t.Fatal("归档重放没有仅恢复元数据", e)
	}
	if _, e = s.Save(ctx, "test", 1, 3, 2, dossier.Selection{ThemeID: 7, Version: 99}); !errors.Is(e, conversation.ErrConflict) {
		t.Fatal("归档允许新增")
	}
	if _, e = s.Read(ctx, "test", 1, 3, 1); !errors.Is(e, dossier.ErrNotFound) || m.core.publications != 1 {
		t.Fatal("旧引用绕过当前资料读取")
	}
	m.commit = errors.New("仅测试：提交确认未知")
	if out, e := s.Receipt(ctx, "test", 1, 3, dossier.Selection{ThemeID: 7, Version: 1}); e == nil || out != (dto.AIConversationDossier{}) {
		t.Fatal("未知提交泄露预构造收据")
	}
	if out, e := s.List(ctx, "test", 1, 3, 0); e == nil || out.Items != nil {
		t.Fatal("未知提交泄露列表")
	}
	m.commit = nil
	if r, e = s.Receipt(ctx, "test", 1, 3, dossier.Selection{ThemeID: 7, Version: 1}); e != nil || r.ID != 1 {
		t.Fatal("原组合收据不能恢复", e)
	}
}

func TestConversationDossierRollbackLimitAndCorruption(t *testing.T) {
	s, m := referenceFixture(t)
	ctx := context.Background()
	if _, e := s.Save(ctx, "test", 1, 3, 2, dossier.Selection{ThemeID: 7, Version: 1}); !errors.Is(e, conversation.ErrConflict) || len(m.rows) != 0 {
		t.Fatal("旧会话版本未拒绝")
	}
	if out, e := s.Save(ctx, "test", 1, 3, 1, dossier.Selection{ThemeID: 7, Version: 1}); !errors.Is(e, dossier.ErrNotFound) || out != (dto.AIConversationDossier{}) || len(m.rows) != 0 || m.core.publications != 1 {
		t.Fatal("权限失败未回滚新引用")
	}
	seedReferences(m, 100)
	if _, e := s.Save(ctx, "test", 1, 3, 1, dossier.Selection{ThemeID: 7, Version: 999}); !errors.Is(e, dossier.ErrReferenceLimit) || len(m.rows) != 100 {
		t.Fatal("数量上限未拒绝")
	}
	if _, e := s.Save(ctx, "test", 1, 3, 1, dossier.Selection{ThemeID: 7, Version: 1}); e != nil {
		t.Fatal("上限妨碍原收据恢复", e)
	}
	m.extra = true
	if out, e := s.List(ctx, "test", 1, 3, 0); e == nil || out.Items != nil {
		t.Fatal("超量仓储结果未整组拒绝")
	}
	m.extra = false
	r := m.rows[100]
	r.CreatedAt = m.core.now.Add(time.Hour)
	m.rows[100] = r
	if out, e := s.List(ctx, "test", 1, 3, 0); e == nil || out.Items != nil {
		t.Fatal("未来时点未整组拒绝")
	}
}

func TestConversationDossierFinalIdentityClearsMetadata(t *testing.T) {
	s, m := referenceFixture(t)
	seedReferences(m, 1)
	calls := 0
	s.auth = referenceAuthFunc(func(context.Context, string) (int64, error) {
		calls++
		if calls > 1 {
			return 0, memberauth.ErrUnauthorized
		}
		return 1, nil
	})
	if out, e := s.List(context.Background(), "test", 1, 3, 0); !errors.Is(e, memberauth.ErrUnauthorized) || out.Items != nil {
		t.Fatal("末尾身份失效仍返回私有元数据")
	}
}
