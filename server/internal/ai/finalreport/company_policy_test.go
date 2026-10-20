package finalreport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func companyPolicyFixture(t *testing.T) (*Policy, task.AdmissionPlan, *CompanyDraft, []sources.Snapshot, string) {
	t.Helper()
	base, a, s, r, now := companyFixture(t)
	_, v, topic := fixture(t)
	topic.ID = 2
	topic.URL += "?topic=1"
	a.Materials = append(a.Materials, task.AdmissionMaterial{SnapshotID: topic.ID, Digest: topic.Hash, Domain: a.Materials[0].Domain, Category: topic.SourceCategory, URL: topic.URL, CollectedAt: topic.FetchedAt})
	s = append(s, topic)
	d, err := NewCompanyDraft(base, a, s, r, now)
	if err != nil {
		t.Fatal(err)
	}
	c := base.cfg
	c.CompanyDigest = d.ContentDigest()
	a.CompanyDigest = c.CompanyDigest
	p, err := NewCompanyPolicy(c, d, func(context.Context, EntailmentInput) (Verdict, error) { return Uncertain, nil }, func(context.Context, string) (Verdict, error) { return Uncertain, nil })
	if err != nil {
		t.Fatal(err)
	}
	sel, err := NewSelectionStage(p, v.TaskNo, v.ThemeID, v.ThemeName, s)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sel.Build(context.Background(), `{"selections":[{"section":"definition","snapshot_id":2,"sentence_ids":[1]},{"section":"timeline","snapshot_id":2,"sentence_ids":[2]},{"section":"segment","snapshot_id":2,"sentence_ids":[3]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	return p, a, d, s, raw
}

func TestCompanyPolicyFullFactsAndModelInputs(t *testing.T) {
	p, a, d, s, raw := companyPolicyFixture(t)
	v, err := p.Decode(raw)
	if err != nil || !p.MatchesAdmission(a, v) || len(v.SnapshotIDs()) != 1 || len(p.EvidenceIDs(v)) != 2 {
		t.Fatal("公司独有材料未绑定", err)
	}
	if p.Bind(v, s, v.ThemeName) != nil {
		t.Fatal("完整报告证据拒绝")
	}
	in, visible, err := p.Inputs(raw, s)
	if err != nil || !strings.Contains(in.EvidenceJSON, s[0].Body) {
		t.Fatal("公司证据未进蕴含", err)
	}
	for _, value := range []string{"示例公司", "600001", "示例环节", "节选", d.rows[0].Row.Evidence[0].Excerpt} {
		if !strings.Contains(visible, value) {
			t.Fatal("语义漏检公司可见字段", value)
		}
	}
	if strings.Contains(in.ProposalJSON+in.EvidenceJSON+visible, "test-only-license") {
		t.Fatal("内部许可流入判定")
	}
	// 同一事实集可被完整新准入绑定，不循环依赖整个准入/执行摘要。
	a.ExecutionDigest = strings.Repeat("c", 64)
	_, _, _, r, now := companyFixture(t)
	again, err := NewCompanyDraft(p, a, s, r, now)
	if err != nil || again.ContentDigest() != d.ContentDigest() || again.Digest() == d.Digest() {
		t.Fatal("完整准入与事实摘要循环或混用", err)
	}
	base, _ := NewPolicy(p.cfg, nil, nil)
	if base != nil {
		t.Fatal("缺校验器默认放行")
	}
	legacyCfg := p.cfg
	legacyCfg.CompanyDigest = ""
	legacy, err := NewPolicy(legacyCfg, p.entailment, p.semantic)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Decode(raw); err == nil || legacy.Digest() == p.Digest() {
		t.Fatal("新公司形态放宽旧政策")
	}
	if p.Bind(v, s[1:], v.ThemeName) == nil {
		t.Fatal("缺公司独有快照通过")
	}
	a.CompanyDigest = ""
	if p.MatchesAdmission(a, v) {
		t.Fatal("旧准入被补造公司")
	}
}

func TestCompanyPolicyRejectsMutations(t *testing.T) {
	for _, mode := range []string{"omit", "reorder", "duplicate", "name", "code", "node", "excerpt", "url", "time", "trimmed", "change", "ai", "provenance", "mixed"} {
		t.Run(mode, func(t *testing.T) {
			p, _, _, _, raw := companyPolicyFixture(t)
			var v Proposal
			if json.Unmarshal([]byte(raw), &v) != nil {
				t.Fatal("夹具错误")
			}
			s := &v.Report.Sections[3]
			r := s.Nodes[0].Row
			switch mode {
			case "omit":
				s.Nodes = s.Nodes[:1]
			case "reorder":
				s.Nodes[0], s.Nodes[1] = s.Nodes[1], s.Nodes[0]
			case "duplicate":
				s.Nodes = append(s.Nodes, s.Nodes[0])
			case "name":
				r.Name = "另一个公司"
			case "code":
				r.Code = "600099"
			case "node":
				r.Node = "另一个环节"
			case "excerpt":
				r.Evidence[0].Excerpt += "未披露文字"
			case "url":
				r.Evidence[0].URL += "?change=1"
			case "time":
				r.Evidence[0].FetchedAt++
			case "trimmed":
				r.Evidence[0].Trimmed = false
			case "change":
				r.Change = &render.Change{Pct: 1, Basis: "测试", DelayMin: 15, AsOf: r.Evidence[0].FetchedAt, Vendor: "示例"}
			case "ai":
				s.AILabel = true
			case "provenance":
				s.Nodes[0].Provenance = render.ProvAIExtractedQuote
			case "mixed":
				s.Nodes = append(s.Nodes, v.Report.Sections[0].Nodes[0])
			}
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Decode(string(b)); err == nil {
				t.Fatal("模型可改公司全集", mode)
			}
		})
	}
}

func TestCompanyPolicyFactoryAndSelectionClosure(t *testing.T) {
	p, _, d, s, _ := companyPolicyFixture(t)
	for _, mode := range []string{"nil", "empty", "wrong_digest", "bad_config", "nil_check"} {
		c := p.cfg
		draft := d
		e := p.entailment
		switch mode {
		case "nil":
			draft = nil
		case "empty":
			cp := *d
			cp.rows = nil
			draft = &cp
		case "wrong_digest":
			c.CompanyDigest = strings.Repeat("f", 64)
		case "bad_config":
			c.Ref = ""
		case "nil_check":
			e = nil
		}
		if _, err := NewCompanyPolicy(c, draft, e, p.semantic); err == nil {
			t.Fatal("工厂放行缺失配置", mode)
		}
	}
	sel, err := NewSelectionStage(p, strings.Repeat("a", 32), 1, "示例题材", s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sel.Build(context.Background(), `{"selections":[{"section":"definition","snapshot_id":2,"sentence_ids":[1]},{"section":"timeline","snapshot_id":2,"sentence_ids":[2]},{"section":"companies","snapshot_id":2,"sentence_ids":[3]}]}`); err == nil {
		t.Fatal("模型参与公司选择")
	}
	var missing *Policy
	if missing.CompanyDigest() != "" {
		t.Fatal("空政策有公司绑定")
	}
	if p.CompanyDigest() != d.ContentDigest() {
		t.Fatal("公司政策未绑定集合")
	}
	var absent *CompanyDraft
	if absent.ContentDigest() != "" {
		t.Fatal("空候选有事实摘要")
	}
}

func TestCompanyPolicyBindingsAndCapturedChecks(t *testing.T) {
	p, a, d, s, raw := companyPolicyFixture(t)
	v, err := p.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"material", "row", "snapshot", "content"} {
		copyPlan := a.Clone()
		copyProposal := v
		copySnaps := append([]sources.Snapshot(nil), s...)
		switch mode {
		case "material":
			copyPlan.Materials[0].URL += "?changed=1"
			if p.MatchesAdmission(copyPlan, v) {
				t.Fatal("原公司材料身份变化放行")
			}
			continue
		case "row":
			copyProposal.Report.Sections = append([]render.Section(nil), v.Report.Sections...)
			copyProposal.Report.Sections[3].Nodes = nil
		case "snapshot":
			copySnaps[0].URL += "?changed=1"
		case "content":
			_, _, _, records, now := companyFixture(t)
			records[0].ReviewerID++
			if _, err := NewCompanyDraft(p, a, s, records, now); err == nil {
				t.Fatal("后来审核不能复用原事实摘要")
			}
			continue
		}
		if p.Bind(copyProposal, copySnaps, v.ThemeName) == nil {
			t.Fatal("结构化事实绑定变异通过", mode)
		}
	}
	input, visible, err := p.Inputs(raw, s)
	if err != nil {
		t.Fatal(err)
	}
	config := checkConfig()
	ent, err := NewEntailmentCheck(config, input)
	if err != nil {
		t.Fatal(err)
	}
	sem, err := NewSemanticCheck(config, v.TaskNo, visible)
	if err != nil {
		t.Fatal(err)
	}
	e := captureEvidence(t, ent, checkReply(config, `{"verdict":"pass","reason":"supported"}`))
	m := captureEvidence(t, sem, checkReply(config, `{"verdict":"pass","reason":"factual"}`))
	c := p.cfg
	c.EntailmentVersion = ent.PolicyVersion()
	c.SemanticVersion = sem.PolicyVersion()
	bound, err := NewCapturedPolicy(c, e, m, d)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, reason, err := bound.Review(context.Background(), raw, v, s); err != nil || reason != "" {
		t.Fatal("已捕获公司判定不能纯回放", reason, err)
	}
	if _, err := NewCapturedPolicy(c, e, m, d, d); err == nil {
		t.Fatal("多份公司候选被静默选择")
	}
	_, taskValue, _, hint := selectionModelFixture(t)
	stage, err := NewSelectionStage(p, taskValue.No, taskValue.SubjectID, p.companies.name, s)
	if err != nil {
		t.Fatal(err)
	}
	modelPlan, err := NewSelectionModelPlan(config, stage, taskValue, hint)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(modelPlan.Request().Schema), `"companies"`) || !strings.Contains(modelPlan.Request().System, "三节") {
		t.Fatal("模型仍可选择公司")
	}
}
