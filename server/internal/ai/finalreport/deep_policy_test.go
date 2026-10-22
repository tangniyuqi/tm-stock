package finalreport

import (
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
)

func TestDeepPolicyRequiresExplicitTypeAndSixQuotes(t *testing.T) {
	p, _, _, a := admissionScopeFixture(t)
	l2cfg := p.cfg
	l2cfg.AnalysisType, l2cfg.MinQuotes = task.DeepAnalysisType, task.DeepMinQuotes
	l2, err := NewPolicy(l2cfg, p.entailment, p.semantic)
	if err != nil {
		t.Fatal(err)
	}
	a.Scope.AnalysisType, a.Price.Points = task.DeepAnalysisType, task.DeepPricePoints
	if err := a.Validate(a.Scope); err != nil {
		t.Fatal("深度准入格式被拒绝", err)
	}
	// 四节各一条引文不足以满足深度最低交付。
	if p.MatchesAdmission(a, vFromScopeFixture(t)) {
		t.Fatal("L1策略错误接受深度范围")
	}
	v := vFromScopeFixture(t)
	if l2.MatchesAdmission(a, v) {
		t.Fatal("四条引文错误通过深度最低交付")
	}
	node := v.Report.Sections[0].Nodes[0]
	v.Report.Sections[0].Nodes = append(v.Report.Sections[0].Nodes, node, node)
	if !l2.MatchesAdmission(a, v) {
		t.Fatal("六条逐字引文未通过深度最低交付")
	}
	v.Report.Sections[0].Nodes = v.Report.Sections[0].Nodes[:1]
	if l2.MatchesAdmission(a, v) {
		t.Fatal("低于六条引文仍通过深度最低交付")
	}
}

func vFromScopeFixture(t *testing.T) Proposal {
	t.Helper()
	_, v, _, _ := admissionScopeFixture(t)
	return v
}

func TestDeepPolicyConfigBounds(t *testing.T) {
	p, _, _, _ := admissionScopeFixture(t)
	for _, c := range []Config{
		{Ref: p.cfg.Ref, ModelName: p.cfg.ModelName, FilingNo: p.cfg.FilingNo, EntailmentVersion: p.cfg.EntailmentVersion, SemanticVersion: p.cfg.SemanticVersion, AllowedDomains: p.cfg.AllowedDomains, AnalysisType: task.DeepAnalysisType, MinQuotes: task.DeepMinQuotes - 1},
		{Ref: p.cfg.Ref, ModelName: p.cfg.ModelName, FilingNo: p.cfg.FilingNo, EntailmentVersion: p.cfg.EntailmentVersion, SemanticVersion: p.cfg.SemanticVersion, AllowedDomains: p.cfg.AllowedDomains, MinQuotes: task.DeepMinQuotes},
		{Ref: p.cfg.Ref, ModelName: p.cfg.ModelName, FilingNo: p.cfg.FilingNo, EntailmentVersion: p.cfg.EntailmentVersion, SemanticVersion: p.cfg.SemanticVersion, AllowedDomains: p.cfg.AllowedDomains, AnalysisType: 1},
		{Ref: p.cfg.Ref, ModelName: p.cfg.ModelName, FilingNo: p.cfg.FilingNo, EntailmentVersion: p.cfg.EntailmentVersion, SemanticVersion: p.cfg.SemanticVersion, AllowedDomains: p.cfg.AllowedDomains, AnalysisType: 3},
	} {
		if _, err := NewPolicy(c, p.entailment, p.semantic); err == nil {
			t.Fatal("非法深度配置未拒绝", c.AnalysisType, c.MinQuotes)
		}
	}
	if _, err := NewPolicy(func() Config { c := l2Config(p.cfg); return c }(), p.entailment, p.semantic); err != nil {
		t.Fatal("合法深度配置被拒绝", err)
	}
}

func l2Config(c Config) Config {
	c.AnalysisType, c.MinQuotes = task.DeepAnalysisType, task.DeepMinQuotes
	return c
}
