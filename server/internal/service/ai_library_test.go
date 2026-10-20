package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

var errLibraryFixture = errors.New("测试事务失败")

type libraryFake struct {
	mu                sync.Mutex
	source            sources.Source
	snapshot          sources.Snapshot
	events            []sources.SourceAudit
	fail              string
	prepared, txCount int
	prepareHook       func()
}

func libraryFixture() *libraryFake {
	return &libraryFake{source: sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryAnnualReport,
		LicenseTag: "test-authorization", LicenseRef: "test-fixture:written", MinInterval: time.Second, Enabled: true}}
}
func (f *libraryFake) WithinSourceTx(ctx context.Context, action func(sources.MutationTx) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.txCount++
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.fail == "begin" {
		return errLibraryFixture
	}
	beforeSource, beforeSnapshot, beforeEvents := f.source, f.snapshot, append([]sources.SourceAudit(nil), f.events...)
	err := action(&libraryFakeTx{f})
	if err == nil && f.fail == "commit" {
		err = errLibraryFixture
	}
	if err != nil {
		f.source, f.snapshot, f.events = beforeSource, beforeSnapshot, beforeEvents
	}
	return err
}

type libraryFakeTx struct{ f *libraryFake }

func (t *libraryFakeTx) SourceByID(context.Context, int64) (sources.Source, error) {
	if t.f.fail == "read" {
		return sources.Source{}, errLibraryFixture
	}
	return t.f.source, nil
}
func (t *libraryFakeTx) RegisterSource(_ context.Context, s sources.Source) (sources.Source, error) {
	if t.f.fail == "register" {
		return sources.Source{}, errLibraryFixture
	}
	s.ID = 1
	t.f.source = s
	return s, nil
}
func (t *libraryFakeTx) SetSourceEnabled(_ context.Context, _ int64, enabled bool) error {
	if t.f.fail == "state" {
		return errLibraryFixture
	}
	if enabled {
		s := t.f.source
		s.Enabled = true
		if _, err := s.NormalizeRegistration(); err != nil {
			return err
		}
	}
	t.f.source.Enabled = enabled
	return nil
}
func (t *libraryFakeTx) SaveSnapshot(_ context.Context, s sources.Snapshot) (sources.Snapshot, error) {
	if t.f.fail == "save" {
		return sources.Snapshot{}, errLibraryFixture
	}
	if !t.f.source.HasValidAuthorization(time.Now()) || t.f.source.LicenseRef != s.LicenseRef {
		return sources.Snapshot{}, sources.ErrUnauthorized
	}
	if t.f.snapshot.Hash == s.Hash {
		return t.f.snapshot, nil
	}
	s.ID = 2
	t.f.snapshot = s
	return s, nil
}
func (t *libraryFakeTx) AppendSourceAudit(_ context.Context, e sources.SourceAudit) error {
	if t.f.fail == "audit" {
		return errLibraryFixture
	}
	e.ID = int64(len(t.f.events) + 1)
	e.CreatedAt = time.Now().UTC()
	t.f.events = append(t.f.events, e)
	return nil
}
func (f *libraryFake) SourceByID(context.Context, int64) (sources.Source, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == "read" {
		return sources.Source{}, errLibraryFixture
	}
	return f.source, nil
}
func (f *libraryFake) SnapshotByID(context.Context, int64) (sources.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshot, nil
}
func (f *libraryFake) ListSourceAudits(_ context.Context, _ int64, after int64, limit int) ([]sources.SourceAudit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == "list" {
		return nil, errLibraryFixture
	}
	out := []sources.SourceAudit{}
	for _, e := range f.events {
		if e.ID > after && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *libraryFake) PrepareImport(ctx context.Context, id int64, url, media string, body []byte) (sources.Snapshot, error) {
	f.mu.Lock()
	f.prepared++
	source, fail, hook := f.source, f.fail, f.prepareHook
	f.mu.Unlock()
	if hook != nil {
		hook()
	} // 事务外执行，测试通过此钩子模拟采集期间撤销授权。
	if fail == "prepare" {
		return sources.Snapshot{}, errLibraryFixture
	}
	if err := ctx.Err(); err != nil {
		return sources.Snapshot{}, err
	}
	if fail == "wrong-source" {
		source.ID++
	}
	return sources.BuildSnapshot(source, url, media, body, time.Now().UTC(), 2<<20)
}
func (f *libraryFake) PrepareFetch(ctx context.Context, id int64, url string) (sources.Snapshot, error) {
	return f.PrepareImport(ctx, id, url, "text/plain", []byte("测试资料：公司披露主营服务。"))
}

func TestLibraryServiceAuditedOperations(t *testing.T) {
	f := libraryFixture()
	svc, err := NewAILibraryService(f, f)
	if err != nil {
		t.Fatal(err)
	}
	source := f.source
	source.ID = 0
	source.Enabled = false
	source.MinInterval = 0
	source.Domain = "EXAMPLE.TEST."
	created, err := svc.RegisterSource(context.Background(), 7, source)
	if err != nil || created.Enabled || created.Domain != "example.test" || created.MinInterval != time.Second {
		t.Fatalf("登记错误: %+v %v", created, err)
	}
	if err := svc.EnableSource(context.Background(), 8, 1, true); err != nil {
		t.Fatal(err)
	}
	first, err := svc.ImportText(context.Background(), 9, 1, "https://example.test/report", "text/plain", []byte("测试资料：公司披露主营服务。"))
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := svc.FetchSnapshot(context.Background(), 10, 1, "https://example.test/report")
	if err != nil || duplicate.ID != first.ID || !duplicate.FetchedAt.Equal(first.FetchedAt) {
		t.Fatalf("重复资料未复用快照: %v", err)
	}
	if err := svc.EnableSource(context.Background(), 11, 1, false); err != nil {
		t.Fatal(err)
	}
	for i, operation := range []string{"register", "enable", "import", "fetch", "disable"} {
		e := f.events[i]
		if e.Operation != operation || e.ActorID != int64(i+7) || e.Validate() != nil {
			t.Fatalf("审计身份/内容错误: %+v", e)
		}
	}
	if !f.events[4].Before.Enabled || f.events[4].After.Enabled || f.events[1].Before.Enabled {
		t.Fatal("启停前后副本错误")
	}
	if got, err := svc.SourceAudits(context.Background(), 1, 2, 2); err != nil || len(got) != 2 || got[0].ID != 3 {
		t.Fatalf("分页错误: %v", err)
	}
	if _, err := svc.SourceByID(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SnapshotByID(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryServiceFailClosedAndRollback(t *testing.T) {
	for _, operation := range []string{"register", "enable", "disable", "import", "fetch"} {
		failures := []string{"begin", "audit", "commit"}
		switch operation {
		case "register":
			failures = append(failures, "register")
		case "enable", "disable":
			failures = append(failures, "read", "state")
		case "import", "fetch":
			failures = append(failures, "prepare", "read", "save")
		}
		for _, failure := range failures {
			t.Run(operation+"/"+failure, func(t *testing.T) {
				f := libraryFixture()
				f.fail = failure
				before := f.source
				svc, _ := NewAILibraryService(f, f)
				var err error
				switch operation {
				case "register":
					source := f.source
					source.ID = 0
					source.Enabled = false
					_, err = svc.RegisterSource(context.Background(), 7, source)
				case "enable", "disable":
					err = svc.EnableSource(context.Background(), 7, 1, operation == "enable")
				case "import":
					_, err = svc.ImportText(context.Background(), 7, 1, "https://example.test/report", "text/plain", []byte("测试资料。"))
				case "fetch":
					_, err = svc.FetchSnapshot(context.Background(), 7, 1, "https://example.test/report")
				}
				if !errors.Is(err, errLibraryFixture) || f.source != before || f.snapshot.ID != 0 || len(f.events) != 0 {
					t.Fatalf("失败未回滚: err=%v %+v", err, f)
				}
			})
		}
	}
}

func TestLibraryServiceValidationAndUnauthorizedUse(t *testing.T) {
	f := libraryFixture()
	svc, _ := NewAILibraryService(f, f)
	for _, s := range []sources.Source{{Domain: "127.0.0.1", Category: sources.CategoryAnnualReport}, {ID: 1}, {Enabled: true}, {MinInterval: -1}, {MinInterval: 25 * time.Hour}} {
		if _, err := svc.RegisterSource(context.Background(), 7, s); !errors.Is(err, ErrInvalidLibraryRequest) {
			t.Fatal("错误来源参数未拒绝")
		}
	}
	for _, actor := range []int64{0, -1} {
		if _, err := svc.RegisterSource(context.Background(), actor, sources.Source{}); !errors.Is(err, ErrInvalidLibraryRequest) {
			t.Fatal(err)
		}
		if err := svc.EnableSource(context.Background(), actor, 1, true); !errors.Is(err, ErrInvalidLibraryRequest) {
			t.Fatal(err)
		}
		if _, err := svc.ImportText(context.Background(), actor, 1, "", "", []byte("x")); !errors.Is(err, ErrInvalidLibraryRequest) {
			t.Fatal(err)
		}
		if _, err := svc.FetchSnapshot(context.Background(), actor, 1, ""); !errors.Is(err, ErrInvalidLibraryRequest) {
			t.Fatal(err)
		}
	}
	if _, err := svc.ImportText(context.Background(), 7, 1, "", "", []byte(strings.Repeat("x", 2<<20+1))); !errors.Is(err, sources.ErrBodyTooLarge) {
		t.Fatal(err)
	}
	if err := svc.EnableSource(context.Background(), 7, 0, true); !errors.Is(err, ErrInvalidLibraryRequest) {
		t.Fatal(err)
	}
	if _, err := svc.SourceByID(context.Background(), 0); !errors.Is(err, ErrInvalidLibraryRequest) {
		t.Fatal(err)
	}
	if _, err := svc.SnapshotByID(context.Background(), 0); !errors.Is(err, ErrInvalidLibraryRequest) {
		t.Fatal(err)
	}
	if _, err := svc.ImportText(context.Background(), 7, 0, "", "", nil); !errors.Is(err, ErrInvalidLibraryRequest) {
		t.Fatal(err)
	}
	if _, err := svc.FetchSnapshot(context.Background(), 7, 0, ""); !errors.Is(err, ErrInvalidLibraryRequest) {
		t.Fatal(err)
	}
	for _, params := range [][3]int64{{0, 0, 1}, {1, -1, 1}, {1, 0, 0}, {1, 0, 201}} {
		if _, err := svc.SourceAudits(context.Background(), params[0], params[1], int(params[2])); !errors.Is(err, ErrInvalidLibraryRequest) {
			t.Fatal(err)
		}
	}
	if f.txCount != 0 || f.prepared != 0 {
		t.Fatal("无效参数仍触发操作")
	}
	if _, err := NewAILibraryService(nil, f); err == nil {
		t.Fatal("允许空依赖")
	}
	if _, err := NewAILibraryService(f, nil); err == nil {
		t.Fatal("允许空仓储")
	}
	f.source.Enabled = false
	if _, err := svc.FetchSnapshot(context.Background(), 7, 1, "https://example.test/report"); !errors.Is(err, sources.ErrUnauthorized) {
		t.Fatal(err)
	}
	f.source.Enabled = true
	f.prepareHook = func() { f.mu.Lock(); f.source.Enabled = false; f.mu.Unlock() }
	if _, err := svc.FetchSnapshot(context.Background(), 7, 1, "https://example.test/report"); !errors.Is(err, sources.ErrUnauthorized) || len(f.events) != 0 {
		t.Fatal("采集期间授权失效仍落库")
	}
	f.prepareHook = nil
	f.source.Enabled = true
	f.fail = "wrong-source"
	if _, err := svc.FetchSnapshot(context.Background(), 7, 1, "https://example.test/report"); !errors.Is(err, sources.ErrInvalidSourceAudit) {
		t.Fatal("错绑来源仍保存")
	}
	f.fail = "list"
	if _, err := svc.SourceAudits(context.Background(), 1, 0, 10); !errors.Is(err, errLibraryFixture) {
		t.Fatal(err)
	}
	f.fail = "read"
	if _, err := svc.SourceAudits(context.Background(), 1, 0, 10); !errors.Is(err, errLibraryFixture) {
		t.Fatal(err)
	}
}

func TestLibraryConcurrentStateChangesHaveContinuousAudit(t *testing.T) {
	f := libraryFixture()
	svc, _ := NewAILibraryService(f, f)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := svc.EnableSource(context.Background(), int64(i+1), 1, i%2 == 0); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if len(f.events) != 20 {
		t.Fatal("并发操作丢失审计")
	}
	previous := true
	for _, e := range f.events {
		if e.Before.Enabled != previous {
			t.Fatal("审计变更链断裂")
		}
		previous = e.After.Enabled
	}
	if f.source.Enabled != previous {
		t.Fatal("状态与审计不一致")
	}
}
