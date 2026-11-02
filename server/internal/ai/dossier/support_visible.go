package dossier

import (
	"fmt"
	"strings"
)

// VisibleReview只呈现原计划所核值和固定标签；权限由当前读取/发布服务另核。
// 不输出模型自由文本，不把候选字段的known解释为支持。
func (p *SupportPlan) VisibleReview(r *SupportReview) (string, error) {
	if !p.Valid() || r == nil || r.inputDigest != p.digest || r.draftDigest != p.draftDigest || !supportDigest(r.payloadDigest) || len(r.results) != len(p.checks) {
		return "", ErrSupportReview
	}
	fields := map[string]string{"section": "章节含义", "subject": "主体", "publishedDate": "发布日期", "effectiveDate": "生效日期", "period": "所属期间", "scope": "适用范围", "stage": "阶段", "basis": "统计口径", "unit": "单位"}
	verdicts := map[string]string{"supported": "原文完整支持", "partial": "原文部分支持", "unsupported": "原文不支持", "uncertain": "资料不足"}
	reasons := map[string]string{"direct_support": "原文直接支持", "partial_support": "原文仅支持部分含义", "not_supported": "缺少原文支持", "insufficient_context": "语境或证据不足", "section_mismatch": "章节含义不符", "subject_mismatch": "主体不符", "time_mismatch": "时间口径不符", "scope_mismatch": "适用范围不符", "stage_mismatch": "阶段不符", "basis_mismatch": "统计口径不符", "unit_mismatch": "单位不符"}
	lines := []string{"专题字段原文核验；以本次获授权资料为范围，候选字段与核验结果分别呈现。"}
	for i, row := range r.results {
		check := p.checks[i]
		if row.ID != check.ID || row.Card != check.Card || row.Field != check.Field || !supportReason(row.Field, row.Verdict, row.Reason) || fields[row.Field] == "" {
			return "", ErrSupportReview
		}
		lines = append(lines, fmt.Sprintf("%s\n所核内容：%s\n核验结果：%s\n原因：%s", fields[row.Field], check.Value, verdicts[row.Verdict], reasons[row.Reason]))
	}
	return strings.Join(lines, "\n"), nil
}

func (r *SupportReview) ResolvedCount() int {
	if r == nil {
		return 0
	}
	n := 0
	for _, row := range r.results {
		if row.Verdict == "supported" || row.Verdict == "partial" || row.Verdict == "unsupported" {
			n++
		}
	}
	return n
}
