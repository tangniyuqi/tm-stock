package service

import (
	"context"

	"github.com/tangniyuqi/tm-stock/server/internal/ai/dossier"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
)

// ViewVersions必须先完成整个单/双版当前授权读取，再转换全部结果；失败不返回部分响应。
func (s *AIDossierService) ViewVersions(ctx context.Context, token string, user, theme int64, versions []int64) (dto.AIDossierView, error) {
	bound, err := s.ReadVersions(ctx, token, user, theme, versions)
	if err != nil {
		return dto.AIDossierView{}, err
	}
	out, err := dossierView(bound)
	if err != nil {
		return dto.AIDossierView{}, err
	}
	if err = ctx.Err(); err != nil {
		return dto.AIDossierView{}, err
	}
	return out, nil
}
func dossierView(versions []*dossier.BoundDraft) (dto.AIDossierView, error) {
	out := dto.AIDossierView{Notice: "以下为所选版本的资料摘录；引用存在不等于已核实表述，未知字段表示资料不足。", Versions: []dto.AIDossierVersion{}}
	if len(versions) < 1 || len(versions) > 2 {
		return dto.AIDossierView{}, dossier.ErrInvalidDraft
	}
	for _, bound := range versions {
		identity := bound.Identity()
		if identity.ThemeID <= 0 || identity.Version <= 0 || identity.CreatedAt <= 0 || out.ThemeID != 0 && out.ThemeID != identity.ThemeID {
			return dto.AIDossierView{}, dossier.ErrInvalidDraft
		}
		out.ThemeID = identity.ThemeID
		origins := map[int64]dossier.Origin{}
		for _, o := range bound.Origins() {
			origins[o.SnapshotID] = o
		}
		quote := func(text, context string, ref dossier.SentenceRef) (dto.AIDossierQuote, error) {
			o, ok := origins[ref.SnapshotID]
			if !ok || o.Hash != ref.Hash {
				return dto.AIDossierQuote{}, dossier.ErrInvalidDraft
			}
			return dto.AIDossierQuote{Text: text, Context: context, SourceType: o.Category, SourceURL: o.URL, CollectedAt: o.FetchedAt, SnapshotID: ref.SnapshotID, Sentence: ref.Number, StartByte: ref.StartByte, EndByte: ref.EndByte}, nil
		}
		v := dto.AIDossierVersion{Version: identity.Version, CompiledAt: identity.CreatedAt, Cards: []dto.AIDossierCard{}, Gaps: []dto.AIDossierGap{}}
		for _, card := range bound.Cards() {
			q, err := quote(card.Statement, card.Statement, card.StatementRef)
			if err != nil {
				return dto.AIDossierView{}, err
			}
			c := dto.AIDossierCard{Key: card.Key, Section: card.Section, Quote: q, Fields: []dto.AIDossierField{}}
			for _, field := range []struct {
				key   string
				value dossier.BoundField
			}{
				{"subject", card.Subject}, {"publishedDate", card.PublishedDate}, {"effectiveDate", card.EffectiveDate}, {"period", card.Period}, {"scope", card.Scope}, {"stage", card.Stage}, {"basis", card.Basis}, {"unit", card.Unit},
			} {
				f := dto.AIDossierField{Key: field.key, State: field.value.State}
				if f.State == "known" {
					q, err := quote(field.value.Text, field.value.Context, field.value.Ref)
					if err != nil {
						return dto.AIDossierView{}, err
					}
					f.Quote = &q
				}
				c.Fields = append(c.Fields, f)
			}
			v.Cards = append(v.Cards, c)
		}
		for _, g := range bound.Gaps() {
			v.Gaps = append(v.Gaps, dto.AIDossierGap{Key: g.Key, Section: g.Section, Reason: g.Reason})
		}
		out.Versions = append(out.Versions, v)
	}
	if len(versions) == 2 {
		diff, err := dossier.CompareVersions(versions[0], versions[1])
		if err != nil {
			return dto.AIDossierView{}, err
		}
		comparison := &dto.AIDossierComparison{Notice: "仅比较档案中已引用的原句、字段和资料缺口；本版未出现不表示事实撤回，摘录调整不等于事实更正。", Added: diff.Added, NotPresent: diff.NotPresent, Changed: []dto.AIDossierCardChange{}, Gaps: []dto.AIDossierGapChange{}, QuotedSentenceOrderChanged: diff.QuotedSentenceOrderChanged}
		for _, c := range diff.Changed {
			comparison.Changed = append(comparison.Changed, dto.AIDossierCardChange{BeforeKey: c.BeforeKey, AfterKey: c.AfterKey, QuoteChanged: c.QuoteChanged, SourceChanged: c.SourceChanged, ReferenceChanged: c.ReferenceChanged, SectionChanged: c.SectionChanged, Fields: c.Fields})
		}
		for _, g := range diff.Gaps {
			comparison.Gaps = append(comparison.Gaps, dto.AIDossierGapChange{BeforeKey: g.BeforeKey, AfterKey: g.AfterKey})
		}
		out.Comparison = comparison
	}
	return out, nil
}
