package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/tangniyuqi/tm-stock/server/internal/model"
	"time"
)

type DailyRepository struct{ db *sql.DB }

func NewDailyRepository(db *sql.DB) *DailyRepository { return &DailyRepository{db: db} }
func (r *DailyRepository) ListDaily(ctx context.Context, date time.Time, sortKey string, page, size int) ([]model.ThemeDailyItem, int, error) {
	order := "d.publish_at DESC, d.id DESC"
	if sortKey == "changePct" {
		order = "q.change_pct DESC, d.id DESC"
	}
	q := `SELECT d.id,d.biz_date,d.theme_id,d.title,t.name,d.source,d.source_url,d.publish_at,q.change_pct,q.caliber FROM theme_daily_item d JOIN addon_quant_theme t ON t.id=d.theme_id AND t.deleted_at IS NULL LEFT JOIN theme_daily_quote q ON q.biz_date=d.biz_date AND q.theme_id=d.theme_id WHERE d.biz_date=? ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, q, date.Format("2006-01-02"), size, (page-1)*size)
	if err != nil {
		return nil, 0, fmt.Errorf("查询题材动态失败: %w", err)
	}
	defer rows.Close()
	out := make([]model.ThemeDailyItem, 0, size)
	for rows.Next() {
		var x model.ThemeDailyItem
		var pct sql.NullFloat64
		if err := rows.Scan(&x.ID, &x.BizDate, &x.ThemeID, &x.Title, &x.ThemeName, &x.Source, &x.SourceURL, &x.PublishAt, &pct, &x.Caliber); err != nil {
			return nil, 0, err
		}
		if pct.Valid {
			x.ChangePct = &pct.Float64
		}
		out = append(out, x)
	}
	var total int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM theme_daily_item WHERE biz_date=?", date.Format("2006-01-02")).Scan(&total)
	return out, total, err
}
func (r *DailyRepository) Calendar(ctx context.Context, date time.Time) (model.TradingDay, error) {
	var d model.TradingDay
	err := r.db.QueryRowContext(ctx, "SELECT biz_date,is_trading_day FROM trading_calendar WHERE biz_date=?", date.Format("2006-01-02")).Scan(&d.Date, &d.IsTradingDay)
	if err == sql.ErrNoRows {
		return model.TradingDay{Date: date, IsTradingDay: false}, nil
	}
	return d, err
}
