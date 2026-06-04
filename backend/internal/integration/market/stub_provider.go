package market

import (
	"context"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type StubProvider struct{}

func NewStubProvider() *StubProvider { return &StubProvider{} }

func (p *StubProvider) Market() string  { return "CN" }
func (p *StubProvider) Source() string  { return "stub" }

func (p *StubProvider) HealthCheck(ctx context.Context) error { return nil }

func (p *StubProvider) Indices(ctx context.Context) ([]model.IndexQuote, error) {
	today := time.Now().Truncate(24 * time.Hour)
	return []model.IndexQuote{
		{
			Market: "CN", IndexCode: "000001", IndexName: "上证指数",
			Price: decimal.NewFromInt(3000), ChangePercent: decimal.NewFromFloat(0.5),
			TradeDate: today, SnapshotAt: time.Now(),
		},
		{
			Market: "CN", IndexCode: "399001", IndexName: "深证成指",
			Price: decimal.NewFromInt(10000), ChangePercent: decimal.NewFromFloat(-0.2),
			TradeDate: today, SnapshotAt: time.Now(),
		},
	}, nil
}

func (p *StubProvider) Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error) {
	today := time.Now().Truncate(24 * time.Hour)
	out := make([]model.StockQuote, 0, len(codes))
	for _, code := range codes {
		out = append(out, model.StockQuote{
			Market: "CN", StockCode: code, StockName: "示例-" + code,
			Price: decimal.NewFromInt(10), PrevClose: decimal.NewFromInt(9),
			ChangePercent: decimal.NewFromFloat(11.11),
			TradeDate: today, SnapshotAt: time.Now(),
		})
	}
	return out, nil
}

func (p *StubProvider) Kline(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error) {
	base := time.Now().AddDate(0, 0, -120).Truncate(24 * time.Hour)
	bars := make([]model.StockDailyKline, 0, 120)
	price := decimal.NewFromInt(10)
	for i := 0; i < 120; i++ {
		d := base.AddDate(0, 0, i)
		price = price.Add(decimal.NewFromFloat(0.01))
		bars = append(bars, model.StockDailyKline{
			Market: "CN", Source: "stub", StockCode: code, TradeDate: d,
			Open: price, High: price, Low: price, Close: price,
			Volume: decimal.NewFromInt(1000000), Adjust: adjust,
		})
	}
	return bars, nil
}

func (p *StubProvider) Search(ctx context.Context, kw string) ([]model.Security, error) {
	all, _ := p.StockList(ctx)
	kw = strings.ToLower(kw)
	out := make([]model.Security, 0)
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Code), kw) || strings.Contains(strings.ToLower(s.Name), kw) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (p *StubProvider) StockList(ctx context.Context) ([]model.Security, error) {
	return []model.Security{
		{Market: "CN", Code: "600000", Name: "浦发银行", Status: 1},
		{Market: "CN", Code: "000001", Name: "平安银行", Status: 1},
		{Market: "CN", Code: "600519", Name: "贵州茅台", Status: 1},
	}, nil
}

func (p *StubProvider) Factory() IMarketProvider { return p }
