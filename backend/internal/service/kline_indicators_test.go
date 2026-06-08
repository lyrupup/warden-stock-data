package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/service"
)

func buildKlineBars(closes []float64) []model.StockDailyKline {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]model.StockDailyKline, len(closes))
	for i, c := range closes {
		d := decimal.NewFromFloat(c)
		bars[i] = model.StockDailyKline{
			StockCode: "600000",
			TradeDate: base.AddDate(0, 0, i),
			Open:      d, High: d.Add(decimal.NewFromFloat(0.5)),
			Low: d.Sub(decimal.NewFromFloat(0.5)), Close: d,
			Volume: decimal.NewFromInt(1000),
		}
	}
	return bars
}

// indiRepo 为 nil 时走实时逐 bar 计算：足够数据的指标按 bar 输出，数据不足的指标被跳过且不影响其它指标。
func TestKlineIndicatorsRealtime(t *testing.T) {
	svc := service.NewIndicatorService(nil, nil)
	bars := buildKlineBars([]float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}) // 10 根

	res := svc.KlineIndicators(context.Background(), "600000", "day", "hfq", bars, []string{"ma5", "ma120"})

	// ma5 从第 5 根（下标 4）起可算 → 6 个 bar 有值；ma120 数据不足全程跳过。
	require.Len(t, res, 6)
	for _, r := range res {
		require.Contains(t, r.Values, "ma5")
		require.NotContains(t, r.Values, "ma120")
	}
	// 最后一根 ma5 = (15+16+17+18+19)/5 = 17。
	last := res[len(res)-1]
	require.Equal(t, "2024-01-10", last.TradeDate)
	require.Equal(t, "17.0000", last.Values["ma5"])
}

// 空输入安全返回。
func TestKlineIndicatorsEmpty(t *testing.T) {
	svc := service.NewIndicatorService(nil, nil)
	require.Nil(t, svc.KlineIndicators(context.Background(), "600000", "day", "qfq", nil, []string{"ma5"}))
	bars := buildKlineBars([]float64{10, 11})
	require.Nil(t, svc.KlineIndicators(context.Background(), "600000", "day", "qfq", bars, nil))
}
