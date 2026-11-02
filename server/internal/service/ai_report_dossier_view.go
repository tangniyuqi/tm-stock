package service

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/finalreport"
	"github.com/tangniyuqi/tm-stock/server/internal/ai/task"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/points"
)

func reportDossierReview(ctx context.Context, tx finalreport.Tx, v task.Task, policy *finalreport.Policy) (*dto.AIReportDossierReview, error) {
	plan, review := policy.DossierProof()
	if plan == nil {
		return nil, nil
	}
	if v.Status != task.Succeeded || !plan.MatchesTask(v) || review == nil || review.ResolvedCount() == 0 {
		return nil, points.ErrInvariant
	}
	registry, ok := tx.(interface {
		DossierRegistry() dossier.SupportRegistryTx
	})
	if !ok || registry.DossierRegistry() == nil {
		return nil, points.ErrInvariant
	}
	r := registry.DossierRegistry()
	registration, err := r.SupportManifest(ctx, plan.ComponentDigest())
	if err != nil {
		return nil, err
	}
	manifest, err := registration.Manifest()
	if err != nil {
		return nil, err
	}
	bound, err := readDossierVersions(ctx, r.Dossiers(), v.UserID, v.SubjectID, []int64{manifest.Identity().Version})
	if err != nil {
		return nil, err
	}
	if len(bound) != 1 || bound[0].Digest() != manifest.DraftDigest() || bound[0].Digest() != review.DraftDigest() {
		return nil, points.ErrInvariant
	}
	out, err := dossierReviewDTO(plan, review, bound[0])
	if err != nil {
		return nil, err
	}
	comparison, compared := policy.ComparisonProof()
	if comparison == nil && compared == nil {
		return out, nil
	}
	if !comparison.MatchesTask(v) || comparison.DraftDigest() != bound[0].Digest() || comparison.CheckSupport(plan, review, compared) != nil {
		return nil, points.ErrInvariant
	}
	out.Comparison, err = dossierComparisonDTO(comparison, compared, bound[0])
	if err != nil {
		return nil, err
	}
	return out, nil
}

func dossierReviewDTO(plan *dossier.SupportPlan, review *dossier.SupportReview, bound *dossier.BoundDraft) (*dto.AIReportDossierReview, error) {
	if bound == nil || review == nil || bound.Digest() != review.DraftDigest() || review.ResolvedCount() == 0 {
		return nil, points.ErrInvariant
	}
	fields, err := plan.ReviewedFields(review)
	if err != nil {
		return nil, err
	}
	view, err := dossierView([]*dossier.BoundDraft{bound})
	if err != nil {
		return nil, err
	}
	version := view.Versions[0]
	out := &dto.AIReportDossierReview{Version: version.Version, CompiledAt: version.CompiledAt, Notice: "仅核对本次获授权资料；所核内容为候选字段，不代表已证实事实。未列字段未在本次核验范围内。", Items: make([]dto.AIReportDossierField, 0, len(fields))}
	for _, f := range fields {
		var card *dto.AIDossierCard
		for i := range version.Cards {
			if version.Cards[i].Key == f.Card {
				card = &version.Cards[i]
				break
			}
		}
		if card == nil || card.Section != f.Section || !dossierReviewQuoteMatches(card.Quote, f.Statement, f.StatementRef) {
			return nil, points.ErrInvariant
		}
		var checked *dto.AIDossierQuote
		if f.Field == "section" {
			checked = &card.Quote
		} else {
			for _, field := range card.Fields {
				if field.Key == f.Field && field.State == "known" {
					checked = field.Quote
					break
				}
			}
		}
		if checked == nil || !dossierReviewQuoteMatches(*checked, f.Value, f.Reference) {
			return nil, points.ErrInvariant
		}
		out.Items = append(out.Items, dto.AIReportDossierField{CardKey: f.Card, Section: f.Section, Field: f.Field, Verdict: f.Verdict, Reason: f.Reason, Checked: *checked, Statement: card.Quote})
	}
	return out, nil
}

func dossierReviewQuoteMatches(q dto.AIDossierQuote, text string, ref dossier.SentenceRef) bool {
	return q.Text == text && q.SnapshotID == ref.SnapshotID && q.Sentence == ref.Number && q.StartByte == ref.StartByte && q.EndByte == ref.EndByte
}
