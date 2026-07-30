// Package service 是业务编排层。
//
// 依赖方向：handler → service → repository（单向）。
// 本层【不得】依赖 gin 或任何 HTTP 概念（check-architecture.sh 规则 3）。
//
// 接口按 Go 惯例定义在【消费方】，因此本包只依赖 model，不依赖 repository 包。
package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/model"
)

// ErrThemeNotFound 题材不存在。
var ErrThemeNotFound = errors.New("题材不存在")

// ErrEvidenceNotFound 依据不存在。
//
// 出现该错误的正确响应是 404，**不是**返回一个空依据结构。
// 空依据等于对用户承认"我们也不知道为什么把这家公司归到这里"。
var ErrEvidenceNotFound = errors.New("归属依据不存在")

// ErrAccessDenied 无权限查看付费内容。
//
// 与 ErrEvidenceNotFound 分开是有意的：用户点了个股想看依据，
// 此时应引导订阅（403 + 订阅入口），而不是告诉他"没有依据"——
// 后者会让用户以为数据缺失，是错误的信息。
var ErrAccessDenied = errors.New("无权限查看付费内容")

// ThemeRepository 题材数据访问。
//
// ListApprovedMappings 的命名带 Approved 是刻意的：
// 状态过滤属于【数据访问层的基础约束】，不是调用方的可选项。
// 若做成 ListMappings(status) 让调用方传参，漏传一次就是一次泄漏。
type ThemeRepository interface {
	GetTheme(ctx context.Context, id int64) (*model.Theme, error)
	GetCategoryPath(ctx context.Context, categoryID int64) (string, error)
	ListChainNodes(ctx context.Context, themeID int64) ([]model.ChainNode, error)
	ListApprovedMappings(ctx context.Context, themeID int64) ([]model.ThemeStockMapping, error)
	ListStocks(ctx context.Context, codes []string) (map[string]model.Stock, error)
	ListEvents(ctx context.Context, themeID int64) ([]model.ThemeEvent, error)
	FindApprovedMapping(ctx context.Context, themeID int64, stockCode string) (*model.ThemeStockMapping, error)
	GetChainNodeName(ctx context.Context, nodeID int64) (string, error)
}

// QuoteProvider 行情提供者（延时 15 分钟）。
//
// 返回值用 *float64：nil 表示【该股当前无行情数据】。
// 实现方在拉取失败时必须返回 nil，**不得**返回 0 或上一次的旧值。
type QuoteProvider interface {
	BatchChangePct(ctx context.Context, codes []string) (map[string]*float64, time.Time, error)
}

// QuoteConfig 行情开关。
//
// 一期 Enabled=false（ADR-0006：涨跌幅不是差异化，且需行情商用授权）。
// 关闭时所有涨跌幅返回 nil，页面显示「—」。
type QuoteConfig struct {
	Enabled  bool
	DelayMin int  // 启用时应为 15
	IsMock   bool // true 时接口置 QuoteIsMock，前端必须显著标注「示例数据」
}

// Access 调用方的权限上下文，由上层（handler + 会员服务）计算后传入。
// service 不自己查会员表——那会让本层同时承担鉴权与编排两件事。
type Access struct {
	CanViewPaid bool // 已订阅，或本次命中试吃
	TrialLeft   int
}

// ThemeService 题材查询服务。
type ThemeService struct {
	repo  ThemeRepository
	quote QuoteProvider
	cfg   QuoteConfig
}

// NewThemeService 构造。quote 允许为 nil（cfg.Enabled=false 时不会被调用）。
func NewThemeService(repo ThemeRepository, quote QuoteProvider, cfg QuoteConfig) *ThemeService {
	return &ThemeService{repo: repo, quote: quote, cfg: cfg}
}

// GetDetail 组装题材详情。
//
// 本方法集中了三条合规逻辑，是全项目最需要测试覆盖的地方：
//  1. 证据不全的映射【不返回】（第三重防线中唯一可单测的一层）
//  2. 无权限时付费层【置 nil 且 Locked=true】，不返回空壳
//  3. 无行情数据时 ChangePct 为 nil，【绝不填 0】
func (s *ThemeService) GetDetail(ctx context.Context, themeID int64, access Access) (*dto.ThemeDetailResp, error) {
	theme, err := s.repo.GetTheme(ctx, themeID)
	if err != nil {
		return nil, err
	}
	if theme == nil {
		return nil, ErrThemeNotFound
	}

	path, err := s.repo.GetCategoryPath(ctx, theme.CategoryID)
	if err != nil {
		return nil, err
	}

	resp := &dto.ThemeDetailResp{
		ID:            theme.ID,
		Name:          theme.Name,
		CategoryPath:  path,
		Summary:       theme.Summary,
		UpdatedAt:     toMillis(theme.UpdatedAt),
		DataSource:    theme.DataSource,
		QuoteEnabled:  s.cfg.Enabled,
		QuoteDelayMin: s.cfg.DelayMin,
		QuoteIsMock:   s.cfg.Enabled && s.cfg.IsMock,
		TrialLeft:     access.TrialLeft,
	}

	// 顶层免费；中层及以下需权限。无权限直接返回，付费字段保持零值（nil 切片）。
	if !access.CanViewPaid {
		resp.Locked = true
		return resp, nil
	}

	nodes, err := s.repo.ListChainNodes(ctx, themeID)
	if err != nil {
		return nil, err
	}
	mappings, err := s.repo.ListApprovedMappings(ctx, themeID)
	if err != nil {
		return nil, err
	}

	// ── 合规逻辑 1：过滤证据不全的映射 ──
	valid := make([]model.ThemeStockMapping, 0, len(mappings))
	for _, m := range mappings {
		if m.HasCompleteEvidence() && m.IsVisibleToPublic() {
			valid = append(valid, m)
		}
	}

	codes := make([]string, 0, len(valid))
	for _, m := range valid {
		codes = append(codes, m.StockCode)
	}
	stocks, err := s.repo.ListStocks(ctx, codes)
	if err != nil {
		return nil, err
	}

	// ── 合规逻辑 3：行情缺失一律 nil ──
	quotes := map[string]*float64{}
	if s.cfg.Enabled && s.quote != nil {
		q, at, qErr := s.quote.BatchChangePct(ctx, codes)
		if qErr == nil {
			quotes = q
			resp.QuoteAt = toMillis(at)
		}
		// 行情拉取失败不影响主流程：quotes 保持空 map → 全部 nil → 前端显示「—」
	}

	byNode := map[int64][]model.ThemeStockMapping{}
	for _, m := range valid {
		byNode[m.ChainNodeID] = append(byNode[m.ChainNodeID], m)
	}

	resp.ChainNodes = make([]dto.ChainNodeResp, 0, len(nodes))
	for _, n := range nodes {
		items := make([]dto.StockItemResp, 0, len(byNode[n.ID]))
		for _, m := range byNode[n.ID] {
			st, ok := stocks[m.StockCode]
			if !ok {
				// 个股基础信息缺失 → 不展示。展示一个只有代码没有名称的条目毫无意义，
				// 且会让人怀疑数据质量。
				continue
			}
			items = append(items, dto.StockItemResp{
				Code:        st.Code,
				Name:        st.Name,
				Market:      st.Market,
				ChangePct:   quotes[m.StockCode], // 不存在时天然为 nil
				HasEvidence: true,                // 能到这里的都已通过证据校验
			})
		}
		// 排序只允许客观字段。默认按代码升序——它稳定、可解释、
		// 且不像"按涨幅"那样天然带出一个"第一名"。
		sort.Slice(items, func(i, j int) bool { return items[i].Code < items[j].Code })

		resp.ChainNodes = append(resp.ChainNodes, dto.ChainNodeResp{
			ID:          n.ID,
			Name:        n.Name,
			Description: n.Description,
			ChangePct:   aggregateChangePct(items),
			Caliber:     "已收录成分股等权平均",
			Stocks:      items,
		})
	}

	events, err := s.repo.ListEvents(ctx, themeID)
	if err != nil {
		return nil, err
	}
	resp.Events = make([]dto.ThemeEventResp, 0, len(events))
	for _, e := range events {
		resp.Events = append(resp.Events, dto.ThemeEventResp{
			ID:        e.ID,
			Title:     e.Title,
			PublishAt: toMillis(e.PublishAt),
			Source:    e.Source,
			SourceURL: e.SourceURL,
		})
	}

	return resp, nil
}

// GetEvidence 返回归属依据。这是本产品差异化的核心接口。
//
// 权限与详情页一致——否则用户可以绕过付费墙直接调本接口拿到最值钱的内容。
func (s *ThemeService) GetEvidence(ctx context.Context, themeID int64, stockCode string, access Access) (*dto.EvidenceResp, error) {
	if !access.CanViewPaid {
		return nil, ErrAccessDenied // handler 转 403 + 订阅引导
	}

	m, err := s.repo.FindApprovedMapping(ctx, themeID, stockCode)
	if err != nil {
		return nil, err
	}
	// 证据不全视为不存在。返回空依据比返回 404 更糟。
	if m == nil || !m.HasCompleteEvidence() {
		return nil, ErrEvidenceNotFound
	}

	stocks, err := s.repo.ListStocks(ctx, []string{stockCode})
	if err != nil {
		return nil, err
	}
	st, ok := stocks[stockCode]
	if !ok {
		return nil, ErrEvidenceNotFound
	}

	nodeName, err := s.repo.GetChainNodeName(ctx, m.ChainNodeID)
	if err != nil {
		return nil, err
	}

	return &dto.EvidenceResp{
		StockCode:     st.Code,
		StockName:     st.Name,
		ChainNodeName: nodeName,
		SourceType:    m.SourceType,
		SourceExcerpt: m.SourceExcerpt,
		SourceURL:     m.SourceURL,
		CollectedAt:   toMillis(m.CollectedAt),
	}, nil
}

// aggregateChangePct 环节涨跌幅 = 已收录成分股等权平均。
//
// 只有【有行情】的个股参与平均；若一只都没有则返回 nil（前端显示「—」）。
// 把无数据当 0 参与平均会系统性地把结果拉向 0，那是编造数据。
func aggregateChangePct(items []dto.StockItemResp) *float64 {
	sum, n := 0.0, 0
	for _, it := range items {
		if it.ChangePct != nil {
			sum += *it.ChangePct
			n++
		}
	}
	if n == 0 {
		return nil
	}
	avg := sum / float64(n)
	return &avg
}

// toMillis 统一毫秒时间戳口径。零值时间返回 0，避免出现 1970 年这种可疑数字。
func toMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
