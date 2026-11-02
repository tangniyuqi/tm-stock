package dossier

func (p *CardComparisonPlan) MatchesFields(fields *SupportPlan) bool {
	if !p.Valid() || !fields.Valid() || p.identity != fields.identity || p.draftDigest != fields.draftDigest {
		return false
	}
	return comparisonCardsCovered(p.checks, fields.checks)
}

func comparisonCardsCovered(needed, available []supportCheck) bool {
	keys := []string{}
	for _, check := range needed {
		if check.Field == "section" {
			keys = append(keys, check.Card)
		}
	}
	if len(keys) != 2 {
		return false
	}
	for _, key := range keys {
		found := false
		for _, check := range available {
			if check.Card == key && check.Field == "section" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (p *CardComparisonPreparation) MatchesFields(fields *SupportPreparation) bool {
	return p.Valid() && fields.Valid() && p.base.identity == fields.identity && p.base.draftDigest == fields.draftDigest && comparisonCardsCovered(p.base.checks, fields.checks)
}

func (p *CardComparisonPreparation) Config() CardComparisonConfig {
	if !p.Valid() {
		return CardComparisonConfig{}
	}
	return p.cfg
}

// CheckSupport只核两个已捕获回执的一致性；不产生新的支持事实或模型投票。
func (p *CardComparisonPlan) CheckSupport(fields *SupportPlan, support *SupportReview, comparison *CardComparisonReview) error {
	if !p.MatchesFields(fields) {
		return ErrCardComparison
	}
	if _, err := fields.VisibleReview(support); err != nil {
		return ErrCardComparison
	}
	if _, err := p.VisibleReview(comparison); err != nil {
		return ErrCardComparison
	}
	verdicts := map[string]string{}
	for _, row := range support.Results() {
		verdicts[row.Card+"/"+row.Field] = row.Verdict
	}
	for _, key := range p.CardKeys() {
		if verdicts[key+"/section"] != "supported" {
			return ErrCardComparison
		}
	}
	for i, axis := range p.axes {
		for _, side := range []struct {
			id      int
			verdict string
		}{{axis.LeftCheckID, comparison.axes[i].Left}, {axis.RightCheckID, comparison.axes[i].Right}} {
			if side.id == 0 {
				continue
			}
			if side.id < 1 || side.id > len(p.checks) {
				return ErrCardComparison
			}
			check := p.checks[side.id-1]
			if verdicts[check.Card+"/"+check.Field] != side.verdict {
				return ErrCardComparison
			}
		}
	}
	return nil
}
