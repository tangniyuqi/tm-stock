package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/qa"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

// AIReportReader 是已实现本人报告当前可读性复核的读取入口；不得用历史缓存或他人报告替代。
type AIReportReader interface {
	Read(context.Context, int64, int64) (dto.AIReport, error)
}

// AIReportQAEvidence 把当前可读报告转换为报告追问证据；每次调用都重新读取并复核报告。
type AIReportQAEvidence struct {
	reports AIReportReader
}

func NewAIReportQAEvidence(reports AIReportReader) (*AIReportQAEvidence, error) {
	if reports == nil {
		return nil, qa.ErrInvalid
	}
	return &AIReportQAEvidence{reports: reports}, nil
}

type reportEvidenceDigest struct {
	TaskID      int64    `json:"taskId"`
	SubjectID   int64    `json:"subjectId"`
	GeneratedAt int64    `json:"generatedAt"`
	Quotes      []string `json:"quotes"`
}

// ForQA 只取 AI 章节的公开引文；公司事实行不是本章节的问答证据，不进入追问范围。
func (s *AIReportQAEvidence) ForQA(ctx context.Context, uid, taskID int64) (qa.ReportEvidence, error) {
	if s == nil || s.reports == nil || ctx == nil || uid <= 0 || taskID <= 0 {
		return qa.ReportEvidence{}, qa.ErrInvalid
	}
	report, err := s.reports.Read(ctx, uid, taskID)
	if err != nil {
		if errors.Is(err, finalreport.ErrUnavailable) {
			return qa.ReportEvidence{}, qa.ErrUnavailable
		}
		return qa.ReportEvidence{}, err
	}
	if report.TaskID != taskID || report.SubjectType != "theme" || report.SubjectID <= 0 || report.SubjectName == "" || report.GeneratedAt <= 0 {
		return qa.ReportEvidence{}, qa.ErrProtocol
	}
	quotes := make([]qa.ReportQuote, 0, qa.ReportMaxQuotes)
	digests := make([]string, 0, qa.ReportMaxQuotes)
	for _, section := range report.Sections {
		if section.Key == "companies" {
			continue
		}
		for _, quote := range section.Quotes {
			if len(quotes) >= qa.ReportMaxQuotes {
				return qa.ReportEvidence{}, qa.ErrUnavailable
			}
			quotes = append(quotes, qa.ReportQuote{
				Section:     section.Key,
				Text:        quote.Text,
				SourceType:  quote.SourceType,
				SourceURL:   quote.SourceURL,
				CollectedAt: time.UnixMilli(quote.CollectedAt).UTC(),
			})
			// 出处与原文分别哈希，避免分隔符歧义，也不把正文拼进日志或错误。
			urlSum := sha256.Sum256([]byte(quote.SourceURL))
			textSum := sha256.Sum256([]byte(quote.Text))
			digests = append(digests, hex.EncodeToString(urlSum[:])+hex.EncodeToString(textSum[:]))
		}
	}
	if len(quotes) == 0 {
		return qa.ReportEvidence{}, qa.ErrUnavailable
	}
	raw, err := json.Marshal(reportEvidenceDigest{TaskID: report.TaskID, SubjectID: report.SubjectID, GeneratedAt: report.GeneratedAt, Quotes: digests})
	if err != nil {
		return qa.ReportEvidence{}, qa.ErrProtocol
	}
	sum := sha256.Sum256(raw)
	return qa.ReportEvidence{
		TaskID:      report.TaskID,
		SubjectID:   report.SubjectID,
		SubjectType: report.SubjectType,
		SubjectName: report.SubjectName,
		GeneratedAt: time.UnixMilli(report.GeneratedAt).UTC(),
		Digest:      hex.EncodeToString(sum[:]),
		Quotes:      quotes,
	}, nil
}
