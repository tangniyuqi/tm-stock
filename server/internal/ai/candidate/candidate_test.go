package candidate

import (
	"strings"
	"testing"
	"time"
)

func validCandidate() Candidate {
	return Candidate{
		SnapshotID: 1, ClaimIndex: 0, ThemeID: 2, ThemeKey: "算力", StockID: 3, EntityKey: "星河科技",
		TsCode: "688001.SH", Predicate: "DISCLOSES_BUSINESS", SourceType: 2,
		SourceExcerpt: "星河科技主营算力服务。", SentenceIDs: []int{1}, QuoteSegments: []string{"星河科技主营算力服务。"},
		SourceURL: "https://example.test/report", CollectedAt: time.Now().UTC(),
		Provider: "domestic-approved", Model: "model-v1", PromptVersion: "l0-extraction-v1",
	}
}

func TestCandidateValidate(t *testing.T) {
	if err := validCandidate().Validate(); err != nil {
		t.Fatalf("valid candidate rejected: %v", err)
	}
}

func TestCandidateRejectsBrokenCitationContract(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Candidate)
	}{
		{"quote not tied to segments", func(c *Candidate) { c.SourceExcerpt = "模型自写引文" }},
		{"non-contiguous sentences not marked excerpt", func(c *Candidate) {
			c.SentenceIDs = []int{1, 3}
			c.QuoteSegments = []string{"甲句", "乙句"}
			c.SourceExcerpt = "甲句……乙句"
		}},
		{"invalid evidence predicate", func(c *Candidate) { c.Predicate = "RECOMMEND" }},
		{"overlong quote", func(c *Candidate) {
			c.SourceExcerpt = strings.Repeat("字", 1001)
			c.QuoteSegments = []string{c.SourceExcerpt}
		}},
		{"non-https source", func(c *Candidate) { c.SourceURL = "http://example.test/report" }},
		{"invalid sentence order", func(c *Candidate) { c.SentenceIDs = []int{2, 1} }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			item := validCandidate()
			test.mutate(&item)
			if err := item.Validate(); err == nil {
				t.Fatal("invalid candidate unexpectedly passed validation")
			}
		})
	}
}
