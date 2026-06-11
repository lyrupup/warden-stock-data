package service_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/service"
)

// secRepo 为 nil 时（如未接库），Search 应回源行情提供方并返回结果。
func TestQuoteSearchProviderFallback(t *testing.T) {
	svc := service.NewQuoteService(newFakeProvider(), nil, nil, nil)
	res, err := svc.Search(context.Background(), "600")
	require.NoError(t, err)
	require.NotEmpty(t, res)
	for _, r := range res {
		require.Equal(t, "CN", r.Market)
	}
}

func TestEnrichQuoteNames(t *testing.T) {
	quotes := []model.StockQuote{
		{StockCode: "600519", StockName: "600519"},
		{StockCode: "000001", StockName: "000001"},
	}
	service.EnrichQuoteNamesForTest(quotes, map[string]string{
		"600519": "贵州茅台",
	})
	require.Equal(t, "贵州茅台", quotes[0].StockName)
	require.Equal(t, "000001", quotes[1].StockName)
}

func TestQuoteSearchEmptyKeyword(t *testing.T) {
	svc := service.NewQuoteService(newFakeProvider(), nil, nil, nil)
	_, err := svc.Search(context.Background(), "   ")
	require.Error(t, err)
}

func TestIntradayHappyPath(t *testing.T) {
	svc := service.NewQuoteService(newFakeProvider(), nil, nil, nil)
	res, err := svc.Intraday(context.Background(), "600519")
	require.NoError(t, err)
	require.Equal(t, "600519", res.StockCode)
	require.NotEmpty(t, res.Points)
	require.True(t, res.PreClose.GreaterThan(decimal.Zero))
	for _, p := range res.Points {
		require.NotEmpty(t, p.Time)
	}
}

func TestIntradayEmptyCode(t *testing.T) {
	svc := service.NewQuoteService(newFakeProvider(), nil, nil, nil)
	_, err := svc.Intraday(context.Background(), "  ")
	require.Error(t, err)
}
