package market

import (
	"context"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type IMarketProvider interface {
	Market() string
	Source() string
	Indices(ctx context.Context) ([]model.IndexQuote, error)
	Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error)
	Kline(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error)
	Intraday(ctx context.Context, code string) (model.StockIntraday, error)
	Search(ctx context.Context, kw string) ([]model.Security, error)
	StockList(ctx context.Context) ([]model.Security, error)
	HealthCheck(ctx context.Context) error
}
