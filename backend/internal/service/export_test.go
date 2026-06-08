package service

import (
	"time"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

// FilterPendingCodesForTest 暴露内部纯函数 filterPendingCodes 供外部测试包使用。
func FilterPendingCodesForTest(codes []string, wmMap map[string]time.Time, latest *time.Time) []string {
	return filterPendingCodes(codes, wmMap, latest)
}

// FilterFromWatermarkForTest 暴露内部纯函数 filterFromWatermark 供外部测试包使用。
func FilterFromWatermarkForTest(bars []model.StockDailyKline, wm *time.Time) []model.StockDailyKline {
	return filterFromWatermark(bars, wm)
}

// EnrichQuoteNamesForTest 暴露 enrichQuoteNames 供外部测试包使用。
func EnrichQuoteNamesForTest(quotes []model.StockQuote, names map[string]string) {
	enrichQuoteNames(quotes, names)
}

// UntradedFromQuotesForTest 暴露内部纯函数 untradedFromQuotes 供外部测试包使用。
func UntradedFromQuotesForTest(quotes []model.StockQuote) bool {
	return untradedFromQuotes(quotes)
}

// WindowBarsForTest 暴露内部纯函数 windowBars 供外部测试包使用。
func WindowBarsForTest(bars []model.StockDailyKline, offset, limit int) ([]model.StockDailyKline, bool) {
	return windowBars(bars, offset, limit)
}
