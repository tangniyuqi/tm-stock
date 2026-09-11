package handler

import (
	"context"
	"github.com/tangniyuqi/tm-stock/server/internal/dto"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type dailyFake struct{ sort string }

func (f *dailyFake) List(_ context.Context, _ time.Time, sort string, _, _ int) (*dto.ThemeDailyPageResp, error) {
	f.sort = sort
	return &dto.ThemeDailyPageResp{BizDay: "2026-09-11", Sort: sort, List: []dto.ThemeDailyItemResp{}}, nil
}

func TestThemeDaily_DefaultSortIsPublishAt(t *testing.T) {
	f := &dailyFake{}
	mux := http.NewServeMux()
	NewThemeDailyHandler(f).Register(mux)
	rec := doDaily(mux, "/api/v1/theme-daily/list")
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，得到 %d: %s", rec.Code, rec.Body.String())
	}
	if f.sort != "publishAt" {
		t.Fatalf("默认排序应为 publishAt，得到 %q", f.sort)
	}
}

func doDaily(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}
