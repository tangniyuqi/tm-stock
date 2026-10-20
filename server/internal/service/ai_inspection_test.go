package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

type inspectionTestStore struct {
	mu        sync.Mutex
	window    []candidate.AuditEvent
	batch     candidate.InspectionBatch
	item      candidate.InspectionItem
	mapping   candidate.InspectionMapping
	events    map[string]candidate.InspectionEvent
	review    *reviewTestStore
	failAt    string
	refreshed bool
}
type inspectionTestTx struct{ *inspectionTestStore }

func newInspectionTestService(t *testing.T) (*AIInspectionService, *inspectionTestStore, candidate.InspectionRequest) {
	t.Helper()
	_, review, request := newReviewTestService(t)
	original := candidate.AuditEvent{ID: 10, Request: request, ActorID: 7, MappingID: 100, CreatedAt: time.Now().UTC().Truncate(time.Millisecond)}
	c := review.record.Candidate
	store := &inspectionTestStore{window: []candidate.AuditEvent{original}, item: candidate.InspectionItem{Selection: candidate.InspectionSelection{ID: 1, BatchID: 1, ReviewID: 10, CandidateID: c.ID}, ActorID: 8, Original: original, Candidate: c}, events: map[string]candidate.InspectionEvent{}, review: review,
		mapping: candidate.InspectionMapping{ID: 100, ThemeID: c.ThemeID, StockID: c.StockID, TsCode: c.TsCode, SourceType: c.SourceType, SourceExcerpt: c.SourceExcerpt, SourceURL: c.SourceURL, CollectedAt: c.CollectedAt, AuditStatus: 2, Status: 1}}
	svc, err := NewAIInspectionService(store)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := candidate.InspectionDigest(store.item)
	if err != nil {
		t.Fatal(err)
	}
	return svc, store, candidate.InspectionRequest{ItemID: 1, EvidenceDigest: digest, Decision: "confirm", RequestID: "test_inspection_decision_0001"}
}

func TestAIInspectionTrimmedEvidenceAndLabelTampering(t *testing.T) {
	for _, tamper := range []bool{false, true} {
		svc, store, request := newInspectionTestService(t)
		setTrimmedReviewFixture(t, store.review, "星河科技在年报中披露主营算力服务。建议买入。", []int{1, 2})
		c := store.review.record.Candidate
		store.item.Candidate = c
		store.item.Original.Request.EvidenceDigest, _ = candidate.EvidenceDigest(c)
		store.mapping.SourceExcerpt, store.mapping.ExcerptTrimmed = c.SourceExcerpt, !tamper
		request.EvidenceDigest, _ = candidate.InspectionDigest(store.item)
		event, err := svc.Decide(context.Background(), 8, request)
		if tamper {
			if !errors.Is(err, candidate.ErrEvidenceChanged) || event.ID != 0 || len(store.events) != 0 {
				t.Fatalf("复核未阻止标记被改: %v", err)
			}
		} else if err != nil || event.MappingID != 100 {
			t.Fatalf("节选不能复核确认: %+v %v", event, err)
		}
	}
}

func (s *inspectionTestStore) WithinInspectionTx(ctx context.Context, fn func(candidate.InspectionTx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	copyStore := &inspectionTestStore{window: s.window, batch: s.batch, item: s.item, mapping: s.mapping, events: map[string]candidate.InspectionEvent{}, review: s.review, failAt: s.failAt, refreshed: s.refreshed}
	for k, v := range s.events {
		copyStore.events[k] = v
	}
	if err := fn(&inspectionTestTx{copyStore}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.batch, s.mapping, s.events, s.refreshed = copyStore.batch, copyStore.mapping, copyStore.events, copyStore.refreshed
	return nil
}
func (s *inspectionTestStore) BatchByRequest(_ context.Context, id string) (candidate.InspectionBatch, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.batch, s.batch.ID > 0 && s.batch.Request.RequestID == id, nil
}
func (s *inspectionTestStore) ItemByID(_ context.Context, id int64) (candidate.InspectionItem, error) {
	if id != s.item.Selection.ID {
		return candidate.InspectionItem{}, candidate.ErrInspectionNotFound
	}
	return s.item, nil
}
func (s *inspectionTestStore) ListIssues(context.Context, int64, int) ([]candidate.InspectionEvent, error) {
	var out []candidate.InspectionEvent
	for _, e := range s.events {
		if e.Request.Decision == "issue" {
			out = append(out, e)
		}
	}
	return out, nil
}
func (t *inspectionTestTx) BatchByRequest(_ context.Context, id string) (candidate.InspectionBatch, bool, error) {
	return t.batch, t.batch.ID > 0 && t.batch.Request.RequestID == id, nil
}
func (t *inspectionTestTx) SamplingWindow(context.Context, int64, int) ([]candidate.AuditEvent, error) {
	if t.failAt == "window" {
		return nil, errors.New("测试窗口失败")
	}
	return t.window, nil
}
func (t *inspectionTestTx) InsertBatch(_ context.Context, b candidate.InspectionBatch, selected []candidate.AuditEvent) (candidate.InspectionBatch, error) {
	if t.failAt == "batch" {
		return b, errors.New("测试批次失败")
	}
	b.ID = 1
	b.CreatedAt = time.Now().UTC()
	for i, e := range selected {
		b.Items = append(b.Items, candidate.InspectionSelection{ID: int64(i + 1), BatchID: 1, ReviewID: e.ID, CandidateID: e.Request.CandidateID})
	}
	t.batch = b
	return b, nil
}
func (t *inspectionTestTx) LockItem(ctx context.Context, id int64) (candidate.InspectionItem, error) {
	return t.ItemByID(ctx, id)
}
func (t *inspectionTestTx) EventByRequest(_ context.Context, id string) (candidate.InspectionEvent, bool, error) {
	e, ok := t.events[id]
	return e, ok, nil
}
func (t *inspectionTestTx) EventByItem(_ context.Context, id int64) (candidate.InspectionEvent, bool, error) {
	for _, e := range t.events {
		if e.Request.ItemID == id {
			return e, true, nil
		}
	}
	return candidate.InspectionEvent{}, false, nil
}
func (t *inspectionTestTx) ApprovalSnapshot(context.Context, candidate.Candidate) (sources.Snapshot, error) {
	if t.review.unauthorized {
		return sources.Snapshot{}, sources.ErrUnauthorized
	}
	return t.review.snapshot, nil
}
func (t *inspectionTestTx) LockCatalog(context.Context, candidate.Candidate) (candidate.ReviewCatalog, error) {
	return t.review.catalog, nil
}
func (t *inspectionTestTx) LockMapping(context.Context, int64, int64) (candidate.InspectionMapping, error) {
	if t.failAt == "mapping" {
		return t.mapping, errors.New("测试归属失败")
	}
	return t.mapping, nil
}
func (t *inspectionTestTx) Quarantine(context.Context, int64, int64) error {
	if t.failAt == "quarantine" {
		return errors.New("测试暂停失败")
	}
	t.mapping.Status = 0
	return nil
}
func (t *inspectionTestTx) AppendInspection(_ context.Context, r candidate.InspectionRequest, actor, mapping int64) (candidate.InspectionEvent, error) {
	if t.failAt == "audit" {
		return candidate.InspectionEvent{}, errors.New("测试审计失败")
	}
	e := candidate.InspectionEvent{ID: 1, Request: r, ActorID: actor, MappingID: mapping, CreatedAt: time.Now().UTC()}
	t.events[r.RequestID] = e
	return e, nil
}
func (t *inspectionTestTx) RefreshThemeCount(context.Context, int64) error {
	if t.failAt == "count" {
		return errors.New("测试计数失败")
	}
	t.refreshed = true
	return nil
}

func TestInspectionConfirmIssueAndReplay(t *testing.T) {
	for _, decision := range []string{"confirm", "issue"} {
		t.Run(decision, func(t *testing.T) {
			svc, s, r := newInspectionTestService(t)
			r.Decision = decision
			if decision == "issue" {
				r.Reason = "原文未充分支持当前归属"
				s.review.unauthorized = true
			}
			e, err := svc.Decide(context.Background(), 8, r)
			if err != nil || e.ID != 1 || len(s.events) != 1 {
				t.Fatalf("复核失败: %+v %v", e, err)
			}
			if (s.mapping.Status == 0) != (decision == "issue") || s.refreshed != (decision == "issue") {
				t.Fatal("问题暂停或确认影响面错误")
			}
			s.review.unauthorized = true
			s.mapping.SourceExcerpt = "其他流程已改动"
			if replay, err := svc.Decide(context.Background(), 8, r); err != nil || replay != e {
				t.Fatalf("重放不应再更新归属: %v", err)
			}
			r.RequestID = "test_inspection_decision_0002"
			if _, err := svc.Decide(context.Background(), 8, r); !errors.Is(err, candidate.ErrConflict) {
				t.Fatal(err)
			}
		})
	}
}

func TestInspectionRejectsChangedEvidenceAndIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*inspectionTestStore, *candidate.InspectionRequest)
		actor  int64
		want   error
	}{
		{"非分派人", func(*inspectionTestStore, *candidate.InspectionRequest) {}, 9, candidate.ErrInspectionActor},
		{"自己复核", func(s *inspectionTestStore, _ *candidate.InspectionRequest) { s.item.ActorID = 7 }, 7, candidate.ErrInspectionActor},
		{"伪摘要", func(_ *inspectionTestStore, r *candidate.InspectionRequest) {
			r.EvidenceDigest = strings.Repeat("0", 64)
		}, 8, candidate.ErrEvidenceChanged},
		{"来源失效", func(s *inspectionTestStore, _ *candidate.InspectionRequest) { s.review.unauthorized = true }, 8, sources.ErrUnauthorized},
		{"目录变化", func(s *inspectionTestStore, _ *candidate.InspectionRequest) {
			s.review.catalog.ThemeName = "其他题材"
		}, 8, candidate.ErrEvidenceChanged},
		{"原文变化", func(s *inspectionTestStore, _ *candidate.InspectionRequest) { s.review.snapshot.Body += "已变动" }, 8, nil},
		{"归属变化", func(s *inspectionTestStore, _ *candidate.InspectionRequest) { s.mapping.SourceExcerpt = "其他记录" }, 8, candidate.ErrEvidenceChanged},
		{"归属串号", func(s *inspectionTestStore, _ *candidate.InspectionRequest) { s.mapping.ID = 999 }, 8, candidate.ErrEvidenceChanged},
		{"隐藏后确认", func(s *inspectionTestStore, _ *candidate.InspectionRequest) { s.mapping.Status = 0 }, 8, candidate.ErrEvidenceChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, s, r := newInspectionTestService(t)
			tc.change(s, &r)
			_, err := svc.Decide(context.Background(), tc.actor, r)
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("反例未拒绝: %v", err)
			}
			if len(s.events) != 0 || s.refreshed {
				t.Fatal("反例产生写入")
			}
		})
	}
}

func TestInspectionIssueFailuresRollbackAndConcurrentDecision(t *testing.T) {
	for _, phase := range []string{"mapping", "quarantine", "audit", "count"} {
		t.Run(phase, func(t *testing.T) {
			svc, s, r := newInspectionTestService(t)
			s.failAt = phase
			r.Decision = "issue"
			r.Reason = "测试问题"
			if _, err := svc.Decide(context.Background(), 8, r); err == nil {
				t.Fatal("预期故障")
			}
			if s.mapping.Status != 1 || len(s.events) != 0 || s.refreshed {
				t.Fatal("故障未回滚")
			}
		})
	}
	svc, s, r := newInspectionTestService(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := r
			req.RequestID = fmt.Sprintf("test_concurrent_inspection_%d", i)
			_, err := svc.Decide(context.Background(), 8, req)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	ok, conflict := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, candidate.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflict != 1 || len(s.events) != 1 {
		t.Fatal("并发终态未互斥")
	}
}

func TestInspectionRandomBatchWindowAndIdempotency(t *testing.T) {
	svc, s, _ := newInspectionTestService(t)
	original := s.window[0]
	s.window = nil
	for i := int64(1); i <= 10; i++ {
		e := original
		e.ID = i
		e.Request.RequestID = fmt.Sprintf("test_original_review_%04d", i)
		if i%2 == 0 {
			e.ActorID = 8
		}
		s.window = append(s.window, e)
	}
	// 确定性随机桩选每次范围末尾，验证不放回、排除本人及游标按原窗口而非选中项推进。
	svc.choose = func(n int) (int, error) { return n - 1, nil }
	r := candidate.InspectionBatchRequest{RequestID: "test_inspection_batch_0001", WindowSize: 10, SampleSize: 3}
	b, err := svc.Create(context.Background(), 8, r)
	if err != nil || b.PopulationCount != 10 || b.EligibleCount != 5 || b.NextReviewID != 10 || len(b.Items) != 3 {
		t.Fatalf("抽样错误: %+v %v", b, err)
	}
	seen := map[int64]bool{}
	for _, item := range b.Items {
		if seen[item.ReviewID] || item.ReviewID%2 == 0 {
			t.Fatal("抽样重复或包含本人")
		}
		seen[item.ReviewID] = true
	}
	svc.choose = func(int) (int, error) { t.Fatal("重放重新抽样"); return 0, nil }
	replay, err := svc.Create(context.Background(), 8, r)
	if err != nil || !reflect.DeepEqual(replay, b) {
		t.Fatal(err)
	}
	if _, err := svc.Create(context.Background(), 9, r); !errors.Is(err, candidate.ErrConflict) {
		t.Fatal(err)
	}
	if got, err := svc.Batch(context.Background(), 8, r.RequestID); err != nil || got.ID != b.ID {
		t.Fatal(err)
	}
	if _, err := svc.Batch(context.Background(), 9, r.RequestID); !errors.Is(err, candidate.ErrInspectionActor) {
		t.Fatal(err)
	}
}

func TestInspectionEmptyWindowRandomFailureAndReadGuards(t *testing.T) {
	r := candidate.InspectionBatchRequest{RequestID: "test_inspection_batch_0001", WindowSize: 10, SampleSize: 3}
	for _, phase := range []string{"empty", "window", "batch", "random", "bad-random", "bad-order"} {
		t.Run(phase, func(t *testing.T) {
			svc, s, _ := newInspectionTestService(t)
			switch phase {
			case "empty":
				s.window = nil
			case "random":
				svc.choose = func(int) (int, error) { return 0, errors.New("测试熵源失败") }
			case "bad-random":
				svc.choose = func(int) (int, error) { return -1, nil }
			case "bad-order":
				s.window = append(s.window, s.window[0])
			default:
				s.failAt = phase
			}
			b, err := svc.Create(context.Background(), 8, r)
			if phase == "empty" {
				if err != nil || b.PopulationCount != 0 || len(b.Items) != 0 {
					t.Fatal(err)
				}
			} else if err == nil || s.batch.ID != 0 {
				t.Fatal("异常抽样落了批次")
			}
		})
	}
	svc, _, request := newInspectionTestService(t)
	if _, err := svc.Item(context.Background(), 8, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Item(context.Background(), 9, 1); !errors.Is(err, candidate.ErrInspectionActor) {
		t.Fatal(err)
	}
	if _, err := svc.Item(context.Background(), 8, 2); !errors.Is(err, candidate.ErrInspectionNotFound) {
		t.Fatal(err)
	}
	if _, err := svc.Decide(context.Background(), 0, request); err == nil {
		t.Fatal("无身份未拒绝")
	}
	if _, err := svc.Decide(context.Background(), 8, candidate.InspectionRequest{}); err == nil {
		t.Fatal("坏请求未拒绝")
	}
	if _, err := svc.Issues(context.Background(), 0, 20); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Issues(context.Background(), -1, 20); err == nil {
		t.Fatal("坏分页未拒绝")
	}
}

func TestInspectionSamplingUniformByEnumeration(t *testing.T) {
	// 三选二有六条等概率随机路径；每个无序组合必须恰好出现两次，避免统计随机测试抖动。
	counts := map[string]int{}
	for first := 0; first < 3; first++ {
		for second := 0; second < 2; second++ {
			svc, s, _ := newInspectionTestService(t)
			original := s.window[0]
			s.window = nil
			for id := int64(1); id <= 3; id++ {
				e := original
				e.ID = id
				s.window = append(s.window, e)
			}
			step := 0
			svc.choose = func(n int) (int, error) {
				step++
				if step == 1 {
					if n != 3 {
						t.Fatal("首轮抽样范围错误")
					}
					return first, nil
				}
				if n != 2 {
					t.Fatal("次轮抽样范围错误")
				}
				return second, nil
			}
			b, err := svc.Create(context.Background(), 8, candidate.InspectionBatchRequest{RequestID: "test_enumerated_sample_0001", WindowSize: 3, SampleSize: 2})
			if err != nil {
				t.Fatal(err)
			}
			a, z := b.Items[0].ReviewID, b.Items[1].ReviewID
			if a > z {
				a, z = z, a
			}
			counts[fmt.Sprintf("%d-%d", a, z)]++
		}
	}
	if len(counts) != 3 {
		t.Fatal("抽样未覆盖全部组合")
	}
	for _, n := range counts {
		if n != 2 {
			t.Fatal("抽样组合概率不均匀")
		}
	}
}
