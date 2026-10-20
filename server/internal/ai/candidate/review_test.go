package candidate

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

func TestReviewRequestContract(t *testing.T) {
	digest, err := EvidenceDigest(validCandidate())
	if err != nil {
		t.Fatal(err)
	}
	valid := ReviewRequest{CandidateID: 1, EvidenceDigest: digest, Decision: "approve", RequestID: "test_review_00001"}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*ReviewRequest)
	}{
		{"空候选", func(r *ReviewRequest) { r.CandidateID = 0 }},
		{"错误摘要", func(r *ReviewRequest) { r.EvidenceDigest = strings.Repeat("z", 64) }},
		{"大写摘要", func(r *ReviewRequest) { r.EvidenceDigest = strings.ToUpper(digest) }},
		{"短请求编号", func(r *ReviewRequest) { r.RequestID = "short" }},
		{"请求编号含空格", func(r *ReviewRequest) { r.RequestID = "test_review 00001" }},
		{"未知决定", func(r *ReviewRequest) { r.Decision = "publish" }},
		{"通过携带原因", func(r *ReviewRequest) { r.Reason = "不能夹带理由" }},
		{"驳回无原因", func(r *ReviewRequest) { r.Decision = "reject" }},
		{"超长原因", func(r *ReviewRequest) { r.Decision = "reject"; r.Reason = strings.Repeat("字", 251) }},
		{"原因含控制符", func(r *ReviewRequest) { r.Decision = "reject"; r.Reason = "原文\n不支持" }},
		{"原因含无效编码", func(r *ReviewRequest) { r.Decision = "reject"; r.Reason = string([]byte{0xff}) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := valid
			tc.mutate(&r)
			if err := r.Validate(); !errors.Is(err, ErrInvalidReview) {
				t.Fatalf("参数未被拒绝: %v", err)
			}
		})
	}
	valid.Decision, valid.Reason = "reject", "原文未明确支持归属"
	if valid.Validate() != nil || valid.Status() != Rejected {
		t.Fatal("有效驳回请求未通过")
	}
}

func TestEvidenceDigestBindsEvidenceAndProvenance(t *testing.T) {
	item := validCandidate()
	first, err := EvidenceDigest(item)
	if err != nil {
		t.Fatal(err)
	}
	item.CollectedAt = item.CollectedAt.In(time.FixedZone("测试时区", 8*3600))
	second, err := EvidenceDigest(item)
	if err != nil || first != second {
		t.Fatalf("同一时点的时区表示改变了摘要: %v", err)
	}
	for _, mutate := range []func(*Candidate){
		func(c *Candidate) { c.ID++ }, func(c *Candidate) { c.ThemeID++ }, func(c *Candidate) { c.StockID++ },
		func(c *Candidate) { c.SourceURL += "?page=2" }, func(c *Candidate) { c.Provider += "-v2" },
		func(c *Candidate) { c.Model += "-v2" }, func(c *Candidate) { c.PromptVersion += "-v2" },
	} {
		changed := item
		mutate(&changed)
		next, err := EvidenceDigest(changed)
		if err != nil || first == next {
			t.Fatalf("依据或追溯字段未绑定: %v", err)
		}
	}
}

func TestCandidateSnapshotValidation(t *testing.T) {
	item := validCandidate()
	source := sources.Source{ID: 1, Domain: "example.test", Category: sources.CategoryAnnualReport,
		LicenseTag: "test-license", LicenseRef: "test-written-authorization", Enabled: true}
	snapshot, err := sources.BuildSnapshot(source, item.SourceURL, "text/plain; charset=utf-8", []byte(item.SourceExcerpt), item.CollectedAt, 1024)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.ID = item.SnapshotID
	item.CollectedAt = snapshot.FetchedAt
	if err := ValidateAgainstSnapshot(item, snapshot); err != nil {
		t.Fatalf("逐字正例失败: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Candidate)
	}{
		{"换快照", func(c *Candidate) { c.SnapshotID++ }},
		{"错来源类型", func(c *Candidate) { c.SourceType = 1 }},
		{"错链接", func(c *Candidate) { c.SourceURL += "?changed=1" }},
		{"错时点", func(c *Candidate) { c.CollectedAt = c.CollectedAt.Add(time.Second) }},
		{"编号越界", func(c *Candidate) { c.SentenceIDs = []int{2} }},
		{"伪造引文", func(c *Candidate) {
			c.SourceExcerpt = "星河科技披露算力业务。"
			c.QuoteSegments = []string{c.SourceExcerpt}
		}},
		{"伪造共现", func(c *Candidate) { c.EntityKey = "其他公司" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := item
			tc.mutate(&changed)
			if err := ValidateAgainstSnapshot(changed, snapshot); err == nil {
				t.Fatal("错误依据通过复核")
			}
		})
	}
}
