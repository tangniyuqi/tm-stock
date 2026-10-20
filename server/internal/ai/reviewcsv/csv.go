// Package reviewcsv 定义可核对原文的内部审核 CSV；只能编辑决定与原因。
package reviewcsv

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/candidate"
)

const MaxRows = 2000
const MaxBytes = 16 << 20

var header = []string{"candidate_id", "evidence_digest", "decision", "reason", "request_id", "snapshot_id", "claim_index",
	"theme_id", "theme_key", "stock_id", "entity_key", "ts_code", "predicate", "source_type", "source_excerpt",
	"sentence_ids", "quote_segments", "excerpt_trimmed", "source_url", "collected_at", "provider", "model", "prompt_version"}

// Write 输出 UTF-8 CSV；文本做可逆的公式注入防护，不改动原文摘要。
func Write(w io.Writer, items []candidate.Candidate) error {
	if len(items) > MaxRows {
		return fmt.Errorf("审核 CSV 超过 %d 条，请分批处理", MaxRows)
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, item := range items {
		if item.ID <= 0 {
			return fmt.Errorf("审核 CSV 候选编号无效")
		}
		digest, err := candidate.EvidenceDigest(item)
		if err != nil {
			return err
		}
		var requestID [16]byte
		if _, err := rand.Read(requestID[:]); err != nil {
			return fmt.Errorf("生成审核请求编号失败: %w", err)
		}
		ids, err := json.Marshal(item.SentenceIDs)
		if err != nil {
			return err
		}
		segments, err := json.Marshal(item.QuoteSegments)
		if err != nil {
			return err
		}
		row := []string{number(item.ID), digest, "", "", hex.EncodeToString(requestID[:]), number(item.SnapshotID), strconv.Itoa(item.ClaimIndex),
			number(item.ThemeID), item.ThemeKey, number(item.StockID), item.EntityKey, item.TsCode, item.Predicate, number(int64(item.SourceType)),
			item.SourceExcerpt, string(ids), string(segments), strconv.FormatBool(item.ExcerptTrimmed), item.SourceURL, number(item.CollectedAt.UnixMilli()),
			item.Provider, item.Model, item.PromptVersion}
		for i := range row {
			row[i] = escapeCell(row[i])
		}
		if err := writer.Write(row); err != nil {
			return err
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		if buffer.Len() > MaxBytes {
			return fmt.Errorf("审核 CSV 超过 16 MiB，请分批处理")
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	n, err := w.Write(buffer.Bytes())
	if err != nil {
		return fmt.Errorf("写审核 CSV 失败: %w", err)
	}
	if n != buffer.Len() {
		return fmt.Errorf("写审核 CSV 失败: %w", io.ErrShortWrite)
	}
	return nil
}

// ReadDecisions 先校验整份 CSV 再返回请求，避免后半段篡改导致前半段已经执行。
func ReadDecisions(r io.Reader) ([]candidate.ReviewRequest, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读审核 CSV 失败: %w", err)
	}
	if len(raw) > MaxBytes {
		return nil, fmt.Errorf("审核 CSV 超过 16 MiB")
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})))
	columns, err := reader.Read()
	if err != nil || !slices.Equal(columns, header) {
		return nil, fmt.Errorf("审核 CSV 表头必须与导出一致")
	}
	requests := make([]candidate.ReviewRequest, 0)
	seenCandidates := map[int64]bool{}
	seenRequests := map[string]bool{}
	for line := 2; ; line++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("审核 CSV 第 %d 条记录格式无效", line)
		}
		if line > MaxRows+1 {
			return nil, fmt.Errorf("审核 CSV 超过 %d 条", MaxRows)
		}
		for i := range row {
			row[i] = unescapeCell(row[i])
		}
		request, err := decodeRow(row)
		if err != nil {
			return nil, fmt.Errorf("审核 CSV 第 %d 条记录未通过校验: %w", line, err)
		}
		if seenCandidates[request.CandidateID] || seenRequests[request.RequestID] {
			return nil, fmt.Errorf("审核 CSV 含重复候选或请求编号")
		}
		seenCandidates[request.CandidateID], seenRequests[request.RequestID] = true, true
		if request.Decision == "" {
			continue
		}
		if err := request.Validate(); err != nil {
			return nil, fmt.Errorf("审核 CSV 第 %d 条决定无效", line)
		}
		requests = append(requests, request)
	}
	return requests, nil
}

func decodeRow(row []string) (candidate.ReviewRequest, error) {
	if len(row) != len(header) {
		return candidate.ReviewRequest{}, fmt.Errorf("列数错误")
	}
	var item candidate.Candidate
	integers := map[int]*int64{0: &item.ID, 5: &item.SnapshotID, 7: &item.ThemeID, 9: &item.StockID}
	for index, cell := range integers {
		value, err := strconv.ParseInt(row[index], 10, 64)
		if err != nil {
			return candidate.ReviewRequest{}, fmt.Errorf("编号不是整数")
		}
		*cell = value
	}
	claimIndex, err := strconv.ParseInt(row[6], 10, 16)
	if err != nil {
		return candidate.ReviewRequest{}, fmt.Errorf("命题索引无效")
	}
	item.ClaimIndex = int(claimIndex)
	sourceType, err := strconv.ParseInt(row[13], 10, 8)
	if err != nil {
		return candidate.ReviewRequest{}, fmt.Errorf("来源类型无效")
	}
	item.SourceType = int8(sourceType)
	collectedAt, err := strconv.ParseInt(row[19], 10, 64)
	if err != nil {
		return candidate.ReviewRequest{}, fmt.Errorf("采集时点无效")
	}
	item.CollectedAt = time.UnixMilli(collectedAt).UTC()
	item.ExcerptTrimmed, err = strconv.ParseBool(row[17])
	if err != nil {
		return candidate.ReviewRequest{}, fmt.Errorf("节选标记无效")
	}
	if err := json.Unmarshal([]byte(row[15]), &item.SentenceIDs); err != nil {
		return candidate.ReviewRequest{}, fmt.Errorf("句子编号无效")
	}
	if err := json.Unmarshal([]byte(row[16]), &item.QuoteSegments); err != nil {
		return candidate.ReviewRequest{}, fmt.Errorf("原文分段无效")
	}
	item.ThemeKey, item.EntityKey, item.TsCode, item.Predicate = row[8], row[10], row[11], row[12]
	item.SourceExcerpt, item.SourceURL, item.Provider, item.Model, item.PromptVersion = row[14], row[18], row[20], row[21], row[22]
	digest, err := candidate.EvidenceDigest(item)
	if err != nil {
		return candidate.ReviewRequest{}, err
	}
	if digest != row[1] {
		return candidate.ReviewRequest{}, candidate.ErrEvidenceChanged
	}
	request := candidate.ReviewRequest{CandidateID: item.ID, EvidenceDigest: digest, Decision: row[2], Reason: row[3], RequestID: row[4]}
	// 空决定是暂不处理，但其他控制字段仍须合法，避免遗漏损坏的请求编号。
	check := request
	if check.Decision == "" {
		if check.Reason != "" {
			return request, candidate.ErrInvalidReview
		}
		check.Decision = "approve"
	}
	if err := check.Validate(); err != nil {
		return request, err
	}
	return request, nil
}

func number(n int64) string { return strconv.FormatInt(n, 10) }

func needsEscape(value string) bool {
	if value == "" {
		return false
	}
	if strings.ContainsRune("'\t\r\n", rune(value[0])) {
		return true
	}
	value = strings.TrimLeft(value, " ")
	return value != "" && strings.ContainsRune("'=+-@\t\r\n", rune(value[0]))
}
func escapeCell(value string) string {
	if needsEscape(value) {
		return "'" + value
	}
	return value
}
func unescapeCell(value string) string {
	if strings.HasPrefix(value, "'") && needsEscape(value[1:]) {
		return value[1:]
	}
	return value
}
