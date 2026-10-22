package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/feedback"
)

var errFeedbackTest = errors.New("测试反馈故障，不能返回会员")

type feedbackMemory struct {
	mu      sync.Mutex
	rows    map[int64]feedback.Record
	events  map[int64][]feedback.Event
	next    int64
	fail    string
	unknown bool
}
type feedbackMemoryTx struct{ *feedbackMemory }

func (s *feedbackMemory) Within(ctx context.Context, fn func(feedback.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.fail == "begin" {
		return errFeedbackTest
	}
	copy := &feedbackMemory{rows: map[int64]feedback.Record{}, events: map[int64][]feedback.Event{}, next: s.next, fail: s.fail}
	for k, v := range s.rows {
		copy.rows[k] = v
	}
	for k, v := range s.events {
		copy.events[k] = append([]feedback.Event(nil), v...)
	}
	if err := fn(&feedbackMemoryTx{copy}); err != nil {
		return err
	}
	if s.fail == "commit" {
		return errFeedbackTest
	}
	s.rows, s.events, s.next = copy.rows, copy.events, copy.next
	if s.unknown {
		s.unknown = false
		return errFeedbackTest
	}
	return nil
}
func (s *feedbackMemoryTx) Now(ctx context.Context) (time.Time, error) {
	if s.fail == "now" {
		return time.Time{}, errFeedbackTest
	}
	return time.UnixMilli(1792480000000).UTC(), ctx.Err()
}
func (s *feedbackMemoryTx) OwnedReport(ctx context.Context, uid, id int64) (string, error) {
	if s.fail == "owner" {
		return "", errFeedbackTest
	}
	if uid != 1 {
		return "", feedback.ErrNotFound
	}
	if id == 8 {
		return "", feedback.ErrNoReport
	}
	if id != 9 {
		return "", feedback.ErrNotFound
	}
	return strings.Repeat("a", 64), ctx.Err()
}
func (s *feedbackMemoryTx) ByCategory(_ context.Context, uid, id int64, category string) (feedback.Record, error) {
	if s.fail == "find" {
		return feedback.Record{}, errFeedbackTest
	}
	for _, r := range s.rows {
		if r.UserID == uid && r.TaskID == id && r.Category == category {
			return r, nil
		}
	}
	return feedback.Record{}, feedback.ErrNotFound
}
func (s *feedbackMemoryTx) ByID(_ context.Context, id int64) (feedback.Record, error) {
	if s.fail == "find" {
		return feedback.Record{}, errFeedbackTest
	}
	r, ok := s.rows[id]
	if !ok {
		return r, feedback.ErrNotFound
	}
	return r, nil
}
func (s *feedbackMemoryTx) Insert(_ context.Context, r feedback.Record) (feedback.Record, error) {
	s.next++
	r.ID = s.next
	s.rows[r.ID] = r
	if s.fail == "insert" {
		return r, errFeedbackTest
	}
	return r, nil
}
func (s *feedbackMemoryTx) Update(_ context.Context, old, next feedback.Record) error {
	s.rows[old.ID] = next
	if s.fail == "update" {
		return errFeedbackTest
	}
	return nil
}
func (s *feedbackMemoryTx) Append(_ context.Context, e feedback.Event) error {
	s.events[e.FeedbackID] = append(s.events[e.FeedbackID], e)
	if s.fail == "append" {
		return errFeedbackTest
	}
	return nil
}
func (s *feedbackMemoryTx) History(_ context.Context, id int64) ([]feedback.Event, error) {
	if s.fail == "history" {
		return nil, errFeedbackTest
	}
	return append([]feedback.Event(nil), s.events[id]...), nil
}
func (s *feedbackMemoryTx) ForReport(ctx context.Context, uid, id int64) ([]feedback.Record, error) {
	rows, err := s.List(ctx, 0, 100)
	out := []feedback.Record{}
	for _, r := range rows {
		if r.UserID == uid && r.TaskID == id {
			out = append(out, r)
		}
	}
	return out, err
}
func (s *feedbackMemoryTx) List(_ context.Context, after int64, limit int) ([]feedback.Record, error) {
	if s.fail == "list" {
		return nil, errFeedbackTest
	}
	out := []feedback.Record{}
	for _, r := range s.rows {
		if r.ID > after {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func feedbackFixture(t *testing.T) (*AIFeedbackService, *feedbackMemory) {
	t.Helper()
	s := &feedbackMemory{rows: map[int64]feedback.Record{}, events: map[int64][]feedback.Event{}}
	v, err := NewAIFeedbackService(s)
	if err != nil {
		t.Fatal(err)
	}
	return v, s
}

func TestAIFeedbackLifecycle(t *testing.T) {
	s, m := feedbackFixture(t)
	ctx := context.Background()
	for _, category := range []string{"fact", "source", "display", "other"} {
		a, err := s.Submit(ctx, 1, 9, category)
		if err != nil || a.Status != "received" {
			t.Fatal(a, err)
		}
		b, err := s.Submit(ctx, 1, 9, category)
		if err != nil || a != b {
			t.Fatal("重复受理", err)
		}
	}
	if len(m.rows) != 4 {
		t.Fatal("类别数量")
	}
	first, err := s.List(ctx, 7, 0, 2)
	if err != nil || len(first.Items) != 2 || !first.HasMore {
		t.Fatal(first, err)
	}
	last, err := s.List(ctx, 7, first.NextAfterID, 2)
	if err != nil || len(last.Items) != 2 || last.HasMore || last.NextAfterID != 0 {
		t.Fatal(last, err)
	}
	if _, err = s.Decide(ctx, 7, 1, feedback.Decision{ExpectedVersion: 2, Status: "resolved", Resolution: "reviewed"}); !errors.Is(err, feedback.ErrConflict) {
		t.Fatal(err)
	}
	process := feedback.Decision{ExpectedVersion: 1, Status: "processing"}
	if _, err = s.Decide(ctx, 7, 1, process); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Decide(ctx, 8, 1, process); !errors.Is(err, feedback.ErrConflict) {
		t.Fatal("操作人覆盖", err)
	}
	if _, err = s.Decide(ctx, 7, 1, feedback.Decision{ExpectedVersion: 2, Status: "resolved", Resolution: "reviewed"}); err != nil {
		t.Fatal(err)
	}
	r, err := s.Decide(ctx, 7, 1, process)
	if err != nil || r.Status != "resolved" || len(m.events[1]) != 3 {
		t.Fatal("原操作重放", r, err)
	}
	detail, err := s.Detail(ctx, 7, 1)
	if err != nil || len(detail.Events) != 3 || detail.Events[1].ActorID != 7 {
		t.Fatal(detail, err)
	}
	if _, err = s.Decide(ctx, 7, 2, process); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Decide(ctx, 7, 2, feedback.Decision{ExpectedVersion: 2, Status: "dismissed", Resolution: "unconfirmed"}); err != nil {
		t.Fatal(err)
	}
	public, err := s.ForReport(ctx, 1, 9)
	if err != nil || len(public.Items) != 4 || public.Items[0].Status != "resolved" {
		t.Fatal(public, err)
	}
	b, _ := json.Marshal(public)
	for _, word := range []string{"actorId", "userId", "reportDigest", "events"} {
		if strings.Contains(string(b), word) {
			t.Fatal("公开DTO泄漏", word)
		}
	}
}
func TestAIFeedbackRollbackAndUnknown(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"begin", "owner", "find", "now", "insert", "append", "history", "commit"} {
		t.Run(stage, func(t *testing.T) {
			s, m := feedbackFixture(t)
			m.fail = stage
			r, err := s.Submit(ctx, 1, 9, "fact")
			if err == nil || r.ID != 0 || len(m.rows) != 0 || len(m.events) != 0 {
				t.Fatal("受理失败未回滚", r, err)
			}
		})
	}
	for _, stage := range []string{"begin", "find", "now", "update", "append", "history", "commit"} {
		t.Run("decision_"+stage, func(t *testing.T) {
			s, m := feedbackFixture(t)
			r, err := s.Submit(ctx, 1, 9, "fact")
			if err != nil {
				t.Fatal(err)
			}
			m.fail = stage
			_, err = s.Decide(ctx, 7, r.ID, feedback.Decision{ExpectedVersion: 1, Status: "processing"})
			if err == nil || m.rows[r.ID].Status != "received" || len(m.events[r.ID]) != 1 {
				t.Fatal("处理失败未回滚", err)
			}
		})
	}
	s, m := feedbackFixture(t)
	m.unknown = true
	r, err := s.Submit(ctx, 1, 9, "fact")
	if err == nil || r.ID != 0 {
		t.Fatal("提交未知伪造成功")
	}
	list, err := s.ForReport(ctx, 1, 9)
	if err != nil || len(list.Items) != 1 {
		t.Fatal("原类别查询失败", err)
	}
	again, err := s.Submit(ctx, 1, 9, "fact")
	if err != nil || again.ID != list.Items[0].ID || len(m.rows) != 1 {
		t.Fatal("重放重复受理", err)
	}
	m.unknown = true
	_, err = s.Decide(ctx, 7, again.ID, feedback.Decision{ExpectedVersion: 1, Status: "processing"})
	if err == nil {
		t.Fatal("处理未知伪成功")
	}
	detail, err := s.Detail(ctx, 7, again.ID)
	if err != nil || detail.Item.Status != "processing" || len(detail.Events) != 2 {
		t.Fatal(detail, err)
	}
}
func TestAIFeedbackConcurrencyAndIsolation(t *testing.T) {
	s, m := feedbackFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Submit(ctx, 1, 9, "fact"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(m.rows) != 1 || len(m.events[1]) != 1 {
		t.Fatal("并发重复受理")
	}
	wins := 0
	var mu sync.Mutex
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(actor int64) {
			defer wg.Done()
			_, err := s.Decide(ctx, actor, 1, feedback.Decision{ExpectedVersion: 1, Status: "processing"})
			if err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			} else if !errors.Is(err, feedback.ErrConflict) {
				t.Error(err)
			}
		}(int64(i + 1))
	}
	wg.Wait()
	if wins != 1 || len(m.events[1]) != 2 {
		t.Fatal("并发处理覆盖")
	}
	if _, err := s.Submit(ctx, 2, 9, "fact"); !errors.Is(err, feedback.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.ForReport(ctx, 2, 9); !errors.Is(err, feedback.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.ForReport(ctx, 1, 8); !errors.Is(err, feedback.ErrNoReport) {
		t.Fatal(err)
	}
	if _, err := s.Submit(ctx, 1, 9, "free_text"); !errors.Is(err, feedback.ErrInvalid) {
		t.Fatal(err)
	}
	m.events[1] = m.events[1][:1]
	if r, err := s.ForReport(ctx, 1, 9); !errors.Is(err, feedback.ErrInvariant) || len(r.Items) != 0 {
		t.Fatal("损坏历史公开", err)
	}
	if _, err := s.Detail(ctx, 0, 1); !errors.Is(err, feedback.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Decide(ctx, 0, 1, feedback.Decision{}); !errors.Is(err, feedback.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.List(ctx, 7, 0, 101); !errors.Is(err, feedback.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Detail(ctx, 7, 999); !errors.Is(err, feedback.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := NewAIFeedbackService(nil); err == nil {
		t.Fatal("空存储")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.ForReport(cancelled, 1, 9); !errors.Is(err, context.Canceled) {
		t.Fatal("取消上下文被吞", err)
	}
}
