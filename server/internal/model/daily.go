package model

import "time"

type ThemeDailyItem struct {
	ID        int64
	BizDate   time.Time
	ThemeID   int64
	Title     string
	ThemeName string
	Source    string
	SourceURL string
	PublishAt time.Time
	ChangePct *float64
	Caliber   string
}

type TradingDay struct {
	Date         time.Time
	IsTradingDay bool
}
