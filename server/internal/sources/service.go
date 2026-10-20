package sources

import (
	"context"
	"fmt"
	"time"
)

// Repository 只提供采集准备所需的只读接口；变更必须由资料管理服务附带审计提交。
type Repository interface {
	SourceByID(context.Context, int64) (Source, error)
	ValidateEnabledSources(context.Context) error
}

type snapshotFetcher interface {
	Fetch(context.Context, Source, string) ([]byte, string, error)
}

// Service 负责授权校验和采集准备，不提供无审计写入入口。
type Service struct {
	repo     Repository
	fetcher  snapshotFetcher
	maxBytes int64
}

// NewService 在生产环境启动时检查全部启用来源的书面授权状态。
func NewService(ctx context.Context, repo Repository, fetcher *Fetcher, production bool) (*Service, error) {
	if repo == nil || fetcher == nil {
		return nil, fmt.Errorf("来源服务依赖不能为空")
	}
	if production {
		if err := repo.ValidateEnabledSources(ctx); err != nil {
			return nil, fmt.Errorf("生产环境来源授权检查失败: %w", err)
		}
	}
	return &Service{repo: repo, fetcher: fetcher, maxBytes: defaultMaxBodyBytes}, nil
}

// PrepareImport 校验并构造快照，不写库；内部管理服务负责与审计原子提交。
func (s *Service) PrepareImport(ctx context.Context, sourceID int64, rawURL, mediaType string, body []byte) (Snapshot, error) {
	if s == nil || s.repo == nil || sourceID <= 0 {
		return Snapshot{}, fmt.Errorf("来源服务或来源编号无效")
	}
	source, err := s.repo.SourceByID(ctx, sourceID)
	if err != nil {
		return Snapshot{}, err
	}
	now := time.Now().UTC()
	if !source.HasValidAuthorization(now) {
		return Snapshot{}, ErrUnauthorized
	}
	snapshot, err := BuildSnapshot(source, rawURL, mediaType, body, now, s.maxBytes)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

// PrepareFetch 在数据库事务外执行有界采集，保存时仍须重新校验当前授权。
func (s *Service) PrepareFetch(ctx context.Context, sourceID int64, rawURL string) (Snapshot, error) {
	if s == nil || s.repo == nil || s.fetcher == nil || sourceID <= 0 {
		return Snapshot{}, fmt.Errorf("来源服务或来源编号无效")
	}
	source, err := s.repo.SourceByID(ctx, sourceID)
	if err != nil {
		return Snapshot{}, err
	}
	body, mediaType, err := s.fetcher.Fetch(ctx, source, rawURL)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot, err := BuildSnapshot(source, rawURL, mediaType, body, time.Now().UTC(), s.maxBytes)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}
