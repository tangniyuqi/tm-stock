package reviewcsv

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
)

func fixture() candidate.Candidate {
	return candidate.Candidate{ID: 1, SnapshotID: 2, ClaimIndex: 0, ThemeID: 3, ThemeKey: "算力", StockID: 4, EntityKey: "星河科技", TsCode: "688001.SH",
		Predicate: "DISCLOSES_BUSINESS", SourceType: 2, SourceExcerpt: "星河科技主营算力服务。", SentenceIDs: []int{1}, QuoteSegments: []string{"星河科技主营算力服务。"},
		SourceURL: "https://example.test/report", CollectedAt: time.UnixMilli(1700000000123), Provider: "test-provider", Model: "test-model", PromptVersion: "test-v1"}
}

func exportRows(t *testing.T, items []candidate.Candidate) [][]string {
	t.Helper()
	var buffer bytes.Buffer
	if err := Write(&buffer, items); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&buffer).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
func readRows(t *testing.T, rows [][]string) ([]candidate.ReviewRequest, error) {
	t.Helper()
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.WriteAll(rows); err != nil {
		t.Fatal(err)
	}
	return ReadDecisions(&buffer)
}

func TestReviewCSVRoundTripAndBlankDecisions(t *testing.T) {
	rows := exportRows(t, []candidate.Candidate{fixture()})
	requests, err := readRows(t, rows)
	if err != nil || len(requests) != 0 {
		t.Fatalf("空决定被执行: %+v %v", requests, err)
	}
	rows[1][2], rows[1][3] = "reject", "原文未明确支持归属"
	requests, err = readRows(t, rows)
	if err != nil || len(requests) != 1 || requests[0].CandidateID != 1 || requests[0].Reason != rows[1][3] {
		t.Fatalf("CSV 回传失败: %+v %v", requests, err)
	}
	first := requests[0]
	requests, err = readRows(t, rows)
	if err != nil || requests[0] != first {
		t.Fatal("同一 CSV 重读改变幂等请求")
	}
	var buffer bytes.Buffer
	if err := Write(&buffer, nil); err != nil {
		t.Fatal(err)
	}
	if requests, err := ReadDecisions(strings.NewReader("\ufeff" + buffer.String())); err != nil || len(requests) != 0 {
		t.Fatalf("UTF-8 BOM 不兼容: %v", err)
	}
}

func TestReviewCSVRejectsTamperingBeforeReturningAnyDecision(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([][]string)
	}{
		{"正文被改", func(rows [][]string) { rows[1][14] = "星河科技主营其他业务。" }},
		{"代码被改", func(rows [][]string) { rows[1][11] = "688002.SH" }},
		{"句子被改", func(rows [][]string) { rows[1][15] = "[2]" }},
		{"追溯被改", func(rows [][]string) { rows[1][21] = "test-another-model" }},
		{"摘要被改", func(rows [][]string) { rows[1][1] = strings.Repeat("0", 64) }},
		{"表头被改", func(rows [][]string) { rows[0][0] = "actor_id" }},
		{"候选重复", func(rows [][]string) { rows[2] = slicesOf(rows[1]) }},
		{"空决定夹理由", func(rows [][]string) { rows[1][2] = ""; rows[1][3] = "有理由但无决定" }},
		{"无效来源类型", func(rows [][]string) { rows[1][13] = "1000" }},
		{"节选标记错误", func(rows [][]string) { rows[1][17] = "unknown" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			one, two := fixture(), fixture()
			two.ID = 2
			rows := exportRows(t, []candidate.Candidate{one, two})
			rows[1][2], rows[2][2] = "approve", "approve"
			tc.change(rows)
			requests, err := readRows(t, rows)
			if err == nil || len(requests) != 0 {
				t.Fatalf("坏 CSV 返回了部分决定: %+v %v", requests, err)
			}
		})
	}
	if _, err := ReadDecisions(strings.NewReader(strings.Repeat("x", MaxBytes+1))); err == nil {
		t.Fatal("超大 CSV 未拒绝")
	}
}

func slicesOf(row []string) []string { return append([]string(nil), row...) }

func TestReviewCSVFormulaEscapeIsReversible(t *testing.T) {
	for _, value := range []string{"=1+1", "+cmd", "-1", "@SUM(1)", "  =formula", " \t=formula", " '\tformula", "\tformula", "\rformula", "'原文", "'=formula", "普通原文", ""} {
		if got := unescapeCell(escapeCell(value)); got != value {
			t.Fatalf("转义破坏原文: %q -> %q", value, got)
		}
		if needsEscape(value) && !strings.HasPrefix(escapeCell(value), "'") {
			t.Fatal("公式未加防护")
		}
	}
	item := fixture()
	item.SourceExcerpt = "=星河科技主营算力服务。"
	item.QuoteSegments = []string{item.SourceExcerpt}
	rows := exportRows(t, []candidate.Candidate{item})
	if !strings.HasPrefix(rows[1][14], "'=") {
		t.Fatal("公式开头的原文未保护")
	}
	rows[1][2] = "approve"
	requests, err := readRows(t, rows)
	if err != nil || len(requests) != 1 {
		t.Fatalf("安全转义影响原文摘要: %v", err)
	}
	segments, err := json.Marshal(item.QuoteSegments)
	if err != nil || rows[1][16] != string(segments) {
		t.Fatal("分段原文改变")
	}
}
