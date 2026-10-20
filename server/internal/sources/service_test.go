package sources

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryRepository struct {
	sources            map[int64]Source
	snapshots          []Snapshot
	productionCheckErr error
	productionChecks   int
}

func (r *memoryRepository) SourceByID(_ context.Context, id int64) (Source, error) {
	source, ok := r.sources[id]
	if !ok {
		return Source{}, ErrSourceUnavailable
	}
	return source, nil
}

func (r *memoryRepository) ValidateEnabledSources(_ context.Context) error {
	r.productionChecks++
	return r.productionCheckErr
}

func newAuthorizedMemoryRepo(now time.Time) *memoryRepository {
	source := authorizedTestSource(now)
	source.Domain = "example.test"
	return &memoryRepository{sources: map[int64]Source{source.ID: source}}
}

func TestNewServiceChecksAuthorizationOnlyForProduction(t *testing.T) {
	repo := newAuthorizedMemoryRepo(time.Now().UTC())
	repo.productionCheckErr = errors.New("授权缺失")
	if _, err := NewService(context.Background(), repo, NewFetcher(), true); err == nil {
		t.Fatal("生产环境授权检查失败时应拒绝启动")
	}
	if repo.productionChecks != 1 {
		t.Fatalf("生产校验次数 = %d, want 1", repo.productionChecks)
	}
	repo.productionChecks = 0
	if _, err := NewService(context.Background(), repo, NewFetcher(), false); err != nil {
		t.Fatalf("开发环境不应执行生产校验: %v", err)
	}
	if repo.productionChecks != 0 {
		t.Fatalf("开发环境校验次数 = %d, want 0", repo.productionChecks)
	}
}

func TestPrepareImportRequiresAuthorizationAndDoesNotPersist(t *testing.T) {
	now := time.Now().UTC()
	repo := newAuthorizedMemoryRepo(now)
	service, err := NewService(context.Background(), repo, NewFetcher(), false)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.PrepareImport(context.Background(), 7, "https://example.test/annual-report", "text/plain; charset=utf-8", []byte("公司在年度报告中披露营业收入。"))
	if err != nil {
		t.Fatalf("授权来源导入失败: %v", err)
	}
	if snapshot.ID != 0 || len(repo.snapshots) != 0 || snapshot.Hash == "" {
		t.Fatalf("准备步骤不应写入仓储: snapshot=%#v stored=%#v", snapshot, repo.snapshots)
	}

	for _, test := range []struct {
		name   string
		source Source
	}{
		{"来源关闭", func() Source { s := repo.sources[7]; s.Enabled = false; return s }()},
		{"缺授权引用", func() Source { s := repo.sources[7]; s.LicenseRef = ""; return s }()},
		{"授权已过期", func() Source {
			s := repo.sources[7]
			expired := now.Add(-time.Second)
			s.LicenseExpires = &expired
			return s
		}()},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo.sources[7] = test.source
			before := len(repo.snapshots)
			if _, err := service.PrepareImport(context.Background(), 7, "https://example.test/other-report", "text/plain", []byte("另一份公告正文。")); !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("未授权来源应拒绝，得到 %v", err)
			}
			if len(repo.snapshots) != before {
				t.Fatal("未授权来源不应写入快照")
			}
			repo.sources[7] = newAuthorizedMemoryRepo(now).sources[7]
		})
	}
}

type preparedFetchStub struct {
	body  []byte
	media string
	err   error
}

func (f preparedFetchStub) Fetch(context.Context, Source, string) ([]byte, string, error) {
	return f.body, f.media, f.err
}

func TestPrepareFetchAndImportBoundaries(t *testing.T) {
	repo := newAuthorizedMemoryRepo(time.Now().UTC())
	svc, err := NewService(context.Background(), repo, NewFetcher(), false)
	if err != nil {
		t.Fatal(err)
	}
	svc.fetcher = preparedFetchStub{body: []byte("年度报告披露主营服务。"), media: "text/plain"}
	snapshot, err := svc.PrepareFetch(context.Background(), 7, "https://example.test/report")
	if err != nil || snapshot.ID != 0 || snapshot.Validate() != nil || len(repo.snapshots) != 0 {
		t.Fatalf("采集准备不应写库: %v", err)
	}
	svc.fetcher = preparedFetchStub{err: ErrFetchFailed}
	if _, err := svc.PrepareFetch(context.Background(), 7, "https://example.test/report"); !errors.Is(err, ErrFetchFailed) {
		t.Fatal(err)
	}
	svc.fetcher = preparedFetchStub{body: []byte("正文。"), media: "application/pdf"}
	if _, err := svc.PrepareFetch(context.Background(), 7, "https://example.test/report"); !errors.Is(err, ErrUnsupportedMedia) {
		t.Fatal(err)
	}
	if _, err := svc.PrepareImport(context.Background(), 7, "https://other.test/report", "text/plain", []byte("正文。")); !errors.Is(err, ErrUnsafeURL) {
		t.Fatal(err)
	}
	for _, id := range []int64{0, 99} {
		if _, err := svc.PrepareImport(context.Background(), id, "", "", nil); err == nil {
			t.Fatal("无效来源未拒绝")
		}
		if _, err := svc.PrepareFetch(context.Background(), id, ""); err == nil {
			t.Fatal("无效来源未拒绝")
		}
	}
	for _, empty := range []*Service{nil, {}} {
		if _, err := empty.PrepareImport(context.Background(), 7, "", "", nil); err == nil {
			t.Fatal("空服务未拒绝")
		}
		if _, err := empty.PrepareFetch(context.Background(), 7, ""); err == nil {
			t.Fatal("空服务未拒绝")
		}
	}
	if _, err := NewService(context.Background(), nil, NewFetcher(), false); err == nil {
		t.Fatal("空仓储未拒绝")
	}
	if _, err := NewService(context.Background(), repo, nil, false); err == nil {
		t.Fatal("空采集器未拒绝")
	}
}
