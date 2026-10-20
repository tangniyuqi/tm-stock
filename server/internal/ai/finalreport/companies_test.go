package finalreport

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/render"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func companyFixture(t *testing.T) (*Policy, task.AdmissionPlan, []sources.Snapshot, []CompanyRecord, time.Time) {
	t.Helper()
	p, _, _, a := admissionScopeFixture(t)
	s, err := sources.BuildSnapshot(sources.Source{ID: 1, Domain: a.Materials[0].Domain, Category: sources.CategoryAnnualReport, Enabled: true, LicenseTag: "test-only", LicenseRef: "test-only-license"}, a.Materials[0].URL, sources.MediaPlain, []byte("示例公司在示例题材的资料中披露业务事项。另一示例公司在示例环节披露业务事项。"), a.Materials[0].CollectedAt, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	s.ID = 1
	a.Materials[0].Digest = s.Hash
	at := s.FetchedAt.Add(time.Second)
	r := CompanyRecord{ID: 1, NodeID: a.Scope.SubjectID, StockID: 1, ReviewerID: 11, NodeName: a.SubjectName, Name: "示例公司", Symbol: "600002", TsCode: "600002.SH", MappingCode: "600002.SH", SourceType: 2, AuditStatus: 2, Status: 1, Excerpt: s.Sentences[0].Text, URL: s.URL, CollectedAt: s.FetchedAt, ReviewedAt: at, Trimmed: true}
	other := r
	other.ID, other.NodeID, other.ParentID, other.StockID = 2, 2, a.Scope.SubjectID, 2
	other.NodeName, other.Name, other.Symbol, other.TsCode, other.MappingCode, other.Excerpt = "示例环节", "另一示例公司", "600001", "600001.SH", "600001.SH", s.Sentences[1].Text
	return p, a, []sources.Snapshot{s}, []CompanyRecord{r, other}, at
}

func TestCompanyDraftObjectiveOrderAndIsolation(t *testing.T) {
	p, a, s, records, now := companyFixture(t)
	d, err := NewCompanyDraft(p, a, s, records, now)
	if err != nil {
		t.Fatal(err)
	}
	rows := d.Rows()
	if !ValidDigest(d.Digest()) || len(rows) != 2 || rows[0].Row.Code != "600001" || rows[1].Row.Code != "600002" {
		t.Fatal("未按客观代码排序", err)
	}
	for _, n := range rows {
		if n.Kind != render.KindDataRow || n.Provenance != render.ProvLibrary || n.Row.Change != nil || !n.Row.Evidence[0].Trimmed {
			t.Fatal("来源、节选或空行情错误")
		}
	}
	// 完整合法的数据行也不能借准备能力绕过当前四节引文交付政策。
	_, proposal, _ := fixture(t)
	proposal.Report.Sections[3].Nodes = d.Rows()
	proposal.Report.Sections[3].AILabel = false
	raw, marshalErr := json.Marshal(proposal)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if _, err := p.Decode(string(raw)); err == nil {
		t.Fatal("公司候选被旧引文政策直接接纳")
	}
	records[0].Name = "污染"
	rows[0].Row.Name = "污染"
	rows[0].Row.Evidence[0].Excerpt = "污染"
	if d.Rows()[0].Row.Name == "污染" || d.Rows()[0].Row.Evidence[0].Excerpt == "污染" {
		t.Fatal("候选可被调用方修改")
	}
	b, err := json.Marshal(d)
	if err != nil || string(b) != "{}" || strings.Contains(fmt.Sprintf("%+v %#v %+v %#v", d, d, records[1], records[1]), "另一示例公司") {
		t.Fatal("候选或原文意外输出", err)
	}
	_, a, s, records, now = companyFixture(t)
	records[0], records[1] = records[1], records[0]
	again, err := NewCompanyDraft(p, a, s, records, now)
	if err != nil || again.Digest() != d.Digest() {
		t.Fatal("摘要依赖资料库无序输入", err)
	}
	var empty *CompanyDraft
	if empty.Digest() != "" || empty.Rows() != nil {
		t.Fatal("空候选放行")
	}
	zero, err := NewCompanyDraft(p, a, s, nil, now)
	if err != nil || len(zero.Rows()) != 0 || zero.Digest() == d.Digest() {
		t.Fatal("无公司伪造数据", err)
	}
}

func TestCompanyDraftMultipleNodesAndMetadataBinding(t *testing.T) {
	p, a, s, r, now := companyFixture(t)
	child := r[0]
	child.ID, child.NodeID, child.ParentID, child.NodeName = 3, r[1].NodeID, r[1].ParentID, r[1].NodeName
	r = append(r, child)
	d, err := NewCompanyDraft(p, a, s, r, now)
	if err != nil || len(d.Rows()) != 3 || d.Rows()[1].Row.Node != a.SubjectName || d.Rows()[2].Row.Node != child.NodeName {
		t.Fatal("同一公司不同审核环节丢失或无客观排序", err)
	}
	for _, change := range []func(*CompanyRecord){func(r *CompanyRecord) { r.Trimmed = false }, func(r *CompanyRecord) { r.ReviewerID++ }, func(r *CompanyRecord) { r.ReviewedAt = r.ReviewedAt.Add(-time.Millisecond) }} {
		cp := append([]CompanyRecord(nil), r...)
		change(&cp[0])
		other, err := NewCompanyDraft(p, a, s, cp, now)
		if err != nil || other.Digest() == d.Digest() {
			t.Fatal("审核/节选变化未绑定", err)
		}
	}
	// 证据元数据的可选节选标记不能改变历史未节选行的序列化。
	b, err := json.Marshal(render.EvidenceItem{Type: "年报", Excerpt: "示例", URL: s[0].URL, FetchedAt: now.UnixMilli()})
	if err != nil || strings.Contains(string(b), "trimmed") {
		t.Fatal("未节选旧行编码发生变化", err)
	}
}

func TestCompanyDraftRejectsWholeBatch(t *testing.T) {
	for _, mode := range []string{"policy", "scope", "model", "filing", "no_snapshot", "extra_snapshot", "duplicate_snapshot", "body", "hash", "url", "category", "collected", "future_snapshot", "expired_license", "domain", "missing_id", "node", "parent", "root_name", "stock_id", "reviewer", "audit", "status", "mapping_code", "symbol", "exchange", "excerpt", "excerpt_nonnormal", "excerpt_long", "excerpt_not_original", "company_absent", "redline", "source_type", "source_url", "source_time", "review_time", "review_before", "review_future", "review_precision", "duplicate_id", "duplicate_pair", "ambiguous_code", "inconsistent_stock", "inconsistent_node", "limit", "zero_clock"} {
		t.Run(mode, func(t *testing.T) {
			p, a, s, r, now := companyFixture(t)
			switch mode {
			case "policy":
				p = nil
			case "scope":
				a.Scope.SubjectType = task.Topic
			case "model":
				a.ModelName = "其他模型"
			case "filing":
				a.FilingNo = "other-test-only"
			case "no_snapshot":
				s = nil
			case "extra_snapshot":
				s = append(s, s[0])
			case "duplicate_snapshot":
				a.Materials = append(a.Materials, a.Materials[0])
				a.Materials[1].SnapshotID = 2
				s = append(s, s[0])
			case "body":
				s[0].Body += "不同正文"
			case "hash":
				a.Materials[0].Digest = strings.Repeat("f", 64)
			case "url":
				a.Materials[0].URL += "?changed=1"
			case "category":
				a.Materials[0].Category = sources.CategoryAnnouncement
			case "collected":
				a.Materials[0].CollectedAt = a.Materials[0].CollectedAt.Add(time.Millisecond)
			case "future_snapshot":
				now = s[0].FetchedAt.Add(-time.Second)
			case "expired_license":
				s[0].LicenseExpires = &now
			case "domain":
				a.Materials[0].Domain = "other.example.test"
			case "missing_id":
				r[0].ID = 0
			case "node":
				r[0].NodeID = 0
			case "parent":
				r[1].ParentID = 99
			case "root_name":
				r[0].NodeName = "另一题材"
			case "stock_id":
				r[0].StockID = 0
			case "reviewer":
				r[0].ReviewerID = 0
			case "audit":
				r[0].AuditStatus = 1
			case "status":
				r[0].Status = 2
			case "mapping_code":
				r[0].MappingCode = "600099.SH"
			case "symbol":
				r[0].Symbol = "600099"
			case "exchange":
				r[0].TsCode = "600002.US"
				r[0].MappingCode = r[0].TsCode
			case "excerpt":
				r[0].Excerpt = ""
			case "excerpt_nonnormal":
				r[0].Excerpt += "\u200b"
			case "excerpt_long":
				r[0].Excerpt += strings.Repeat("示", 1001)
			case "excerpt_not_original":
				r[0].Excerpt += "未曾披露"
			case "company_absent":
				r[0].Name = "未披露公司"
			case "redline":
				r[0].NodeName = "买入建议"
			case "source_type":
				r[0].SourceType = 6
			case "source_url":
				r[0].URL += "?changed=1"
			case "source_time":
				r[0].CollectedAt = r[0].CollectedAt.Add(-time.Millisecond)
			case "review_time":
				r[0].ReviewedAt = time.Time{}
			case "review_before":
				r[0].ReviewedAt = r[0].CollectedAt.Add(-time.Millisecond)
			case "review_future":
				r[0].ReviewedAt = now.Add(time.Millisecond)
			case "review_precision":
				r[0].ReviewedAt = now.Add(-time.Nanosecond)
			case "duplicate_id":
				r[1].ID = r[0].ID
			case "duplicate_pair":
				r = append(r, r[0])
				r[2].ID = 3
			case "ambiguous_code":
				r[1].TsCode, r[1].Symbol, r[1].MappingCode = r[0].TsCode, r[0].Symbol, r[0].MappingCode
			case "inconsistent_stock":
				r[1].StockID = r[0].StockID
			case "inconsistent_node":
				r[1].NodeID = r[0].NodeID
				r[1].NodeName = r[0].NodeName
				r[1].ParentID = 9
			case "limit":
				r = make([]CompanyRecord, MaxCompanyMappings+1)
			case "zero_clock":
				now = time.Time{}
			}
			d, err := NewCompanyDraft(p, a, s, r, now)
			if err == nil || d != nil {
				t.Fatal("坏公司记录未整批关闭", mode)
			}
		})
	}
}
