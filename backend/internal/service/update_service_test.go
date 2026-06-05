package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

func TestFilterAfterWatermark(t *testing.T) {
	wm := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
	bars := []model.StockDailyKline{
		{TradeDate: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)},
		{TradeDate: time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)},
	}
	out := filterBars(bars, &wm)
	require.Len(t, out, 1)
	require.Equal(t, 6, out[0].TradeDate.Day())
}

func filterBars(bars []model.StockDailyKline, wm *time.Time) []model.StockDailyKline {
	out := make([]model.StockDailyKline, 0)
	for _, b := range bars {
		if wm == nil || b.TradeDate.After(*wm) {
			out = append(out, b)
		}
	}
	return out
}
