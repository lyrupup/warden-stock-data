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

func TestFilterPendingCodes(t *testing.T) {
	latest := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	wmMap := map[string]time.Time{
		"600519": latest,                                       // 已最新 → 跳过
		"000001": time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC), // 落后 → 保留
	}
	// 600519 最新跳过；000001 落后保留；300750 无水位（新股）保留。
	out := service.FilterPendingCodesForTest(
		[]string{"600519", "000001", "300750"}, wmMap, &latest,
	)
	require.ElementsMatch(t, []string{"000001", "300750"}, out)
}

func TestFilterPendingCodesEmptyDB(t *testing.T) {
	// latest 为 nil（库空）时视为首次全量，返回全部代码。
	out := service.FilterPendingCodesForTest(
		[]string{"600519", "000001"}, map[string]time.Time{}, nil,
	)
	require.Equal(t, []string{"600519", "000001"}, out)
}
