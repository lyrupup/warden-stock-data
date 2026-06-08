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
	// KlineFull 分页拉取数据源全部历史 K 线；API 透传等场景仍用 Kline（最近一页）。
	KlineFull(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error)
	Intraday(ctx context.Context, code string) (model.StockIntraday, error)
	Search(ctx context.Context, kw string) ([]model.Security, error)
	StockList(ctx context.Context) ([]model.Security, error)
	HealthCheck(ctx context.Context) error
}
