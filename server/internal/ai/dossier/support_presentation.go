package dossier

// ReviewedField是原计划与合法回执的纯投影，不含模型自由输出或批准信息。
// 原文出处须由调用方在当前授权下回取并逐项核对，不能直接序列化本结构。
type ReviewedField struct {
	Card, Section, Field, Value, Statement, Verdict, Reason string
	Reference, StatementRef                                 SentenceRef
}

func (ReviewedField) String() string     { return "字段核验展示绑定（原文已隐藏）" }
func (v ReviewedField) GoString() string { return v.String() }

func (p *SupportPlan) ReviewedFields(r *SupportReview) ([]ReviewedField, error) {
	if _, err := p.VisibleReview(r); err != nil {
		return nil, err
	}
	out := make([]ReviewedField, 0, len(p.checks))
	for i, check := range p.checks {
		row := r.results[i]
		out = append(out, ReviewedField{Card: check.Card, Section: check.Section, Field: check.Field, Value: check.Value, Statement: check.Statement, Verdict: row.Verdict, Reason: row.Reason, Reference: check.Reference, StatementRef: check.StatementRef})
	}
	return out, nil
}
