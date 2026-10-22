package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

// 仅测试夹具：会话元数据事务可回滚，原任务独立提交，禁止把创建器放进元数据事务。
type intentMemory struct {
	base                      *conversationMemory
	intents                   map[int64]conversation.Intent
	tasks                     map[string]conversation.TaskIdentity
	failState                 string
	commitUnknown, failAppend bool
}
type intentMemoryTx struct {
	*conversationMemoryTx
	m       *intentMemory
	intents map[int64]conversation.Intent
	msgs    map[int64][]conversation.Message
}

func (m *intentMemory) WithinIntents(ctx context.Context, write bool, fn func(conversation.IntentTx) error) error {
	m.base.mu.Lock()
	defer m.base.mu.Unlock()
	rows := map[int64]conversation.Record{}
	for k, v := range m.base.rows {
		rows[k] = v
	}
	intents := map[int64]conversation.Intent{}
	for k, v := range m.intents {
		intents[k] = v
	}
	msgs := map[int64][]conversation.Message{}
	for k, v := range m.base.msgs {
		msgs[k] = append([]conversation.Message(nil), v...)
	}
	tx := &intentMemoryTx{&conversationMemoryTx{m.base, write, rows}, m, intents, msgs}
	if err := tx.check(ctx); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	fail := false
	for k, v := range intents {
		if v.State == m.failState && m.intents[k].State != v.State {
			fail = true
		}
	}
	if !fail || m.commitUnknown {
		m.base.rows = rows
		m.intents = intents
		m.base.msgs = msgs
	}
	if fail {
		m.failState = ""
		return errors.New("测试事务提交结果未知")
	}
	return nil
}
func (t *intentMemoryTx) IntentByID(ctx context.Context, uid, cid, id int64, lock bool) (conversation.Intent, error) {
	if err := t.check(ctx); err != nil {
		return conversation.Intent{}, err
	}
	i, ok := t.intents[id]
	if !ok || i.UserID != uid || i.ConversationID != cid {
		return conversation.Intent{}, conversation.ErrNotFound
	}
	return i, nil
}
func (t *intentMemoryTx) IntentByKey(ctx context.Context, uid, cid int64, key string) (conversation.Intent, error) {
	for _, i := range t.intents {
		if i.UserID == uid && i.ConversationID == cid && i.Key == key {
			return i, t.check(ctx)
		}
	}
	return conversation.Intent{}, conversation.ErrNotFound
}
func (t *intentMemoryTx) InsertIntent(ctx context.Context, i conversation.Intent) (conversation.Intent, error) {
	if !t.write {
		return i, conversation.ErrInvariant
	}
	if old, err := t.IntentByKey(ctx, i.UserID, i.ConversationID, i.Key); err == nil {
		return old, nil
	}
	i.ID = int64(len(t.intents) + 1)
	if !i.Valid() {
		return i, conversation.ErrInvariant
	}
	t.intents[i.ID] = i
	return i, nil
}
func (t *intentMemoryTx) SetIntent(ctx context.Context, old, next conversation.Intent) error {
	if !t.write || !next.Valid() || t.intents[old.ID].Version != old.Version {
		return conversation.ErrInvariant
	}
	t.intents[old.ID] = next
	return t.check(ctx)
}
func (t *intentMemoryTx) Intents(ctx context.Context, uid, cid, before int64, limit int) ([]conversation.Intent, error) {
	out := []conversation.Intent{}
	for _, i := range t.intents {
		if i.UserID == uid && i.ConversationID == cid && i.ID < before {
			out = append(out, i)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, t.check(ctx)
}
func (t *intentMemoryTx) IntentTask(ctx context.Context, uid int64, key string, lock bool) (conversation.TaskIdentity, error) {
	v, ok := t.m.tasks[key]
	if !ok || v.UserID != uid {
		return v, conversation.ErrNotFound
	}
	return v, t.check(ctx)
}
func (t *intentMemoryTx) Append(ctx context.Context, msg conversation.Message) error {
	if t.m.failAppend {
		return errors.New("测试消息写入失败")
	}
	if !t.write || !msg.Valid() {
		return conversation.ErrInvariant
	}
	t.msgs[msg.ConversationID] = append(t.msgs[msg.ConversationID], msg)
	return t.check(ctx)
}
func (t *intentMemoryTx) Messages(ctx context.Context, uid, cid, after, through int64, limit int) ([]conversation.Message, error) {
	out := []conversation.Message{}
	for _, m := range t.msgs[cid] {
		if m.UserID == uid && m.Seq > after && m.Seq <= through {
			out = append(out, m)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, t.check(ctx)
}

type intentCreator struct {
	m                         *intentMemory
	quotes, creates, receipts atomic.Int64
	points                    int64
	cause, receiptError       error
	commit                    bool
}

func (c *intentCreator) Quote(ctx context.Context, uid int64, in dto.AITaskQuoteInput) (dto.AITaskQuote, error) {
	c.quotes.Add(1)
	c.m.base.mu.Lock()
	c.m.base.mu.Unlock()
	return dto.AITaskQuote{AnalysisType: in.AnalysisType, SubjectType: in.SubjectType, SubjectID: in.SubjectID, PriceVersion: "test-v1", PricePoints: c.points}, nil
}
func (c *intentCreator) CreateQuoted(ctx context.Context, uid int64, key string, in dto.AITaskCreateInput, expected int64) (dto.AITaskCreation, error) {
	c.creates.Add(1)
	c.m.base.mu.Lock()
	defer c.m.base.mu.Unlock()
	if expected != c.points {
		return dto.AITaskCreation{}, points.ErrPriceChanged
	}
	if c.commit {
		level := 1
		if in.AnalysisType == "deep" {
			level = 2
		}
		hash, err := points.RequestHash("ai_task_create", task.CreateRequest{UserID: uid, Key: key, AnalysisType: level, SubjectType: in.SubjectType, SubjectID: in.SubjectID, PriceVersion: in.PriceVersion})
		if err != nil {
			return dto.AITaskCreation{}, err
		}
		c.m.tasks[key] = conversation.TaskIdentity{ID: 40, UserID: uid, Key: key, RequestHash: hash, AnalysisType: level, SubjectType: in.SubjectType, SubjectID: in.SubjectID, PriceVersion: in.PriceVersion, PricePoints: expected}
	}
	return dto.AITaskCreation{}, c.cause
}
func (c *intentCreator) Receipt(ctx context.Context, uid int64, key string) (dto.AITaskCreation, error) {
	c.receipts.Add(1)
	c.m.base.mu.Lock()
	defer c.m.base.mu.Unlock()
	if c.receiptError != nil {
		return dto.AITaskCreation{}, c.receiptError
	}
	v, ok := c.m.tasks[key]
	if !ok || v.UserID != uid {
		return dto.AITaskCreation{}, points.ErrNotFound
	}
	return dto.AITaskCreation{TaskID: v.ID, PricePoints: v.PricePoints, PriceVersion: v.PriceVersion}, nil
}
func intentFixture(t *testing.T) (*AIConversationTaskService, *intentMemory, *intentCreator, int64) {
	t.Helper()
	convs, b := conversationFixture(t)
	r, err := convs.Create(context.Background(), 42, "conversation_intent_test", "测试任务会话")
	if err != nil {
		t.Fatal(err)
	}
	m := &intentMemory{base: b, intents: map[int64]conversation.Intent{}, tasks: map[string]conversation.TaskIdentity{}}
	c := &intentCreator{m: m, points: 7, commit: true}
	s, err := NewAIConversationTaskService(m, c, true)
	if err != nil {
		t.Fatal(err)
	}
	return s, m, c, r.ID
}
func prepareIntent(t *testing.T, s *AIConversationTaskService, cid int64) dto.AIConversationTask {
	t.Helper()
	v, err := s.Prepare(context.Background(), 42, cid, "conversation_task_test", conversation.IntentInput{AnalysisType: "light", SubjectType: "theme", SubjectID: 6, PriceVersion: "test-v1"})
	if err != nil || v.State != conversation.Prepared {
		t.Fatal(v, err)
	}
	return v
}
func TestAIConversationTaskConcurrentConfirmAndHistory(t *testing.T) {
	s, m, c, cid := intentFixture(t)
	i := prepareIntent(t, s, cid)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for n := 0; n < 20; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Confirm(context.Background(), 42, cid, i.ID); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if c.creates.Load() != 1 || len(m.tasks) != 1 || len(m.base.msgs[cid]) != 1 || m.base.rows[cid].LastSeq != 1 {
		t.Fatal("重复执行或消息", c.creates.Load())
	}
	v, err := s.Detail(context.Background(), 42, cid, i.ID)
	if err != nil || v.State != conversation.Linked || v.TaskID != 40 || v.MessageSeq != 1 {
		t.Fatal(v, err)
	}
	r, err := s.Receipt(context.Background(), 42, cid, "conversation_task_test")
	if err != nil || r != v {
		t.Fatal(r, err)
	}
	replay, err := s.Prepare(context.Background(), 42, cid, "conversation_task_test", m.intents[i.ID].Input)
	if err != nil || replay != v || c.quotes.Load() != 1 {
		t.Fatal(replay, err)
	}
	list, err := s.List(context.Background(), 42, cid, 0, 20)
	if err != nil || len(list.Items) != 1 {
		t.Fatal(list, err)
	}
}
func TestAIConversationTaskUnknownClaimNeverCreates(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			s, m, c, cid := intentFixture(t)
			i := prepareIntent(t, s, cid)
			m.failState = conversation.Unconfirmed
			m.commitUnknown = committed
			if _, err := s.Confirm(context.Background(), 42, cid, i.ID); err == nil {
				t.Fatal("提交未知不能授予创建权限")
			}
			if c.creates.Load() != 0 {
				t.Fatal("越过未知提交创建")
			}
			if committed {
				for n := 0; n < 3; n++ {
					v, err := s.Confirm(context.Background(), 42, cid, i.ID)
					if err != nil || v.State != conversation.Unconfirmed {
						t.Fatal(v, err)
					}
				}
				if c.creates.Load() != 0 || len(m.tasks) != 0 {
					t.Fatal("重试重新创建")
				}
			}
		})
	}
}
func TestAIConversationTaskBindingRecoveryNeverResubmits(t *testing.T) {
	for _, mode := range []string{"append_failure", "binding_commit_unknown", "original_commit_unknown"} {
		t.Run(mode, func(t *testing.T) {
			s, m, c, cid := intentFixture(t)
			i := prepareIntent(t, s, cid)
			switch mode {
			case "append_failure":
				m.failAppend = true
			case "binding_commit_unknown":
				m.failState = conversation.Linked
				m.commitUnknown = true
			case "original_commit_unknown":
				c.cause = errors.New("测试原创建提交未知")
			}
			_, err := s.Confirm(context.Background(), 42, cid, i.ID)
			if mode != "original_commit_unknown" && err == nil {
				t.Fatal("故障没有生效")
			}
			m.failAppend = false
			for n := 0; n < 3; n++ {
				v, err := s.Reconcile(context.Background(), 42, cid, i.ID)
				if err != nil || v.State != conversation.Linked {
					t.Fatal(v, err)
				}
			}
			if c.creates.Load() != 1 || len(m.tasks) != 1 || len(m.base.msgs[cid]) != 1 || m.base.rows[cid].LastSeq != 1 {
				t.Fatal("恢复重复创建或序号未回滚")
			}
		})
	}
}
func TestAIConversationTaskKnownRejectAndUnknownRemainDistinct(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause error
		state string
	}{
		{"price", points.ErrPriceChanged, conversation.NotCreated}, {"funds", points.ErrInsufficient, conversation.NotCreated}, {"closed", task.ErrCreationDisabled, conversation.NotCreated}, {"admission", task.ErrAdmissionUnavailable, conversation.NotCreated}, {"invalid", points.ErrInvalid, conversation.NotCreated}, {"unknown", errors.New("测试超时"), conversation.Unconfirmed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, m, c, cid := intentFixture(t)
			i := prepareIntent(t, s, cid)
			c.commit = false
			c.cause = tc.cause
			_, err := s.Confirm(context.Background(), 42, cid, i.ID)
			if tc.state == conversation.Unconfirmed && !errors.Is(err, conversation.ErrUnconfirmed) {
				t.Fatal(err)
			}
			v, err := s.Reconcile(context.Background(), 42, cid, i.ID)
			if err != nil || v.State != tc.state {
				t.Fatal(v, err)
			}
			if _, err := s.Confirm(context.Background(), 42, cid, i.ID); err != nil {
				t.Fatal(err)
			}
			if c.creates.Load() != 1 || len(m.tasks) != 0 || len(m.base.msgs[cid]) != 0 {
				t.Fatal("失败重建或写入任务消息")
			}
		})
	}
}
func TestAIConversationTaskOwnershipAndScopeFailClosed(t *testing.T) {
	s, m, c, cid := intentFixture(t)
	i := prepareIntent(t, s, cid)
	ctx := context.Background()
	for _, fn := range []func() error{
		func() error {
			_, e := s.Prepare(ctx, 43, cid, "conversation_task_test", m.intents[i.ID].Input)
			return e
		},
		func() error { _, e := s.Confirm(ctx, 43, cid, i.ID); return e }, func() error { _, e := s.Reconcile(ctx, 43, cid, i.ID); return e }, func() error { _, e := s.Detail(ctx, 43, cid, i.ID); return e }, func() error { _, e := s.Receipt(ctx, 43, cid, "conversation_task_test"); return e }, func() error { _, e := s.List(ctx, 43, cid, 0, 20); return e },
	} {
		if err := fn(); !errors.Is(err, conversation.ErrNotFound) {
			t.Fatal(err)
		}
	}
	input := m.intents[i.ID].Input
	input.SubjectID++
	if _, err := s.Prepare(ctx, 42, cid, "conversation_task_test", input); !errors.Is(err, conversation.ErrConflict) {
		t.Fatal(err)
	}
	if c.creates.Load() != 0 || c.quotes.Load() != 1 {
		t.Fatal("越权或冲突调用创建器")
	}
}
func TestAIConversationTaskClosingAndPriceChanges(t *testing.T) {
	s, m, c, cid := intentFixture(t)
	i := prepareIntent(t, s, cid)
	c.points = 9
	v, err := s.Confirm(context.Background(), 42, cid, i.ID)
	if err != nil || v.State != conversation.NotCreated || v.RejectionCode != "price_changed" || len(m.tasks) != 0 {
		t.Fatal(v, err)
	}
	closed, _ := NewAIConversationTaskService(m, c, false)
	if _, err := closed.Confirm(context.Background(), 42, cid, i.ID); !errors.Is(err, task.ErrCreationDisabled) {
		t.Fatal(err)
	}
	if _, err := closed.Prepare(context.Background(), 42, cid, "different_intent_key", m.intents[i.ID].Input); !errors.Is(err, task.ErrCreationDisabled) {
		t.Fatal(err)
	}
	if v, err := closed.Reconcile(context.Background(), 42, cid, i.ID); err != nil || v.State != conversation.NotCreated {
		t.Fatal(v, err)
	}
}
func TestAIConversationTaskWrongIdentityCannotBind(t *testing.T) {
	for _, field := range []string{"owner", "subject", "hash", "price", "message"} {
		t.Run(field, func(t *testing.T) {
			s, m, c, cid := intentFixture(t)
			i := prepareIntent(t, s, cid)
			m.failAppend = true
			_, err := s.Confirm(context.Background(), 42, cid, i.ID)
			if err == nil {
				t.Fatal("应在绑定处失败")
			}
			m.failAppend = false
			key := m.intents[i.ID].TaskKey
			v := m.tasks[key]
			switch field {
			case "owner":
				v.UserID++
			case "subject":
				v.SubjectID++
			case "hash":
				v.RequestHash = ""
			case "price":
				v.PricePoints++
			case "message":
			}
			m.tasks[key] = v
			if field == "message" {
				if _, err := s.Reconcile(context.Background(), 42, cid, i.ID); err != nil {
					t.Fatal(err)
				}
				msg := m.base.msgs[cid][0]
				msg.Input.SubjectID++
				m.base.msgs[cid][0] = msg
				_, err = s.Detail(context.Background(), 42, cid, i.ID)
			} else {
				_, err = s.Reconcile(context.Background(), 42, cid, i.ID)
			}
			if field != "owner" && err == nil {
				t.Fatal("错误身份或消息被接受")
			}
			if field == "owner" {
				v, err := s.Detail(context.Background(), 42, cid, i.ID)
				if err != nil || v.State != conversation.Unconfirmed {
					t.Fatal(v, err)
				}
			}
			if c.creates.Load() != 1 {
				t.Fatal("错误身份后重新创建")
			}
		})
	}
}
