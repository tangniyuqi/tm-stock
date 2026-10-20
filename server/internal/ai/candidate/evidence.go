package candidate

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/guard"
	"github.com/tangniyuqi/tm-stock/server/internal/sources"
)

// ValidateAgainstSnapshot 在抽取入队与人工通过时复用同一份逐字、共现、红线规则。
// 当前授权与主数据身份必须由调用方在事务内另行复核。
func ValidateAgainstSnapshot(item Candidate, snapshot sources.Snapshot) error {
	if err := item.Validate(); err != nil {
		return err
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	types := map[string]int8{
		sources.CategoryAnnouncement: 1, sources.CategoryAnnualReport: 2, sources.CategoryProspectus: 3,
		sources.CategoryOfficialList: 4, sources.CategoryInteractive: 5,
	}
	if item.SnapshotID != snapshot.ID || item.SourceType != types[snapshot.SourceCategory] ||
		item.SourceURL != snapshot.URL || !item.CollectedAt.Equal(snapshot.FetchedAt) {
		return fmt.Errorf("候选来源类型、链接或采集时点与快照不一致")
	}
	var segments []string
	for start := 0; start < len(item.SentenceIDs); {
		end := start + 1
		for end < len(item.SentenceIDs) && item.SentenceIDs[end] == item.SentenceIDs[end-1]+1 {
			end++
		}
		firstID, lastID := item.SentenceIDs[start], item.SentenceIDs[end-1]
		if firstID < 1 || lastID > len(snapshot.Sentences) {
			return fmt.Errorf("候选句子编号越界")
		}
		first, last := snapshot.Sentences[firstID-1], snapshot.Sentences[lastID-1]
		segments = append(segments, snapshot.Body[first.StartByte:last.EndByte])
		start = end
	}
	if len(segments) != len(item.QuoteSegments) {
		return fmt.Errorf("候选引文分段数量与快照不一致")
	}
	for i, segment := range item.QuoteSegments {
		if !strings.Contains(segments[i], segment) || (!item.ExcerptTrimmed && segments[i] != segment) {
			return fmt.Errorf("候选引文不是快照对应句子的逐字内容")
		}
	}
	claim := guard.Claim{Predicate: item.Predicate, Entity: item.EntityKey, Theme: item.ThemeKey, SentenceIDs: item.SentenceIDs}
	raw, err := json.Marshal(struct {
		Claims []guard.Claim `json:"claims"`
	}{Claims: []guard.Claim{claim}})
	if err != nil {
		return fmt.Errorf("序列化候选校验命题失败: %w", err)
	}
	vocab := guard.Vocabulary{Predicates: map[string]bool{item.Predicate: true}, Themes: map[string][]string{item.ThemeKey: {item.ThemeKey}}}
	report, err := guard.ValidateExtraction(raw, guard.NewSnapshot(snapshot.Body), vocab)
	if err != nil {
		return fmt.Errorf("候选未通过抽取守卫复核: %w", err)
	}
	if len(report.Accepted) != 1 || len(report.Rejected) != 0 {
		return fmt.Errorf("候选未通过事实、共现或红线复核")
	}
	accepted := report.Accepted[0]
	if accepted.Quote != item.SourceExcerpt || !slices.Equal(accepted.Segments, item.QuoteSegments) ||
		(accepted.Trimmed || len(item.QuoteSegments) > 1) != item.ExcerptTrimmed {
		return fmt.Errorf("候选引文与守卫复核结果不一致")
	}
	return nil
}
