package service_test

import (
	"context"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

// fakeProvider 为单元测试提供确定性行情数据，替代已移除的运行时 stub 数据源。
// 仅存在于测试构建中，不参与生产逻辑。
type fakeProvider struct{}

func newFakeProvider() *fakeProvider { return &fakeProvider{} }

func (p *fakeProvider) Market() string                       { return "CN" }
func (p *fakeProvider) Source() string                       { return "fake" }
func (p *fakeProvider) HealthCheck(ctx context.Context) error { return nil }

func (p *fakeProvider) Indices(ctx context.Context) ([]model.IndexQuote, error) {
	today := time.Now().Truncate(24 * time.Hour)
	return []model.IndexQuote{
		{
			Market: "CN", IndexCode: "000001", IndexName: "上证指数",
			Price: decimal.NewFromInt(3000), ChangePercent: decimal.NewFromFloat(0.5),
			TradeDate: today, SnapshotAt: time.Now(),
		},
	}, nil
}

func (p *fakeProvider) Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error) {
	today := time.Now().Truncate(24 * time.Hour)
	out := make([]model.StockQuote, 0, len(codes))
	for _, code := range codes {
		out = append(out, model.StockQuote{
			Market: "CN", StockCode: code, StockName: "示例-" + code,
			Price: decimal.NewFromInt(10), PrevClose: decimal.NewFromInt(9),
			ChangePercent: decimal.NewFromFloat(11.11),
			TradeDate:     today, SnapshotAt: time.Now(),
		})
	}
	return out, nil
}

func (p *fakeProvider) KlineFull(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error) {
	return p.Kline(ctx, code, period, adjust)
}

func (p *fakeProvider) Kline(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error) {
	base := time.Now().AddDate(0, 0, -120).Truncate(24 * time.Hour)
	bars := make([]model.StockDailyKline, 0, 120)
	price := decimal.NewFromInt(10)
	for i := 0; i < 120; i++ {
		d := base.AddDate(0, 0, i)
		price = price.Add(decimal.NewFromFloat(0.01))
		bars = append(bars, model.StockDailyKline{
			Market: "CN", Source: "fake", StockCode: code, TradeDate: d,
			Open: price, High: price, Low: price, Close: price,
			Volume: decimal.NewFromInt(1000000), Adjust: adjust,
		})
	}
	return bars, nil
}

func (p *fakeProvider) Intraday(ctx context.Context, code string) (model.StockIntraday, error) {
	today := time.Now()
	loc := today.Location()
	open := time.Date(today.Year(), today.Month(), today.Day(), 9, 30, 0, 0, loc)
	points := make([]model.IntradayPoint, 0, 4)
	for i := 0; i < 4; i++ {
		price := decimal.NewFromFloat(10 + float64(i)*0.1)
		points = append(points, model.IntradayPoint{
			Time:     open.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			Price:    price,
			AvgPrice: price,
			Volume:   decimal.NewFromInt(1000),
		})
	}
	return model.StockIntraday{
		Market: "CN", StockCode: code, TradeDate: today.Format("2006-01-02"),
		PreClose: decimal.NewFromInt(9), Points: points,
	}, nil
}

func (p *fakeProvider) Search(ctx context.Context, kw string) ([]model.Security, error) {
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

func (p *fakeProvider) StockList(ctx context.Context) ([]model.Security, error) {
	return []model.Security{
		{Market: "CN", Code: "600000", Name: "浦发银行", Status: 1},
		{Market: "CN", Code: "000001", Name: "平安银行", Status: 1},
		{Market: "CN", Code: "600519", Name: "贵州茅台", Status: 1},
	}, nil
}
