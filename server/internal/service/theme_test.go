package service

import (
	"context"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/model"
)

// ── 测试替身 ──────────────────────────────────────────────────────────────

type fakeRepo struct {
	themes   map[int64]model.Theme
	nodes    []model.Theme
	mappings []model.ThemeStockMapping
	stocks   map[int64]model.Stock
}

func (f *fakeRepo) GetTheme(_ context.Context, id int64) (*model.Theme, error) {
	if t, ok := f.themes[id]; ok {
		return &t, nil
	}
	return nil, nil
}
func (f *fakeRepo) ListChainNodes(_ context.Context, _ int64) ([]model.Theme, error) {
	return f.nodes, nil
}

// 故意原样返回注入的数据（含未审核、无依据的），
// 以便验证 service 层【也】做了校验，而不是完全依赖 SQL 过滤。
func (f *fakeRepo) ListVisibleMappings(_ context.Context, _ []int64) ([]model.ThemeStockMapping, error) {
	return f.mappings, nil
}
func (f *fakeRepo) FindVisibleMapping(_ context.Context, _ int64, tsCode string) (*model.ThemeStockMapping, error) {
	for i := range f.mappings {
		if f.mappings[i].TsCode == tsCode {
			return &f.mappings[i], nil
		}
	}
	return nil, nil
}
func (f *fakeRepo) ListStocksByID(_ context.Context, ids []int64) (map[int64]model.Stock, error) {
	out := map[int64]model.Stock{}
	for _, id := range ids {
		if s, ok := f.stocks[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

type fakeQuote struct {
	data map[string]*float64
	at   time.Time
}

func (f *fakeQuote) BatchChangePct(_ context.Context, _ []string) (map[string]*float64, time.Time, error) {
	return f.data, f.at, nil
}

func pct(v float64) *float64 { return &v }

// goodMapping 造一条证据齐全、已审核通过、启用中的映射。
func goodMapping(nodeID, stockID int64, tsCode string) model.ThemeStockMapping {
	return model.ThemeStockMapping{
		ThemeID:       nodeID,
		StockID:       stockID,
		TsCode:        tsCode,
		SourceType:    model.SourceTypeAnnualReport,
		SourceExcerpt: "公司光学镜头及光源模组业务收入占比 34.2%",
		SourceURL:     "https://example.com/report",
		CollectedAt:   time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
		AuditStatus:   model.AuditStatusApproved,
		Status:        model.StatusEnabled,
	}
}

func baseRepo() *fakeRepo {
	return &fakeRepo{
		themes: map[int64]model.Theme{
			100001: {ID: 100001, Name: "光刻机", Level: 1, ParentID: 0,
				Description: "光刻机相关的产业链构成",
				UpdatedAt:   time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)},
			100010: {ID: 100010, Name: "光源", Level: 2, ParentID: 100001},
		},
		nodes: []model.Theme{{ID: 100010, Name: "光源", Level: 2, ParentID: 100001}},
		stocks: map[int64]model.Stock{
			1: {ID: 1, TsCode: "688502.SH", Symbol: "688502", Name: "茂莱光学", Market: "科创板"},
			2: {ID: 2, TsCode: "000001.SZ", Symbol: "000001", Name: "样例股", Market: "主板"},
		},
	}
}

func paidAccess() Access { return Access{CanViewPaid: true, TrialLeft: 3} }

// ── 合规逻辑 1：证据不全 / 未审核 / 已停用的映射不得返回 ───────────────────

func TestGetDetail_DropsMappingWithoutEvidence(t *testing.T) {
	cases := []struct {
		name  string
		mutfn func(*model.ThemeStockMapping)
	}{
		{"来源类型为0", func(m *model.ThemeStockMapping) { m.SourceType = 0 }},
		{"缺原文摘录", func(m *model.ThemeStockMapping) { m.SourceExcerpt = "" }},
		{"缺原文链接", func(m *model.ThemeStockMapping) { m.SourceURL = "" }},
		{"缺采集时点", func(m *model.ThemeStockMapping) { m.CollectedAt = time.Time{} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := baseRepo()
			m := goodMapping(100010, 1, "688502.SH")
			c.mutfn(&m)
			repo.mappings = []model.ThemeStockMapping{m}

			svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
			resp, err := svc.GetDetail(context.Background(), 100001, paidAccess())
			if err != nil {
				t.Fatalf("不该报错: %v", err)
			}
			for _, n := range resp.ChainNodes {
				if len(n.Stocks) != 0 {
					t.Errorf("证据不全的映射必须被过滤，环节 %q 仍返回 %d 条", n.Name, len(n.Stocks))
				}
			}
		})
	}
}

func TestGetDetail_DropsNotVisibleMapping(t *testing.T) {
	cases := []struct {
		name  string
		mutfn func(*model.ThemeStockMapping)
	}{
		{"草稿", func(m *model.ThemeStockMapping) { m.AuditStatus = model.AuditStatusDraft }},
		{"待审", func(m *model.ThemeStockMapping) { m.AuditStatus = model.AuditStatusPending }},
		{"已驳回", func(m *model.ThemeStockMapping) { m.AuditStatus = model.AuditStatusRejected }},
		{"已停用", func(m *model.ThemeStockMapping) { m.Status = model.StatusDisabled }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := baseRepo()
			m := goodMapping(100010, 1, "688502.SH")
			c.mutfn(&m)
			repo.mappings = []model.ThemeStockMapping{m}

			svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
			resp, _ := svc.GetDetail(context.Background(), 100001, paidAccess())
			for _, n := range resp.ChainNodes {
				if len(n.Stocks) != 0 {
					t.Errorf("%s 的映射不得对外可见，仍返回 %d 条", c.name, len(n.Stocks))
				}
			}
		})
	}
}

func TestGetDetail_KeepsGoodMapping(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{goodMapping(100010, 1, "688502.SH")}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	resp, _ := svc.GetDetail(context.Background(), 100001, paidAccess())

	if len(resp.ChainNodes) != 1 {
		t.Fatalf("应有 1 个环节，实际 %d", len(resp.ChainNodes))
	}
	stocks := resp.ChainNodes[0].Stocks
	if len(stocks) != 1 {
		t.Fatalf("合格映射应返回，实际 %d 条", len(stocks))
	}
	if stocks[0].Name != "茂莱光学" || stocks[0].Code != "688502" || stocks[0].TsCode != "688502.SH" {
		t.Errorf("个股字段错误: %+v", stocks[0])
	}
	if !stocks[0].HasEvidence {
		t.Error("HasEvidence 应恒为 true")
	}
}

// 题材自身直接挂股票（不分环节的题材）也要能展示。
func TestGetDetail_ThemeLevelMappingShown(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{
		goodMapping(100001, 2, "000001.SZ"), // 挂在题材本身
		goodMapping(100010, 1, "688502.SH"), // 挂在环节
	}
	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	resp, _ := svc.GetDetail(context.Background(), 100001, paidAccess())

	total := 0
	for _, n := range resp.ChainNodes {
		total += len(n.Stocks)
	}
	if total != 2 {
		t.Errorf("题材自身与环节下的映射都应展示，共应 2 条，实际 %d（分组数 %d）",
			total, len(resp.ChainNodes))
	}
}

// ── 合规逻辑 2：无权限时付费层被锁且不返回空壳 ───────────────────────────

func TestGetDetail_LockedWhenNoAccess(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{goodMapping(100010, 1, "688502.SH")}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	resp, _ := svc.GetDetail(context.Background(), 100001, Access{CanViewPaid: false, TrialLeft: 2})

	if !resp.Locked {
		t.Error("无权限时 Locked 应为 true")
	}
	if resp.ChainNodes != nil {
		t.Errorf("无权限时不得返回环节数据，实际 %d 条", len(resp.ChainNodes))
	}
	if resp.Name == "" {
		t.Error("顶层（题材名）应免费可见")
	}
	if resp.TrialLeft != 2 {
		t.Errorf("试吃剩余数应透传，实际 %d", resp.TrialLeft)
	}
}

// ── 合规逻辑 3：无行情一律 nil，绝不填 0 ─────────────────────────────────

func TestGetDetail_QuoteDisabledYieldsNil(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{goodMapping(100010, 1, "688502.SH")}
	// 即使注入了行情，Enabled=false 也不得使用
	q := &fakeQuote{data: map[string]*float64{"688502.SH": pct(3.25)}, at: time.Now()}

	svc := NewThemeService(repo, q, QuoteConfig{Enabled: false, DelayMin: 15})
	resp, _ := svc.GetDetail(context.Background(), 100001, paidAccess())

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
	repo.mappings = []model.ThemeStockMapping{
		goodMapping(100010, 1, "688502.SH"),
		goodMapping(100010, 2, "000001.SZ"),
	}
	// 只给 688502.SH 行情，000001.SZ 缺失
	q := &fakeQuote{data: map[string]*float64{"688502.SH": pct(3.25)}, at: time.Now()}

	svc := NewThemeService(repo, q, QuoteConfig{Enabled: true, DelayMin: 15})
	resp, _ := svc.GetDetail(context.Background(), 100001, paidAccess())

	got := map[string]*float64{}
	for _, s := range resp.ChainNodes[0].Stocks {
		got[s.TsCode] = s.ChangePct
	}
	if got["688502.SH"] == nil || *got["688502.SH"] != 3.25 {
		t.Errorf("有行情的应返回实际值，得到 %v", got["688502.SH"])
	}
	if got["000001.SZ"] != nil {
		t.Errorf("无行情的必须为 nil（不是 0），得到 %v", *got["000001.SZ"])
	}
	// 环节均值只对有行情的求平均：只有 3.25 一只 → 3.25，而不是 (3.25+0)/2=1.625
	if avg := resp.ChainNodes[0].ChangePct; avg == nil || *avg != 3.25 {
		t.Errorf("环节均值应只计有行情的个股，期望 3.25，得到 %v", avg)
	}
}

func TestAggregateChangePct_AllMissingReturnsNil(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{goodMapping(100010, 1, "688502.SH")}
	q := &fakeQuote{data: map[string]*float64{}, at: time.Now()}

	svc := NewThemeService(repo, q, QuoteConfig{Enabled: true, DelayMin: 15})
	resp, _ := svc.GetDetail(context.Background(), 100001, paidAccess())

	if resp.ChainNodes[0].ChangePct != nil {
		t.Error("全部无行情时环节涨跌幅应为 nil，不能是 0")
	}
	if resp.ChainNodes[0].Caliber == "" {
		t.Error("涨跌幅必须附口径说明")
	}
}

// ── 依据接口：不得绕过付费墙，不得返回空依据 ────────────────────────────

func TestGetEvidence_DeniedWithoutAccess(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{goodMapping(100010, 1, "688502.SH")}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	_, err := svc.GetEvidence(context.Background(), 100001, "688502.SH", Access{CanViewPaid: false})
	if err != ErrAccessDenied {
		t.Fatalf("无权限时应返回 ErrAccessDenied（引导订阅），得到 %v", err)
	}
}

func TestGetEvidence_IncompleteTreatedAsNotFound(t *testing.T) {
	for _, c := range []struct {
		name  string
		mutfn func(*model.ThemeStockMapping)
	}{
		{"证据不全", func(m *model.ThemeStockMapping) { m.SourceExcerpt = "" }},
		{"未审核通过", func(m *model.ThemeStockMapping) { m.AuditStatus = model.AuditStatusPending }},
		{"已停用", func(m *model.ThemeStockMapping) { m.Status = model.StatusDisabled }},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo := baseRepo()
			m := goodMapping(100010, 1, "688502.SH")
			c.mutfn(&m)
			repo.mappings = []model.ThemeStockMapping{m}

			svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
			_, err := svc.GetEvidence(context.Background(), 100001, "688502.SH", paidAccess())
			if err != ErrEvidenceNotFound {
				t.Fatalf("应视为不存在（返回空依据比 404 更糟），得到 %v", err)
			}
		})
	}
}

func TestGetEvidence_ReturnsFourFields(t *testing.T) {
	repo := baseRepo()
	repo.mappings = []model.ThemeStockMapping{goodMapping(100010, 1, "688502.SH")}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	ev, err := svc.GetEvidence(context.Background(), 100001, "688502.SH", paidAccess())
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	if ev.SourceType == "" || ev.SourceExcerpt == "" || ev.SourceURL == "" || ev.CollectedAt == 0 {
		t.Errorf("依据四项必须齐全: %+v", ev)
	}
	if ev.SourceType != "年报" {
		t.Errorf("来源类型应翻译成中文展示文案，得到 %q", ev.SourceType)
	}
	if ev.ChainNodeName != "光源" {
		t.Errorf("应带出环节名，得到 %q", ev.ChainNodeName)
	}
	if ev.TsCode != "688502.SH" || ev.StockCode != "688502" {
		t.Errorf("代码字段错误: tsCode=%q stockCode=%q", ev.TsCode, ev.StockCode)
	}
}

// 未知枚举不能返回空串——空串在页面上是一片空白，用户会以为依据不完整。
func TestSourceTypeName_UnknownFallsBack(t *testing.T) {
	if got := sourceTypeName(99); got != "未知来源" {
		t.Errorf("未知来源类型应有兜底文案，得到 %q", got)
	}
	if got := sourceTypeName(model.SourceTypeAnnouncement); got != "公告" {
		t.Errorf("已知类型翻译错误，得到 %q", got)
	}
}

func TestGetDetail_ThemeNotFound(t *testing.T) {
	repo := baseRepo()
	repo.themes = map[int64]model.Theme{}

	svc := NewThemeService(repo, nil, QuoteConfig{Enabled: false})
	_, err := svc.GetDetail(context.Background(), 999, paidAccess())
	if err != ErrThemeNotFound {
		t.Fatalf("期望 ErrThemeNotFound，得到 %v", err)
	}
}
