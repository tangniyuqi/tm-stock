package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
)

type conversationMemory struct {
	mu          sync.Mutex
	rows        map[int64]conversation.Record
	msgs        map[int64][]conversation.Message
	clock       time.Time
	err, commit error
	unknown     bool
	readWrites  int
}
type conversationMemoryTx struct {
	store *conversationMemory
	write bool
	rows  map[int64]conversation.Record
}

func conversationFixture(t *testing.T) (*AIConversationService, *conversationMemory) {
	t.Helper()
	m := &conversationMemory{rows: map[int64]conversation.Record{}, msgs: map[int64][]conversation.Message{}, clock: time.UnixMilli(1792281600000)}
	s, err := NewAIConversationService(m)
	if err != nil {
		t.Fatal(err)
	}
	return s, m
}
func (m *conversationMemory) Within(ctx context.Context, write bool, fn func(conversation.Tx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	d, ok := ctx.Deadline()
	if !ok || time.Until(d) > 5*time.Second {
		return conversation.ErrInvariant
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	rows := map[int64]conversation.Record{}
	for k, v := range m.rows {
		rows[k] = v
	}
	tx := &conversationMemoryTx{m, write, rows}
	if err := fn(tx); err != nil {
		return err
	}
	if m.commit == nil || m.unknown {
		m.rows = rows
	}
	return m.commit
}
func (t *conversationMemoryTx) check(ctx context.Context) error {
	d, ok := ctx.Deadline()
	if !ok || time.Until(d) > 5*time.Second {
		return conversation.ErrInvariant
	}
	return ctx.Err()
}
func (t *conversationMemoryTx) Now(ctx context.Context) (time.Time, error) {
	return t.store.clock, t.check(ctx)
}
func (t *conversationMemoryTx) CreateOrGet(ctx context.Context, r conversation.Record) (conversation.Record, error) {
	if err := t.check(ctx); err != nil {
		return r, err
	}
	if !t.write {
		t.store.readWrites++
		return r, conversation.ErrInvariant
	}
	for _, v := range t.rows {
		if v.UserID == r.UserID && v.Key == r.Key {
			return v, nil
		}
	}
	r.ID = int64(len(t.rows) + 1)
	t.rows[r.ID] = r
	return r, nil
}
func (t *conversationMemoryTx) ByID(ctx context.Context, uid, id int64, lock bool) (conversation.Record, error) {
	if err := t.check(ctx); err != nil {
		return conversation.Record{}, err
	}
	if lock && !t.write {
		return conversation.Record{}, conversation.ErrInvariant
	}
	r, ok := t.rows[id]
	if !ok || r.UserID != uid {
		return conversation.Record{}, conversation.ErrNotFound
	}
	return r, nil
}
func (t *conversationMemoryTx) ByKey(ctx context.Context, uid int64, key string) (conversation.Record, error) {
	if err := t.check(ctx); err != nil {
		return conversation.Record{}, err
	}
	for _, r := range t.rows {
		if r.UserID == uid && r.Key == key {
			return r, nil
		}
	}
	return conversation.Record{}, conversation.ErrNotFound
}
func (t *conversationMemoryTx) List(ctx context.Context, uid, before int64, archived bool, limit int) ([]conversation.Record, error) {
	if err := t.check(ctx); err != nil {
		return nil, err
	}
	rows := []conversation.Record{}
	for _, r := range t.rows {
		if r.UserID == uid && r.ID < before && r.Archived == archived {
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID > rows[j].ID })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}
func (t *conversationMemoryTx) Update(ctx context.Context, old, next conversation.Record) error {
	if err := t.check(ctx); err != nil {
		return err
	}
	if !t.write {
		t.store.readWrites++
		return conversation.ErrInvariant
	}
	if t.rows[old.ID].Version != old.Version {
		return conversation.ErrConflict
	}
	t.rows[old.ID] = next
	return nil
}
func (t *conversationMemoryTx) Messages(ctx context.Context, uid, id, after, through int64, limit int) ([]conversation.Message, error) {
	if err := t.check(ctx); err != nil {
		return nil, err
	}
	out := []conversation.Message{}
	for _, m := range t.store.msgs[id] {
		if m.Seq > after && m.Seq <= through {
			out = append(out, m)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (t *conversationMemoryTx) Append(context.Context, conversation.Message) error {
	return conversation.ErrInvalid
}

func TestAIConversationLifecycleAndStableReceipt(t *testing.T) {
	s, m := conversationFixture(t)
	ctx := context.Background()
	key := "conversation_create_test"
	created, err := s.Create(ctx, 42, key, "测试会话")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		r, err := s.Create(ctx, 42, key, "测试会话")
		if err != nil || r != created {
			t.Fatal(r, err)
		}
	}
	if _, err := s.Create(ctx, 42, key, "不同标题"); !errors.Is(err, conversation.ErrConflict) {
		t.Fatal(err)
	}
	v, err := s.Rename(ctx, 42, created.ID, 1, "测试改名")
	if err != nil || v.Version != 2 {
		t.Fatal(v, err)
	}
	if _, err := s.Rename(ctx, 42, created.ID, 1, "测试旧版本"); !errors.Is(err, conversation.ErrConflict) {
		t.Fatal(err)
	}
	same, err := s.Rename(ctx, 42, created.ID, 2, "测试改名")
	if err != nil || same.Version != 2 {
		t.Fatal(same, err)
	}
	r, err := s.Create(ctx, 42, key, "测试会话")
	if err != nil || r != created {
		t.Fatal("改名后创建收据变了", err)
	}
	if _, err := s.Receipt(ctx, 43, key); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Detail(ctx, 43, created.ID); !errors.Is(err, conversation.ErrNotFound) {
		t.Fatal(err)
	}
	v, err = s.Archive(ctx, 42, created.ID, 2)
	if err != nil || !v.Archived || v.Version != 3 {
		t.Fatal(v, err)
	}
	if _, err := s.Rename(ctx, 42, created.ID, 3, "归档后更改"); !errors.Is(err, conversation.ErrConflict) {
		t.Fatal(err)
	}
	if _, err := s.Archive(ctx, 42, created.ID, 2); !errors.Is(err, conversation.ErrConflict) {
		t.Fatal(err)
	}
	active, err := s.List(ctx, 42, 0, false, 20)
	if err != nil || len(active.Items) != 0 {
		t.Fatal(active, err)
	}
	archived, err := s.List(ctx, 42, 0, true, 20)
	if err != nil || len(archived.Items) != 1 || archived.Items[0].ID != created.ID {
		t.Fatal(archived, err)
	}
	history, err := s.Messages(ctx, 42, created.ID, 0, 20)
	if err != nil || history.Items == nil || len(history.Items) != 0 || history.HasMore || history.UpperSeq != 0 {
		t.Fatal(history, err)
	}
	if m.readWrites != 0 || len(m.rows) != 1 {
		t.Fatal("查询写入或重复建会话")
	}
}

func TestAIConversationPaginationAndHistoryFailClosed(t *testing.T) {
	s, m := conversationFixture(t)
	ctx := context.Background()
	for i := 1; i <= 4; i++ {
		if _, err := s.Create(ctx, 42, fmt.Sprintf("conversation_test_%02d", i), "测试会话"); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.List(ctx, 42, 0, false, 2)
	if err != nil || !p.HasMore || p.NextBeforeID != 3 || len(p.Items) != 2 {
		t.Fatal(p, err)
	}
	p, err = s.List(ctx, 42, p.NextBeforeID, false, 2)
	if err != nil || p.HasMore || p.NextBeforeID != 0 || len(p.Items) != 2 {
		t.Fatal(p, err)
	}
	r := m.rows[1]
	r.LastSeq = 3
	r.Version = 4
	m.rows[1] = r
	for seq := int64(1); seq <= 3; seq++ {
		v := conversation.MessageInput{Kind: "question", SubjectType: "theme", SubjectID: 7, Skill: "material_qa", Question: "测试用户输入，非模型回答"}
		hash, err := v.Digest(42, 1)
		if err != nil {
			t.Fatal(err)
		}
		m.msgs[1] = append(m.msgs[1], conversation.Message{ConversationID: 1, UserID: 42, Seq: seq, Key: fmt.Sprintf("conversation_message_%d", seq), RequestHash: hash, Input: v, CreatedAt: r.CreatedAt})
	}
	p1, err := s.Messages(ctx, 42, 1, 0, 2)
	if err != nil || !p1.HasMore || p1.NextAfterSeq != 2 || p1.UpperSeq != 3 {
		t.Fatal(p1, err)
	}
	p2, err := s.Messages(ctx, 42, 1, 2, 2)
	if err != nil || p2.HasMore || p2.NextAfterSeq != 3 || len(p2.Items) != 1 {
		t.Fatal(p2, err)
	}
	if _, err := s.Messages(ctx, 42, 1, 4, 2); !errors.Is(err, conversation.ErrInvalid) {
		t.Fatal(err)
	}
	for _, change := range []func(*conversation.Message){func(m *conversation.Message) { m.UserID = 43 }, func(m *conversation.Message) { m.Seq = 9 }, func(m *conversation.Message) { m.Input.Question = "伪造正文" }, func(m *conversation.Message) { m.CreatedAt = m.CreatedAt.Add(time.Hour) }} {
		old := m.msgs[1][1]
		change(&m.msgs[1][1])
		out, err := s.Messages(ctx, 42, 1, 0, 2)
		if !errors.Is(err, conversation.ErrInvariant) || out.Items != nil {
			t.Fatal("坏历史部分返回", err)
		}
		m.msgs[1][1] = old
	}
	m.msgs[1] = m.msgs[1][:2]
	if _, err := s.Messages(ctx, 42, 1, 0, 20); !errors.Is(err, conversation.ErrInvariant) {
		t.Fatal("缺消息未拒绝", err)
	}
}

func TestAIConversationRollbackUnknownAndLimits(t *testing.T) {
	s, m := conversationFixture(t)
	ctx := context.Background()
	key := "conversation_unknown_test"
	fault := errors.New("测试提交结果未确认")
	m.commit = fault
	if out, err := s.Create(ctx, 42, key, "测试会话"); err == nil || out.ID != 0 || len(m.rows) != 0 {
		t.Fatal("回滚仍返回成功")
	}
	m.unknown = true
	if out, err := s.Create(ctx, 42, key, "测试会话"); err == nil || out.ID != 0 || len(m.rows) != 1 {
		t.Fatal("未知提交不符合夹具")
	}
	m.commit = nil
	m.unknown = false
	r, err := s.Receipt(ctx, 42, key)
	if err != nil || r.ID != 1 {
		t.Fatal(r, err)
	}
	if _, err := s.Create(ctx, 42, key, "测试会话"); err != nil || len(m.rows) != 1 {
		t.Fatal(err)
	}
	for _, title := range []string{"", strings.Repeat("中", 41), " 空白", "无效\n标题"} {
		if _, err := s.Create(ctx, 42, key, title); !errors.Is(err, conversation.ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, run := range []func() error{
		func() error { _, e := s.Detail(ctx, 0, 1); return e }, func() error { _, e := s.List(ctx, 42, -1, false, 20); return e }, func() error { _, e := s.List(ctx, 42, 0, false, 101); return e }, func() error { _, e := s.Messages(ctx, 42, 1, -1, 20); return e }, func() error { _, e := s.Messages(ctx, 42, 1, 0, 0); return e }, func() error { _, e := s.Rename(ctx, 42, 1, 0, "新标题"); return e }, func() error { _, e := s.Receipt(ctx, 42, "bad"); return e },
	} {
		if !errors.Is(run(), conversation.ErrInvalid) {
			t.Fatal("坏参数放行")
		}
	}
	if _, err := NewAIConversationService(nil); err == nil {
		t.Fatal("空store")
	}
	var absent *AIConversationService
	if _, err := absent.Detail(ctx, 42, 1); err == nil {
		t.Fatal("空service")
	}
	if _, err := s.Detail(nil, 42, 1); err == nil {
		t.Fatal("空context")
	}
	m.commit = fault
	if out, err := s.Detail(ctx, 42, 1); err == nil || out.ID != 0 {
		t.Fatal("读取提交失败仍返回DTO")
	}
	m.commit = nil
	m.err = fault
	if _, err := s.Detail(ctx, 42, 1); err == nil {
		t.Fatal("丢弃存储故障")
	}
}

func TestAIConversationConcurrentCreateAndCAS(t *testing.T) {
	s, m := conversationFixture(t)
	ctx := context.Background()
	key := "conversation_parallel_test"
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Create(ctx, 42, key, "测试会话"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(m.rows) != 1 {
		t.Fatal("重复创建")
	}
	var winner int
	var mu sync.Mutex
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := s.Rename(ctx, 42, 1, 1, fmt.Sprintf("测试标题%d", i)); err == nil {
				mu.Lock()
				winner++
				mu.Unlock()
			} else if !errors.Is(err, conversation.ErrConflict) {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if winner != 1 || m.rows[1].Version != 2 {
		t.Fatal("CAS多次成功")
	}
	original := m.rows[1]
	m.clock = m.clock.Add(-time.Hour)
	next, err := s.Archive(ctx, 42, 1, 2)
	if err != nil || next.UpdatedAt != original.UpdatedAt.UnixMilli() {
		t.Fatal("时钟回拨使更新时点倒退", err)
	}
	before := map[int64]conversation.Record{}
	for k, v := range m.rows {
		before[k] = v
	}
	if _, err := s.Detail(ctx, 42, 1); err != nil || !reflect.DeepEqual(before, m.rows) {
		t.Fatal("只读写入", err)
	}
}
