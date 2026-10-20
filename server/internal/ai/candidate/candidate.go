// Package candidate 定义内部证据抽取待审记录；记录不直接对公众可见。
package candidate

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Candidate 是从单份授权来源快照中抽取并通过确定性校验的一条归属候选。
// Excerpt 必须由句子编号从不可变快照回填，不能来自模型自由文本。
type Candidate struct {
	ID             int64
	SnapshotID     int64
	ClaimIndex     int
	ThemeID        int64
	ThemeKey       string
	StockID        int64
	EntityKey      string
	TsCode         string
	Predicate      string
	SourceType     int8
	SourceExcerpt  string
	SentenceIDs    []int
	QuoteSegments  []string
	ExcerptTrimmed bool
	SourceURL      string
	CollectedAt    time.Time
	Provider       string
	Model          string
	PromptVersion  string
}

// Validate 检查候选能否映射到 addon_quant_theme_stock 的依据字段。
func (c Candidate) Validate() error {
	if c.ID < 0 || c.SnapshotID <= 0 || c.ClaimIndex < 0 || c.ClaimIndex >= 50 || c.ThemeID <= 0 || c.StockID <= 0 {
		return fmt.Errorf("候选编号无效")
	}
	if !boundedToken(c.ThemeKey, 192) || utf8.RuneCountInString(c.ThemeKey) < 2 || utf8.RuneCountInString(c.ThemeKey) > 64 ||
		!boundedToken(c.EntityKey, 96) || utf8.RuneCountInString(c.EntityKey) < 2 || utf8.RuneCountInString(c.EntityKey) > 32 {
		return fmt.Errorf("候选公司或题材名称无效")
	}
	if len(c.TsCode) == 0 || len(c.TsCode) > 20 || strings.TrimSpace(c.TsCode) != c.TsCode {
		return fmt.Errorf("候选股票代码无效")
	}
	switch c.Predicate {
	case "DISCLOSES_BUSINESS", "LISTED_IN_CATALOG":
	default:
		return fmt.Errorf("候选事实类型无效")
	}
	if c.SourceType < 1 || c.SourceType > 5 {
		return fmt.Errorf("候选依据类型无效")
	}
	if strings.TrimSpace(c.SourceExcerpt) == "" || utf8.RuneCountInString(c.SourceExcerpt) > 1000 {
		return fmt.Errorf("候选原文摘录无效")
	}
	if len(c.SentenceIDs) == 0 || len(c.SentenceIDs) > 8 {
		return fmt.Errorf("候选句子编号无效")
	}
	runs := 1
	for i, id := range c.SentenceIDs {
		if id <= 0 || (i > 0 && id <= c.SentenceIDs[i-1]) {
			return fmt.Errorf("候选句子编号必须严格递增")
		}
		if i > 0 && id != c.SentenceIDs[i-1]+1 {
			runs++
			if !c.ExcerptTrimmed {
				return fmt.Errorf("含省略段的候选必须标记为节选")
			}
		}
	}
	if len(c.QuoteSegments) != runs {
		return fmt.Errorf("候选引文分段与句子编号不一致")
	}
	for _, segment := range c.QuoteSegments {
		if strings.TrimSpace(segment) == "" {
			return fmt.Errorf("候选引文含空分段")
		}
	}
	if strings.Join(c.QuoteSegments, "……") != c.SourceExcerpt {
		return fmt.Errorf("候选引文与原文分段不一致")
	}
	parsedURL, urlErr := url.Parse(c.SourceURL)
	if strings.TrimSpace(c.SourceURL) == "" || len(c.SourceURL) > 512 || urlErr != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" || parsedURL.User != nil {
		return fmt.Errorf("候选原文链接无效")
	}
	if c.CollectedAt.IsZero() {
		return fmt.Errorf("候选采集时点无效")
	}
	if !boundedToken(c.Provider, 64) || !boundedToken(c.Model, 128) || !boundedToken(c.PromptVersion, 64) {
		return fmt.Errorf("候选模型追溯字段无效")
	}
	return nil
}

// SameSentenceIDs 用于幂等写入时比较重复候选的证据句子。
func (c Candidate) SameSentenceIDs(other Candidate) bool {
	return slices.Equal(c.SentenceIDs, other.SentenceIDs)
}

func boundedToken(value string, maxBytes int) bool {
	return strings.TrimSpace(value) != "" && len(value) <= maxBytes && strings.TrimSpace(value) == value
}
