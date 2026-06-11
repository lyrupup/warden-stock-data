package service_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/service"
)

func TestUntradedFromQuotes(t *testing.T) {
	// 无任何快照行 → 无行情。
	require.True(t, service.UntradedFromQuotesForTest(nil))
	require.True(t, service.UntradedFromQuotesForTest([]model.StockQuote{}))
	// 有快照行但现价为 0（量价全 0 的未上市/停牌）→ 无行情。
	require.True(t, service.UntradedFromQuotesForTest([]model.StockQuote{
		{StockCode: "688797", Price: decimal.Zero},
	}))
	// 有有效现价 → 正常交易。
	require.False(t, service.UntradedFromQuotesForTest([]model.StockQuote{
		{StockCode: "600519", Price: decimal.NewFromFloat(1262.98)},
	}))
}

func TestFilterFromWatermarkInclusive(t *testing.T) {
	wm := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
	bars := []model.StockDailyKline{
		{TradeDate: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)}, // 早于水位 → 过滤
		{TradeDate: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)}, // 等于水位 → 保留（覆盖最新一日）
		{TradeDate: time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)}, // 晚于水位 → 保留
	}
	out := service.FilterFromWatermarkForTest(bars, &wm)
	require.Len(t, out, 2)
	require.Equal(t, 5, out[0].TradeDate.Day())
	require.Equal(t, 6, out[1].TradeDate.Day())
}

func TestFilterFromWatermarkNil(t *testing.T) {
	// 水位为 nil（新股 / 库空）时返回全部 K 线。
	bars := []model.StockDailyKline{
		{TradeDate: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)},
		{TradeDate: time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC)},
	}
	out := service.FilterFromWatermarkForTest(bars, nil)
	require.Len(t, out, 2)
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
