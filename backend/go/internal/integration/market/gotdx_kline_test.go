package market

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

func TestSortDedupeKlineBars(t *testing.T) {
	d1 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)
	bars := []model.StockDailyKline{
		{TradeDate: d3, Close: decimal.NewFromInt(3)},
		{TradeDate: d1, Close: decimal.NewFromInt(1)},
		{TradeDate: d2, Close: decimal.NewFromInt(2)},
		{TradeDate: d2, Close: decimal.NewFromInt(22)}, // 同日重复，保留后者
	}
	out := sortDedupeKlineBars(bars)
	require.Len(t, out, 3)
	require.Equal(t, 2, out[0].TradeDate.Day())
	require.Equal(t, int64(22), out[1].Close.IntPart())
	require.Equal(t, 4, out[2].TradeDate.Day())
}
