package service

import (
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

const reportComparisonNotice = "仅比较本次两张候选资料卡；不同口径不等于事实冲突或更正，原句出处提示不代表来源独立性已核实。"

// 只做纯投影；任务归属与当前双用途、会员、发布、来源必须先由原报告读取事务核验。
func dossierComparisonDTO(plan *dossier.CardComparisonPlan, review *dossier.CardComparisonReview, bound *dossier.BoundDraft) (*dto.AIReportDossierComparison, error) {
	if !plan.Valid() || review == nil || review.ResolvedCount() == 0 || bound == nil || plan.DraftDigest() != bound.Digest() || review.DraftDigest() != bound.Digest() {
		return nil, points.ErrInvariant
	}
	axes, err := plan.ReviewedAxes(review)
	if err != nil {
		return nil, err
	}
	view, err := dossierView([]*dossier.BoundDraft{bound})
	if err != nil {
		return nil, err
	}
	keys := plan.CardKeys()
	if len(keys) != 2 {
		return nil, points.ErrInvariant
	}
	cards := make([]dto.AIDossierCard, 0, 2)
	for _, key := range keys {
		for _, card := range view.Versions[0].Cards {
			if card.Key == key {
				cards = append(cards, card)
			}
		}
	}
	if len(cards) != 2 {
		return nil, points.ErrInvariant
	}
	out := &dto.AIReportDossierComparison{Notice: reportComparisonNotice, Focus: plan.Config().Focus, Outcome: review.Outcome(), Wording: review.Wording(), StatementOrigins: review.StatementOriginRelation(),
		Left: dto.AIReportComparisonCard{Key: cards[0].Key, Section: cards[0].Section, Statement: cards[0].Quote}, Right: dto.AIReportComparisonCard{Key: cards[1].Key, Section: cards[1].Section, Statement: cards[1].Quote}, Axes: make([]dto.AIReportComparisonAxis, 0, len(axes))}
	for _, axis := range axes {
		left, err := dossierComparisonSide(cards[0], axis.Field, axis.Left)
		if err != nil {
			return nil, err
		}
		right, err := dossierComparisonSide(cards[1], axis.Field, axis.Right)
		if err != nil {
			return nil, err
		}
		out.Axes = append(out.Axes, dto.AIReportComparisonAxis{Field: axis.Field, Required: axis.Required, Left: left, Right: right, Relation: axis.Relation, Reason: axis.Reason})
	}
	return out, nil
}

func dossierComparisonSide(card dto.AIDossierCard, key, verdict string) (dto.AIReportComparisonSide, error) {
	for _, field := range card.Fields {
		if field.Key != key {
			continue
		}
		if field.State == "unknown" && field.Quote == nil && verdict == "unknown" {
			return dto.AIReportComparisonSide{State: "unknown", Verdict: verdict}, nil
		}
		if field.State != "known" || field.Quote == nil || verdict == "unknown" {
			return dto.AIReportComparisonSide{}, points.ErrInvariant
		}
		q := *field.Quote
		return dto.AIReportComparisonSide{State: "known", Verdict: verdict, Quote: &q}, nil
	}
	return dto.AIReportComparisonSide{}, points.ErrInvariant
}
