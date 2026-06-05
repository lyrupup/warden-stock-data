package service

import (
	"time"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

// FilterPendingCodesForTest 暴露内部纯函数 filterPendingCodes 供外部测试包使用。
func FilterPendingCodesForTest(codes []string, wmMap map[string]time.Time, latest *time.Time) []string {
	return filterPendingCodes(codes, wmMap, latest)
}

// EnrichQuoteNamesForTest 暴露 enrichQuoteNames 供外部测试包使用。
func EnrichQuoteNamesForTest(quotes []model.StockQuote, names map[string]string) {
	enrichQuoteNames(quotes, names)
}
