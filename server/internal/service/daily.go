package service

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"github.com/tangniyuqi/tm-stock/server/internal/model"
	"time"
)

type DailyRepository interface {
	ListDaily(context.Context, time.Time, string, int, int) ([]model.ThemeDailyItem, int, error)
	Calendar(context.Context, time.Time) (model.TradingDay, error)
}
type ThemeDailyService struct {
	repo  DailyRepository
	delay int
	mock  bool
}

func NewThemeDailyService(r DailyRepository, delay int, mock bool) *ThemeDailyService {
	return &ThemeDailyService{repo: r, delay: delay, mock: mock}
}
func (s *ThemeDailyService) List(ctx context.Context, date time.Time, sortKey string, page, size int) (*dto.ThemeDailyPageResp, error) {
	day, err := s.repo.Calendar(ctx, date)
	if err != nil {
		return nil, err
	}
	items, total, err := s.repo.ListDaily(ctx, date, sortKey, page, size)
	if err != nil {
		return nil, err
	}
	if !day.IsTradingDay {
		items = nil
		total = 0
	}
	out := make([]dto.ThemeDailyItemResp, 0, len(items))
	for _, x := range items {
		out = append(out, dto.ThemeDailyItemResp{ID: x.ID, Title: x.Title, ThemeID: x.ThemeID, ThemeName: x.ThemeName, ChangePct: x.ChangePct, Caliber: x.Caliber, PublishAt: x.PublishAt.UnixMilli(), Source: x.Source, SourceURL: x.SourceURL})
	}
	reason := ""
	if !day.IsTradingDay {
		reason = "NON_TRADING_DAY"
	} else if total == 0 {
		reason = "NO_DATA"
	}
	return &dto.ThemeDailyPageResp{BizDay: date.Format("2006-01-02"), IsTradingDay: day.IsTradingDay, EmptyReason: reason, QuoteDelayMin: s.delay, QuoteIsMock: s.mock, Sort: sortKey, List: out, Total: total, HasMore: page*size < total}, nil
}
