package service

import (
	"context"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/model"
)

// ── 测试替身 ──────────────────────────────────────────────────────────────

type fakeRepo struct {
	theme     *model.Theme
	path      string
	nodes     []model.ChainNode
	mappings  []model.ThemeStockMapping
	stocks    map[string]model.Stock
	events    []model.ThemeEvent
	nodeNames map[int64]string
}

func (f *fakeRepo) GetTheme(_ context.Context, _ int64) (*model.Theme, error) {
	return f.theme, nil
}
func (f *fakeRepo) GetCategoryPath(_ context.Context, _ int64) (string, error) {
	return f.path, nil
}
func (f *fakeRepo) ListChainNodes(_ context.Context, _ int64) ([]model.ChainNode, error) {
	return f.nodes, nil
}

// ListApprovedMappings 故意原样返回注入的数据（含未通过审核的），
// 以便验证 service 层【也】做了状态与证据校验，而不是完全依赖 SQL。
func (f *fakeRepo) ListApprovedMappings(_ context.Context, _ int64) ([]model.ThemeStockMapping, error) {
	return f.mappings, nil
}
func (f *fakeRepo) ListStocks(_ context.Context, codes []string) (map[string]model.Stock, error) {
	out := map[string]model.Stock{}
	for _, c := range codes {
		if s, ok := f.stocks[c]; ok {
			out[c] = s
		}
	}
	return out, nil
}
func (f *fakeRepo) ListEvents(_ context.Context, _ int64) ([]model.ThemeEvent, error) {
	return f.events, nil
}
func (f *fakeRepo) FindApprovedMapping(_ context.Context, _ int64, code string) (*model.ThemeStockMapping, error) {
	for i := range f.mappings {
		if f.mappings[i].StockCode == code {
			return &f.mappings[i], nil
		}
	}
	return nil, nil
}
func (f *fakeRepo) GetChainNodeName(_ context.Context, id int64) (string, error) {
	return f.nodeNames[id], nil
}

type fakeQuote struct {
	data map[string]*float64
	at   time.Time
}

func (f *fakeQuote) BatchChangePct(_ context.Context, _ []string) (map[string]*float64, time.Time, error) {
	return f.data, f.at, nil
}

func pct(v float64) *float64 { return &v }

// completeMapping 造一条证据齐全、已审核通过的映射。
func completeMapping(code string, nodeID int64) model.ThemeStockMapping {
	return model.ThemeStockMapping{
		ThemeID:       1,
		ChainNodeID:   nodeID,
		StockCode:     code,
		SourceType:    "年报",
		SourceExcerpt: "公司光学镜头及光源模组业务收入占比 34.2%",
		SourceURL:     "https://example.com/report",
		CollectedAt:   time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
		Status:        model.MappingStatusApproved,
	}
}

func baseRepo() *fakeRepo {
	return &fakeRepo{
		theme: &model.Theme{
			ID: 1, Name: "OCS 光交换", CategoryID: 9,
			Summary:   "光交换相关的产业链构成",
			UpdatedAt: time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC),
		},
		path:      "半导体与电子 › 光学光电子",
		nodes:     []model.ChainNode{{ID: 100, ThemeID: 1, Name: "光源"}},
		stocks:    map[string]model.Stock{"688502": {Code: "688502", Name: "茂莱光学", Market: "SH"}},
		nodeNames: map[int64]string{100: "光源"},
	}
}

func paidAccess() Access { return Access{CanViewPaid: true, TrialLeft: 3} }

// ── 合规逻辑 1：证据不全的映射不得出现在返回中（AC14）────────────────────

func TestGetDetail_DropsMappingWithoutEvidence(t *testing.T) {
	cases := []struct {
		name  string
		mutfn func(*model.ThemeStockMapping)
	}{
		{"缺来源类型", func(m *model.ThemeStockMapping) { m.SourceType = "" }},
		{"缺原文摘录", func(m *model.ThemeStockMapping) { m.SourceExcerpt = "" }},
		{"缺原文链接", func(m *model.ThemeStockMapping) { m.SourceURL = "" }},
		{"缺采集时点", func(m *model.ThemeStockMapping) { m.CollectedAt = time.Time{} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := baseRepo()
			m := completeMapping("688502", 100)
			c.mutfn(&m)
			repo.mappings = []model.ThemeStockMapping{m}

			svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
			resp, err := svc.GetDetail(context.Background(), 1, paidAccess())
			if err != nil {
				t.Fatalf("不该报错: %v", err)
			}
			if len(resp.ChainNodes) != 1 {
				t.Fatalf("环节数应为 1，实际 %d", len(resp.ChainNodes))
			}
			if got := len(resp.ChainNodes[0].Stocks); got != 0 {
				t.Errorf("证据不全的映射必须被过滤，实际返回 %d 条", got)
			}
		})
	}
}

// 未审核通过的映射同样不得出现——即使 SQL 层漏了过滤条件。
func TestGetDetail_DropsUnapprovedMapping(t *testing.T) {
	for _, st := range []string{model.MappingStatusDraft, model.MappingStatusPending, model.MappingStatusRejected} {
		t.Run(st, func(t *testing.T) {
			repo := baseRepo()
			m := completeMapping("688502", 100)
			m.Status = st
			repo.mappings = []model.ThemeStockMapping{m}

			svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
			resp, _ := svc.GetDetail(context.Background(), 1, paidAccess())
			if got := len(resp.ChainNodes[0].Stocks); got != 0 {
				t.Errorf("状态 %s 的映射不得对外可见，实际返回 %d 条", st, got)
			}
		})
	}
}

func TestGetDetail_KeepsCompleteApprovedMapping(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{completeMapping("688502", 100)}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	resp, _ := svc.GetDetail(context.Background(), 1, paidAccess())

	stocks := resp.ChainNodes[0].Stocks
	if len(stocks) != 1 {
		t.Fatalf("证据齐全且已通过的映射应返回，实际 %d 条", len(stocks))
	}
	if stocks[0].Name != "茂莱光学" {
		t.Errorf("个股名称错误: %q", stocks[0].Name)
	}
	if !stocks[0].HasEvidence {
		t.Error("HasEvidence 应恒为 true")
	}
}

// ── 合规逻辑 2：无权限时付费层被锁且不返回空壳（AC8）──────────────────────

func TestGetDetail_LockedWhenNoAccess(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{completeMapping("688502", 100)}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	resp, _ := svc.GetDetail(context.Background(), 1, Access{CanViewPaid: false, TrialLeft: 2})

	if !resp.Locked {
		t.Error("无权限时 Locked 应为 true")
	}
	if resp.ChainNodes != nil {
		t.Errorf("无权限时不得返回环节数据，实际 %d 条", len(resp.ChainNodes))
	}
	if resp.Name == "" || resp.Summary == "" {
		t.Error("顶层（名称/解读）应免费可见")
	}
	if resp.TrialLeft != 2 {
		t.Errorf("试吃剩余数应透传，实际 %d", resp.TrialLeft)
	}
}

// ── 合规逻辑 3：无行情一律 nil，绝不填 0（数据真实铁律）────────────────────

func TestGetDetail_QuoteDisabledYieldsNil(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{completeMapping("688502", 100)}
	// 即使注入了行情，Enabled=false 也不得使用
	q := &fakeQuote{data: map[string]*float64{"688502": pct(3.25)}, at: time.Now()}

	svc := NewThemeService(repo, q, QuoteConfig{Enabled: false, DelayMin: 15})
	resp, _ := svc.GetDetail(context.Background(), 1, paidAccess())

	if got := resp.ChainNodes[0].Stocks[0].ChangePct; got != nil {
		t.Errorf("行情关闭时涨跌幅必须为 nil，实际 %v", *got)
	}
	if resp.QuoteEnabled {
		t.Error("QuoteEnabled 应为 false")
	}
	if resp.ChainNodes[0].ChangePct != nil {
		t.Error("行情关闭时环节涨跌幅也必须为 nil")
	}
}

func TestGetDetail_MissingQuoteStaysNilNotZero(t *testing.T) {
	repo := baseRepo()
	repo.stocks["600000"] = model.Stock{Code: "600000", Name: "样例股", Market: "SH"}
	repo.mappings = []model.ThemeStockMapping{
		completeMapping("688502", 100),
		completeMapping("600000", 100),
	}
	// 只给 688502 行情，600000 缺失
	q := &fakeQuote{data: map[string]*float64{"688502": pct(3.25)}, at: time.Now()}

	svc := NewThemeService(repo, q, QuoteConfig{Enabled: true, DelayMin: 15})
	resp, _ := svc.GetDetail(context.Background(), 1, paidAccess())

	got := map[string]*float64{}
	for _, s := range resp.ChainNodes[0].Stocks {
		got[s.Code] = s.ChangePct
	}
	if got["688502"] == nil || *got["688502"] != 3.25 {
		t.Errorf("有行情的应返回实际值，得到 %v", got["688502"])
	}
	if got["600000"] != nil {
		t.Errorf("无行情的必须为 nil（不是 0），得到 %v", *got["600000"])
	}
	// 环节均值只对有行情的求平均：只有 3.25 一只 → 3.25，而不是 (3.25+0)/2=1.625
	if avg := resp.ChainNodes[0].ChangePct; avg == nil || *avg != 3.25 {
		t.Errorf("环节均值应只计有行情的个股，期望 3.25，得到 %v", avg)
	}
}

func TestAggregateChangePct_AllMissingReturnsNil(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{completeMapping("688502", 100)}
	q := &fakeQuote{data: map[string]*float64{}, at: time.Now()}

	svc := NewThemeService(repo, q, QuoteConfig{Enabled: true, DelayMin: 15})
	resp, _ := svc.GetDetail(context.Background(), 1, paidAccess())

	if resp.ChainNodes[0].ChangePct != nil {
		t.Error("全部无行情时环节涨跌幅应为 nil，不能是 0")
	}
	if resp.ChainNodes[0].Caliber == "" {
		t.Error("涨跌幅必须附口径说明")
	}
}

// ── 依据接口：不得绕过付费墙，且不得返回空依据 ──────────────────────────

func TestGetEvidence_DeniedWithoutAccess(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{completeMapping("688502", 100)}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	_, err := svc.GetEvidence(context.Background(), 1, "688502", Access{CanViewPaid: false})
	if err != ErrAccessDenied {
		t.Fatalf("无权限时应返回 ErrAccessDenied（引导订阅），得到 %v", err)
	}
}

func TestGetEvidence_IncompleteTreatedAsNotFound(t *testing.T) {
	repo := baseRepo()
	m := completeMapping("688502", 100)
	m.SourceExcerpt = "" // 证据不全
	repo.mappings = []model.ThemeStockMapping{m}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	_, err := svc.GetEvidence(context.Background(), 1, "688502", paidAccess())
	if err != ErrEvidenceNotFound {
		t.Fatalf("证据不全应视为不存在（返回空依据比 404 更糟），得到 %v", err)
	}
}

func TestGetEvidence_ReturnsFourFields(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{completeMapping("688502", 100)}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	ev, err := svc.GetEvidence(context.Background(), 1, "688502", paidAccess())
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	if ev.SourceType == "" || ev.SourceExcerpt == "" || ev.SourceURL == "" || ev.CollectedAt == 0 {
		t.Errorf("依据四项必须齐全: %+v", ev)
	}
	if ev.ChainNodeName != "光源" {
		t.Errorf("应带出环节名，得到 %q", ev.ChainNodeName)
	}
}

func TestGetDetail_ThemeNotFound(t *testing.T) {
	repo := baseRepo()
	repo.theme = nil

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	_, err := svc.GetDetail(context.Background(), 999, paidAccess())
	if err != ErrThemeNotFound {
		t.Fatalf("期望 ErrThemeNotFound，得到 %v", err)
	}
}
