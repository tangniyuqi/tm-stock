package qa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/conversation"
)

// 仅执行控制器单测的事务替身；真实持久/授权/并发另由MySQL专项验证。
type qaMemoryData struct {
	Conv     conversation.Record
	Messages []conversation.Message
	Question *Record
	Calls    []CallRecord
	Quotas   map[int64]Quota
}
type qaMemoryStore struct {
	mu            sync.Mutex
	data          qaMemoryData
	policy        QuotaPolicy
	authorized    bool
	fail, unknown string
	now           time.Time
}
type qaMemoryTx struct {
	*qaMemoryStore
	write bool
}

func copyQA[T any](in T) T {
	b, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}
func (s *qaMemoryStore) WithinQA(ctx context.Context, w bool, fn func(Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.fail == "within" {
		return ErrUnavailable
	}
	old := copyQA(s.data)
	err := fn(&qaMemoryTx{s, w})
	if err != nil {
		s.data = old
		return err
	}
	if s.unknown != "" && s.data.Question != nil && s.data.Question.State == s.unknown {
		s.unknown = ""
		return ErrUnavailable
	}
	return nil
}
func (t *qaMemoryTx) Now(context.Context) (time.Time, error) {
	if t.fail == "now" {
		return time.Time{}, ErrUnavailable
	}
	return t.now, nil
}
func (t *qaMemoryTx) CreateOrGet(context.Context, conversation.Record) (conversation.Record, error) {
	return conversation.Record{}, ErrInvalid
}
func (t *qaMemoryTx) ByKey(context.Context, int64, string) (conversation.Record, error) {
	return conversation.Record{}, ErrInvalid
}
func (t *qaMemoryTx) List(context.Context, int64, int64, bool, int) ([]conversation.Record, error) {
	return nil, ErrInvalid
}
func (t *qaMemoryTx) ByID(_ context.Context, uid, cid int64, _ bool) (conversation.Record, error) {
	if t.fail == "conversation" {
		return conversation.Record{}, ErrUnavailable
	}
	if t.data.Conv.UserID != uid || t.data.Conv.ID != cid {
		return conversation.Record{}, ErrNotFound
	}
	return t.data.Conv, nil
}
func (t *qaMemoryTx) Update(_ context.Context, old, next conversation.Record) error {
	if t.fail == "update" {
		return ErrUnavailable
	}
	if !t.write || t.data.Conv.Version != old.Version {
		return ErrConflict
	}
	t.data.Conv = next
	return nil
}
func (t *qaMemoryTx) Append(_ context.Context, m conversation.Message) error {
	if t.fail == "append" {
		return ErrUnavailable
	}
	t.data.Messages = append(t.data.Messages, m)
	return nil
}
func (t *qaMemoryTx) Messages(_ context.Context, uid, cid, after, through int64, _ int) ([]conversation.Message, error) {
	if t.fail == "messages" {
		return nil, ErrUnavailable
	}
	out := []conversation.Message{}
	for _, m := range t.data.Messages {
		if m.UserID == uid && m.ConversationID == cid && m.Seq > after && m.Seq <= through {
			out = append(out, m)
		}
	}
	return copyQA(out), nil
}
func (t *qaMemoryTx) Authorize(_ context.Context, _ Proof) error {
	if !t.authorized || t.fail == "authorize" {
		return ErrUnavailable
	}
	return nil
}
func (t *qaMemoryTx) QuotaPolicy(context.Context) (QuotaPolicy, error) {
	if t.fail == "policy" {
		return QuotaPolicy{}, ErrUnavailable
	}
	return t.policy, nil
}
func (t *qaMemoryTx) Quota(_ context.Context, day string, uid int64) (Quota, error) {
	if t.fail == "quota" {
		return Quota{}, ErrUnavailable
	}
	q, ok := t.data.Quotas[uid]
	if !ok {
		q = Quota{Day: day, UserID: uid}
		t.data.Quotas[uid] = q
	}
	return q, nil
}
func (t *qaMemoryTx) AddQuota(_ context.Context, q Quota, n int64) error {
	if t.fail == "add_quota" {
		return ErrUnavailable
	}
	q.Requests++
	q.Tokens += n
	t.data.Quotas[q.UserID] = q
	return nil
}
func (t *qaMemoryTx) Question(_ context.Context, id Identity, _ bool) (Record, error) {
	if t.fail == "question" {
		return Record{}, ErrUnavailable
	}
	if t.data.Question == nil || t.data.Question.Proof.Identity != id {
		return Record{}, ErrNotFound
	}
	return copyQA(*t.data.Question), nil
}
func (t *qaMemoryTx) QuestionByKey(_ context.Context, uid, cid int64, key string) (Record, error) {
	if t.fail == "by_key" {
		return Record{}, ErrUnavailable
	}
	if t.data.Question != nil && len(t.data.Messages) > 0 && t.data.Messages[0].Key == key && t.data.Messages[0].UserID == uid && t.data.Messages[0].ConversationID == cid {
		return copyQA(*t.data.Question), nil
	}
	return Record{}, ErrNotFound
}
func (t *qaMemoryTx) InsertQuestion(_ context.Context, r Record) error {
	if t.fail == "insert" {
		return ErrUnavailable
	}
	v := copyQA(r)
	t.data.Question = &v
	return nil
}
func (t *qaMemoryTx) SetQuestion(_ context.Context, old, next Record) error {
	if t.fail == "set" {
		return ErrUnavailable
	}
	if t.data.Question == nil || t.data.Question.Version != old.Version {
		return ErrConflict
	}
	v := copyQA(next)
	t.data.Question = &v
	return nil
}
func (t *qaMemoryTx) Calls(context.Context, string) ([]CallRecord, error) {
	if t.fail == "calls" {
		return nil, ErrUnavailable
	}
	return copyQA(t.data.Calls), nil
}
func (t *qaMemoryTx) InsertCall(_ context.Context, r CallRecord) error {
	if t.fail == "insert_call" {
		return ErrUnavailable
	}
	t.data.Calls = append(t.data.Calls, copyQA(r))
	return nil
}
func (t *qaMemoryTx) FinishCall(_ context.Context, old CallRecord, result CallResult, now time.Time) error {
	if t.fail == "finish" {
		return ErrUnavailable
	}
	next := old
	next.State = "recorded"
	next.Result = copyQA(result)
	next.UpdatedAt = now
	t.data.Calls[old.Call.Step-1] = next
	return nil
}

func durableFixture(t *testing.T) (*DurableControl, *Engine, *qaMemoryStore, *Plan) {
	t.Helper()
	e, p, _, _ := fixture(t, successfulSteps()...)
	now := p.scope.Snapshots[0].FetchedAt
	s := &qaMemoryStore{authorized: true, now: now, policy: QuotaPolicy{Ref: p.cfg.Selector.BudgetRef, UserRequests: 10, InstanceRequests: 20, UserTokens: 1000000, InstanceTokens: 2000000}, data: qaMemoryData{Conv: conversation.Record{ID: p.identity.ConversationID, UserID: p.identity.UserID, Key: "test_conversation_key_01", RequestHash: conversation.CreationHash("示例问答"), Title: "示例问答", Version: 1, CreatedAt: now, UpdatedAt: now}, Quotas: map[int64]Quota{}}}
	c, err := NewDurableControl(s, p)
	if err != nil {
		t.Fatal(err)
	}
	e.control = c
	return c, e, s, p
}
func queueQA(t *testing.T, c *DurableControl) {
	t.Helper()
	if _, err := c.Enqueue(context.Background(), "test_question_key_01", 1); err != nil {
		t.Fatal(err)
	}
}

func TestDurableControlSuccessReadAndImmutableStages(t *testing.T) {
	c, e, s, p := durableFixture(t)
	queueQA(t, c)
	if _, err := c.Enqueue(context.Background(), "test_question_key_01", 1); err != nil || len(s.data.Messages) != 1 {
		t.Fatal("原键重放重复消息", err)
	}
	v, err := c.Run(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.View(context.Background(), p, v); err != nil {
		t.Fatal(err)
	}
	if out, err := e.ReadStored(context.Background(), p); err != nil || len(out.Quotes) != 1 {
		t.Fatal("不能从存证重建", err)
	}
	if len(s.data.Calls) != 3 || s.data.Question.State != "answered" || s.data.Question.ReservedTokens != 39072 || s.data.Quotas[0].Tokens != 39072 || s.data.Quotas[p.identity.UserID].Requests != 1 {
		t.Fatal("存证或预留不符")
	}
	if err := c.FinishCall(context.Background(), s.data.Calls[0].Call, s.data.Calls[0].Result); err == nil {
		t.Fatal("终态不能再写调用")
	}
	if err := c.Fail(context.Background(), "unconfirmed"); err != nil || s.data.Question.State != "answered" {
		t.Fatal("后到失败覆盖终态")
	}
	s.authorized = false
	if out, err := e.ReadStored(context.Background(), p); err == nil || len(out.Quotes) != 0 {
		t.Fatal("撤权仍可读")
	}
}

func TestDurableControlQueueAtomicAndUnknown(t *testing.T) {
	t.Run("撤回后仍可核本人原收据不重开执行", func(t *testing.T) {
		c, e, s, _ := durableFixture(t)
		queueQA(t, c)
		s.authorized = false
		if r, err := c.Enqueue(context.Background(), "test_question_key_01", 1); err != nil || r.State != "queued" || len(s.data.Messages) != 1 {
			t.Fatal("撤权妨碍原收据恢复", err)
		}
		if _, err := c.Run(context.Background(), e); err == nil || len(s.data.Calls) != 0 {
			t.Fatal("原收据授予了执行权限")
		}
	})
	for _, fail := range []string{"within", "conversation", "by_key", "authorize", "now", "update", "append", "insert"} {
		t.Run(fail, func(t *testing.T) {
			c, _, s, _ := durableFixture(t)
			s.fail = fail
			if _, err := c.Enqueue(context.Background(), "test_question_key_01", 1); err == nil || len(s.data.Messages) != 0 || s.data.Question != nil || s.data.Conv.LastSeq != 0 {
				t.Fatal("入队失败未整笔回滚")
			}
		})
	}
	t.Run("提交后回执未知", func(t *testing.T) {
		c, _, s, _ := durableFixture(t)
		s.unknown = "queued"
		if _, err := c.Enqueue(context.Background(), "test_question_key_01", 1); err == nil || s.data.Question == nil {
			t.Fatal("未产生真实的替身提交后未知")
		}
		if _, err := c.Enqueue(context.Background(), "test_question_key_01", 1); err != nil || len(s.data.Messages) != 1 {
			t.Fatal("原键恢复重复保存")
		}
	})
	for _, bad := range []func(*qaMemoryStore){func(s *qaMemoryStore) { s.data.Conv.Archived = true; s.data.Conv.Version++ }, func(s *qaMemoryStore) { s.data.Conv.Version++ }, func(s *qaMemoryStore) { s.data.Conv.LastSeq = 2 }, func(s *qaMemoryStore) { s.authorized = false }, func(s *qaMemoryStore) { s.now = time.Time{} }} {
		c, _, s, _ := durableFixture(t)
		bad(s)
		if _, err := c.Enqueue(context.Background(), "test_question_key_01", 1); err == nil {
			t.Fatal("非法会话/时点入队")
		}
	}
}

func TestDurableControlBeginQuotaAndNoRelease(t *testing.T) {
	for _, fail := range []string{"question", "messages", "policy", "quota", "add_quota", "now", "set"} {
		t.Run(fail, func(t *testing.T) {
			c, _, s, p := durableFixture(t)
			queueQA(t, c)
			s.fail = fail
			if err := c.Begin(context.Background(), p.Proof()); err == nil || s.data.Question.State != "queued" || len(s.data.Quotas) != 0 {
				t.Fatal("开始失败仍取得执行权或部分预留")
			}
		})
	}
	for _, edit := range []func(*qaMemoryStore){func(s *qaMemoryStore) { s.policy.UserTokens = 1 }, func(s *qaMemoryStore) { s.policy.Ref = "另一份预算" }, func(s *qaMemoryStore) { s.policy = QuotaPolicy{} }, func(s *qaMemoryStore) { s.data.Quotas[0] = Quota{Day: s.now.Format("2006-01-02"), Requests: 20} }, func(s *qaMemoryStore) { s.data.Quotas[0] = Quota{Day: "2000-01-01"} }} {
		c, _, s, p := durableFixture(t)
		queueQA(t, c)
		edit(s)
		old := copyQA(s.data.Quotas)
		if err := c.Begin(context.Background(), p.Proof()); err == nil || s.data.Question.State != "queued" || !sameJSON(old, s.data.Quotas) {
			t.Fatal("配额/批准失败仍修改计数")
		}
	}
	t.Run("权限提交后未知不重获", func(t *testing.T) {
		c, e, s, _ := durableFixture(t)
		queueQA(t, c)
		s.unknown = "running"
		if _, err := c.Run(context.Background(), e); err == nil || s.data.Question.State != "running" {
			t.Fatal("未知权限误判失败或未提交")
		}
		if _, err := c.Run(context.Background(), e); err == nil || s.data.Question.State != "running" || s.data.Quotas[0].Requests != 1 {
			t.Fatal("重放覆盖他人执行/再次预留")
		}
	})
}

func TestDurableControlStageFailuresAndTerminalUnknown(t *testing.T) {
	for _, fail := range []string{"calls", "insert_call", "finish"} {
		t.Run(fail, func(t *testing.T) {
			c, e, s, _ := durableFixture(t)
			queueQA(t, c)
			s.fail = fail
			if _, err := c.Run(context.Background(), e); err == nil || s.data.Question.State != "failed" || s.data.Quotas[0].Tokens != 39072 {
				t.Fatal("调用失败未闭集记录或释放预留")
			}
		})
	}
	t.Run("终态提交后未知", func(t *testing.T) {
		c, e, s, p := durableFixture(t)
		queueQA(t, c)
		s.unknown = "answered"
		if _, err := c.Run(context.Background(), e); err == nil || s.data.Question.State != "answered" {
			t.Fatal("终态未知被覆盖")
		}
		if _, err := e.ReadStored(context.Background(), p); err != nil {
			t.Fatal("不能读取已确认原存证", err)
		}
	})
	t.Run("无执行权不可标记别人的失败", func(t *testing.T) {
		c, e, s, p := durableFixture(t)
		queueQA(t, c)
		if err := c.Begin(context.Background(), p.Proof()); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Run(context.Background(), e); err == nil || s.data.Question.State != "running" {
			t.Fatal("重放终结在途执行")
		}
	})
}

func TestDurableControlStoredCorruptionFailsClosed(t *testing.T) {
	for name, edit := range map[string]func(*qaMemoryStore){
		"消息丢失": func(s *qaMemoryStore) { s.data.Messages = nil },
		"消息归属": func(s *qaMemoryStore) { s.data.Messages[0].UserID++ },
		"问题摘要": func(s *qaMemoryStore) { s.data.Question.MessageHash = strings.Repeat("a", 64) },
		"执行证明": func(s *qaMemoryStore) { s.data.Question.Proof.ScopeApproval = "其他范围" },
		"阶段缺失": func(s *qaMemoryStore) { s.data.Calls = s.data.Calls[:2] },
		"阶段断序": func(s *qaMemoryStore) { s.data.Calls[1].Call.Step = 3 },
		"请求摘要": func(s *qaMemoryStore) { s.data.Calls[1].Call.RequestDigest = strings.Repeat("a", 64) },
		"判定改写": func(s *qaMemoryStore) {
			s.data.Calls[1].Result.Decision.Verdict = "block"
			s.data.Calls[1].Result.Decision.Reason = "unsupported"
		},
		"最终选句": func(s *qaMemoryStore) { s.data.Question.Citations[0].SentenceIDs[0] = 2 },
		"用量超限": func(s *qaMemoryStore) {
			s.data.Calls[1].Result.Usage.Prompt = 12001
			s.data.Calls[1].Result.Usage.Total = 12031
		},
		"预留合计": func(s *qaMemoryStore) { s.data.Question.ReservedTokens-- },
	} {
		t.Run(name, func(t *testing.T) {
			c, e, s, p := durableFixture(t)
			queueQA(t, c)
			if _, err := c.Run(context.Background(), e); err != nil {
				t.Fatal(err)
			}
			edit(s)
			if out, err := e.ReadStored(context.Background(), p); err == nil || len(out.Quotes) != 0 {
				t.Fatal("损坏存证仍交付")
			}
		})
	}
}

func TestDurableControlInvalidArguments(t *testing.T) {
	c, e, _, p := durableFixture(t)
	if _, err := NewDurableControl(nil, p); err == nil {
		t.Fatal("空存储")
	}
	if _, err := NewDurableControl(c.store, nil); err == nil {
		t.Fatal("空计划")
	}
	for _, args := range []struct {
		key     string
		version int64
	}{{"bad", 1}, {"test_question_key_01", 0}} {
		if _, err := c.Enqueue(context.Background(), args.key, args.version); err == nil {
			t.Fatal("非法原键/版本")
		}
	}
	if c.within(nil, true, func(context.Context, Tx) error { return nil }) == nil || c.within(context.Background(), true, nil) == nil {
		t.Fatal("空上下文/回调")
	}
	bad := p.Proof()
	bad.ScopeApproval = "其他范围"
	if c.Begin(context.Background(), bad) == nil || c.Recheck(context.Background(), bad) == nil || c.Complete(context.Background(), bad, Receipt{}) == nil {
		t.Fatal("跨计划调用")
	}
	if _, err := c.ReadReceipt(context.Background(), bad); err == nil {
		t.Fatal("跨计划读取")
	}
	if c.Fail(context.Background(), "arbitrary") == nil || c.FinishCall(nil, Call{}, CallResult{}) == nil {
		t.Fatal("非法失败/审计")
	}
	if _, err := c.Run(nil, e); err == nil {
		t.Fatal("空上下文运行")
	}
	if _, err := e.ReadStored(nil, p); err == nil {
		t.Fatal("空上下文读取")
	}
	if _, err := e.ReadStored(context.Background(), p); !errors.Is(err, ErrNotFound) {
		t.Fatal("无存证读取", err)
	}
	for _, v := range []any{Record{Proof: p.Proof()}, CallRecord{Call: Call{InputDigest: p.digest}}} {
		if strings.Contains(fmt.Sprintf("%+v %#v", v, v), p.digest) {
			t.Fatal("格式化泄漏摘要")
		}
	}
}
