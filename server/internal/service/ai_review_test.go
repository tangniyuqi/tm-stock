package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// 内存事务只用于验证 Service 的状态机与失败原子性，不代替 MySQL 集成验收。
type reviewTestStore struct {
	mu           sync.Mutex
	record       candidate.ReviewRecord
	snapshot     sources.Snapshot
	catalog      candidate.ReviewCatalog
	audits       map[string]candidate.AuditEvent
	mappingID    int64
	unauthorized bool
	failAudit    bool
	failCount    bool
	refreshed    bool
}

type reviewTestTx struct{ store *reviewTestStore }

func newReviewTestService(t *testing.T) (*AIReviewService, *reviewTestStore, candidate.ReviewRequest) {
	t.Helper()
	quote := "星河科技在年报中披露主营算力服务。"
	source := sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryAnnualReport,
		LicenseTag: "test-license", LicenseRef: "test-written-authorization", Enabled: true}
	snapshot, err := sources.BuildSnapshot(source, "https://example.test/report", "text/plain; charset=utf-8", []byte(quote), time.Now().UTC(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = 1
	item := candidate.Candidate{ID: 1, SnapshotID: 1, ThemeID: 2, ThemeKey: "算力", StockID: 3, EntityKey: "星河科技", TsCode: "688001.SH",
		Predicate: "DISCLOSES_BUSINESS", SourceType: 2, SourceExcerpt: quote, SentenceIDs: []int{1}, QuoteSegments: []string{quote},
		SourceURL: snapshot.URL, CollectedAt: snapshot.FetchedAt, Provider: "test-provider", Model: "test-model", PromptVersion: "l0-extraction-v1"}
	store := &reviewTestStore{record: candidate.ReviewRecord{Candidate: item}, snapshot: snapshot,
		catalog: candidate.ReviewCatalog{ThemeName: "算力", StockName: "星河科技", TsCode: "688001.SH"}, audits: map[string]candidate.AuditEvent{}}
	svc, err := NewAIReviewService(store)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := candidate.EvidenceDigest(item)
	if err != nil {
		t.Fatal(err)
	}
	return svc, store, candidate.ReviewRequest{CandidateID: 1, EvidenceDigest: digest, Decision: "approve", RequestID: "test_request_00001"}
}

func (s *reviewTestStore) WithinReviewTx(ctx context.Context, action func(candidate.ReviewTx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	copyStore := &reviewTestStore{record: s.record, snapshot: s.snapshot, catalog: s.catalog, mappingID: s.mappingID,
		unauthorized: s.unauthorized, failAudit: s.failAudit, failCount: s.failCount, refreshed: s.refreshed, audits: map[string]candidate.AuditEvent{}}
	for id, event := range s.audits {
		copyStore.audits[id] = event
	}
	if err := action(&reviewTestTx{copyStore}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.record, s.audits, s.mappingID, s.refreshed = copyStore.record, copyStore.audits, copyStore.mappingID, copyStore.refreshed
	return nil
}

func (s *reviewTestStore) ListReviewQueue(ctx context.Context, afterID int64, limit int) ([]candidate.ReviewRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.record.Status == candidate.Pending && s.record.Candidate.ID > afterID {
		return []candidate.ReviewRecord{s.record}, nil
	}
	return []candidate.ReviewRecord{}, nil
}

func (t *reviewTestTx) LockCandidate(_ context.Context, id int64) (candidate.ReviewRecord, error) {
	if id != t.store.record.Candidate.ID {
		return candidate.ReviewRecord{}, candidate.ErrNotFound
	}
	return t.store.record, nil
}
func (t *reviewTestTx) AuditByRequest(_ context.Context, id string) (candidate.AuditEvent, bool, error) {
	e, ok := t.store.audits[id]
	return e, ok, nil
}
func (t *reviewTestTx) ApprovalSnapshot(context.Context, candidate.Candidate) (sources.Snapshot, error) {
	if t.store.unauthorized {
		return sources.Snapshot{}, sources.ErrUnauthorized
	}
	return t.store.snapshot, nil
}
func (t *reviewTestTx) LockCatalog(context.Context, candidate.Candidate) (candidate.ReviewCatalog, error) {
	return t.store.catalog, nil
}
func (t *reviewTestTx) InsertMapping(context.Context, candidate.Candidate, int64) (int64, error) {
	if t.store.mappingID != 0 {
		return 0, candidate.ErrConflict
	}
	t.store.mappingID = 100
	return 100, nil
}
func (t *reviewTestTx) AppendAudit(_ context.Context, request candidate.ReviewRequest, actorID, mappingID int64) (candidate.AuditEvent, error) {
	if t.store.failAudit {
		return candidate.AuditEvent{}, fmt.Errorf("测试审计写入故障")
	}
	event := candidate.AuditEvent{ID: 1, Request: request, ActorID: actorID, MappingID: mappingID, CreatedAt: time.Now().UTC()}
	t.store.audits[request.RequestID] = event
	t.store.record.Status = request.Status()
	t.store.record.MappingID = mappingID
	return event, nil
}
func (t *reviewTestTx) RefreshThemeCount(context.Context, int64) error {
	if t.store.failCount {
		return fmt.Errorf("测试计数写入故障")
	}
	t.store.refreshed = true
	return nil
}

func TestAIReviewApproveAndReplay(t *testing.T) {
	svc, store, request := newReviewTestService(t)
	event, err := svc.Review(context.Background(), 7, request)
	if err != nil || event.ID != 1 || event.MappingID != 100 || store.record.Status != candidate.Approved || !store.refreshed {
		t.Fatalf("通过事务失败: event=%+v err=%v", event, err)
	}
	// 授权后来失效不能把既已完成的请求重放变成新的写入。
	store.unauthorized = true
	replay, err := svc.Review(context.Background(), 7, request)
	if err != nil || replay != event || len(store.audits) != 1 {
		t.Fatalf("重放不幂等: %+v %v", replay, err)
	}
	if _, err := svc.Review(context.Background(), 8, request); !errors.Is(err, candidate.ErrConflict) {
		t.Fatalf("不同身份重放未拒绝: %v", err)
	}
	request.Decision, request.Reason = "reject", "改载荷重放"
	if _, err := svc.Review(context.Background(), 7, request); !errors.Is(err, candidate.ErrConflict) {
		t.Fatalf("不同决定重放未拒绝: %v", err)
	}
	request.RequestID = "test_request_00002"
	if _, err := svc.Review(context.Background(), 7, request); !errors.Is(err, candidate.ErrConflict) {
		t.Fatalf("终态重审未拒绝: %v", err)
	}
}

// 从真实快照与守卫结果构造节选，不能只把普通引文的布尔值改成 true。
func setTrimmedReviewFixture(t *testing.T, store *reviewTestStore, body string, ids []int) {
	t.Helper()
	old := store.snapshot
	source := sources.Source{ID: old.SourceID, Domain: "example.test", Category: old.SourceCategory, LicenseTag: old.LicenseTag, LicenseRef: old.LicenseRef, Enabled: true}
	snapshot, err := sources.BuildSnapshot(source, old.URL, "text/plain; charset=utf-8", []byte(body), old.FetchedAt, 4096)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = old.ID
	c := store.record.Candidate
	claim := guard.Claim{Predicate: c.Predicate, Entity: c.EntityKey, Theme: c.ThemeKey, SentenceIDs: ids}
	raw, err := json.Marshal(struct {
		Claims []guard.Claim `json:"claims"`
	}{[]guard.Claim{claim}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := guard.ValidateExtraction(raw, guard.NewSnapshot(snapshot.Body), guard.Vocabulary{Predicates: map[string]bool{c.Predicate: true}, Themes: map[string][]string{c.ThemeKey: {c.ThemeKey}}})
	if err != nil || len(report.Accepted) != 1 {
		t.Fatalf("测试节选未通过守卫: %+v %v", report, err)
	}
	a := report.Accepted[0]
	c.SourceExcerpt, c.QuoteSegments, c.SentenceIDs, c.ExcerptTrimmed = a.Quote, a.Segments, ids, a.Trimmed || len(a.Segments) > 1
	if !c.ExcerptTrimmed {
		t.Fatal("测试装置未产生节选")
	}
	store.snapshot, store.record.Candidate = snapshot, c
}

func TestAIReviewAllowsVerifiedTrimmedEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		ids        []int
	}{
		{"不连续引文", "星河科技在年报中披露主营算力服务。公司披露其他业务事项。算力服务包括数据中心运营。", []int{1, 3}},
		{"红线句剔除", "星河科技在年报中披露主营算力服务。建议买入。", []int{1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, store, request := newReviewTestService(t)
			setTrimmedReviewFixture(t, store, tc.body, tc.ids)
			request.EvidenceDigest, _ = candidate.EvidenceDigest(store.record.Candidate)
			event, err := svc.Review(context.Background(), 7, request)
			if err != nil || event.MappingID == 0 {
				t.Fatalf("可核对节选不能审核: %+v %v", event, err)
			}
		})
	}
	t.Run("不能去掉节选标记", func(t *testing.T) {
		svc, store, request := newReviewTestService(t)
		setTrimmedReviewFixture(t, store, "星河科技在年报中披露主营算力服务。建议买入。", []int{1, 2})
		store.record.Candidate.ExcerptTrimmed = false
		request.EvidenceDigest, _ = candidate.EvidenceDigest(store.record.Candidate)
		if _, err := svc.Review(context.Background(), 7, request); err == nil || store.mappingID != 0 || len(store.audits) != 0 {
			t.Fatal("去掉标记仍可通过或留下部分写入")
		}
	})
}

func TestAIReviewRejectAfterSourceDisabled(t *testing.T) {
	svc, store, request := newReviewTestService(t)
	store.unauthorized = true
	request.Decision, request.Reason = "reject", "原文未明确支持归属"
	event, err := svc.Review(context.Background(), 7, request)
	if err != nil || event.MappingID != 0 || store.mappingID != 0 || store.record.Status != candidate.Rejected || store.refreshed {
		t.Fatalf("驳回错误产生归属: %+v %v", event, err)
	}
	rows, err := svc.ListPending(context.Background(), 0, 20)
	if err != nil || len(rows) != 0 {
		t.Fatalf("已审候选仍在队列: %v", err)
	}
}

func TestAIReviewFailureRollsBackEverything(t *testing.T) {
	cases := []struct {
		name   string
		change func(*reviewTestStore, *candidate.ReviewRequest)
		want   error
	}{
		{"错证据摘要", func(s *reviewTestStore, r *candidate.ReviewRequest) { r.EvidenceDigest = strings.Repeat("0", 64) }, candidate.ErrEvidenceChanged},
		{"授权失效", func(s *reviewTestStore, r *candidate.ReviewRequest) { s.unauthorized = true }, sources.ErrUnauthorized},
		{"主数据变化", func(s *reviewTestStore, r *candidate.ReviewRequest) { s.catalog.StockName = "另一个公司" }, candidate.ErrEvidenceChanged},
		{"原文被替换", func(s *reviewTestStore, r *candidate.ReviewRequest) { s.snapshot.Body = "其他内容" }, nil},
		{"伪造节选标记", func(s *reviewTestStore, r *candidate.ReviewRequest) {
			s.record.Candidate.ExcerptTrimmed = true
			r.EvidenceDigest, _ = candidate.EvidenceDigest(s.record.Candidate)
		}, nil},
		{"已有归属", func(s *reviewTestStore, r *candidate.ReviewRequest) { s.mappingID = 99 }, candidate.ErrConflict},
		{"审计写入故障", func(s *reviewTestStore, r *candidate.ReviewRequest) { s.failAudit = true }, nil},
		{"计数写入故障", func(s *reviewTestStore, r *candidate.ReviewRequest) { s.failCount = true }, nil},
		{"候选不存在", func(s *reviewTestStore, r *candidate.ReviewRequest) { r.CandidateID++ }, candidate.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store, request := newReviewTestService(t)
			tc.change(store, &request)
			before := store.mappingID
			event, err := svc.Review(context.Background(), 7, request)
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("反例未被阻止: %v", err)
			}
			if event.ID != 0 || store.mappingID != before || len(store.audits) != 0 || store.record.Status != candidate.Pending || store.refreshed {
				t.Fatal("失败留下部分写入")
			}
		})
	}
}

func TestAIReviewConcurrentDecisions(t *testing.T) {
	svc, store, first := newReviewTestService(t)
	second := first
	second.RequestID = "test_request_00002"
	second.Decision, second.Reason = "reject", "证据不足"
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, request := range []candidate.ReviewRequest{first, second} {
		wg.Add(1)
		go func(r candidate.ReviewRequest) {
			defer wg.Done()
			_, err := svc.Review(context.Background(), 7, r)
			results <- err
		}(request)
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, candidate.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 || len(store.audits) != 1 {
		t.Fatalf("并发审核未唯一终态: 成功=%d 冲突=%d", successes, conflicts)
	}
}

func TestAIReviewInvalidAndCancelledRequests(t *testing.T) {
	svc, store, request := newReviewTestService(t)
	if _, err := NewAIReviewService(nil); err == nil {
		t.Fatal("空仓储未拒绝")
	}
	if _, err := svc.Review(context.Background(), 0, request); !errors.Is(err, candidate.ErrInvalidReview) {
		t.Fatal(err)
	}
	bad := request
	bad.Decision = "unknown"
	if _, err := svc.Review(context.Background(), 7, bad); !errors.Is(err, candidate.ErrInvalidReview) {
		t.Fatal(err)
	}
	for _, args := range [][2]int{{-1, 20}, {0, 0}, {0, 201}} {
		if _, err := svc.ListPending(context.Background(), int64(args[0]), args[1]); !errors.Is(err, candidate.ErrInvalidReview) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Review(ctx, 7, request); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(store.audits) != 0 || store.mappingID != 0 {
		t.Fatal("无效请求产生写入")
	}
}
