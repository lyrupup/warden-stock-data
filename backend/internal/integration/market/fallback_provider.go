package market

import (
	"context"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type FallbackProvider struct {
	providers []IMarketProvider
}

func NewFallbackProvider(providers ...IMarketProvider) *FallbackProvider {
	return &FallbackProvider{providers: providers}
}

func (p *FallbackProvider) Market() string {
	if len(p.providers) > 0 {
		return p.providers[0].Market()
	}
	return "CN"
}

func (p *FallbackProvider) Source() string {
	if len(p.providers) > 0 {
		return p.providers[0].Source()
	}
	return "fallback"
}

func (p *FallbackProvider) callFirst(fn func(IMarketProvider) error) error {
	var last error
	for _, pr := range p.providers {
		if err := fn(pr); err == nil {
			return nil
		} else {
			last = err
		}
	}
	return last
}

func (p *FallbackProvider) Indices(ctx context.Context) ([]model.IndexQuote, error) {
	var last error
	for _, pr := range p.providers {
		v, err := pr.Indices(ctx)
		if err == nil {
			return v, nil
		}
		last = err
	}
	return nil, last
}

func (p *FallbackProvider) Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error) {
	var last error
	for _, pr := range p.providers {
		v, err := pr.Quotes(ctx, codes)
		if err == nil {
			return v, nil
		}
		last = err
	}
	return nil, last
}

func (p *FallbackProvider) Kline(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error) {
	var last error
	for _, pr := range p.providers {
		v, err := pr.Kline(ctx, code, period, adjust)
		if err == nil && len(v) > 0 {
			return v, nil
		}
		last = err
	}
	return nil, last
}

func (p *FallbackProvider) KlineFull(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error) {
	var last error
	for _, pr := range p.providers {
		v, err := pr.KlineFull(ctx, code, period, adjust)
		if err == nil && len(v) > 0 {
			return v, nil
		}
		last = err
	}
	return nil, last
}

func (p *FallbackProvider) Intraday(ctx context.Context, code string) (model.StockIntraday, error) {
	var last error
	for _, pr := range p.providers {
		v, err := pr.Intraday(ctx, code)
		if err == nil && len(v.Points) > 0 {
			return v, nil
		}
		last = err
	}
	return model.StockIntraday{}, last
}

func (p *FallbackProvider) Search(ctx context.Context, kw string) ([]model.Security, error) {
	var last error
	for _, pr := range p.providers {
		v, err := pr.Search(ctx, kw)
		if err == nil {
			return v, nil
		}
		last = err
	}
	return nil, last
}

func (p *FallbackProvider) StockList(ctx context.Context) ([]model.Security, error) {
	var last error
	for _, pr := range p.providers {
		v, err := pr.StockList(ctx)
		if err == nil && len(v) > 0 {
			return v, nil
		}
		last = err
	}
	return nil, last
}

func (p *FallbackProvider) HealthCheck(ctx context.Context) error {
	return p.callFirst(func(pr IMarketProvider) error {
		return pr.HealthCheck(ctx)
	})
}

// Close 关闭所有实现了 io.Closer 的下游 provider（如 gotdx 连接池）。
func (p *FallbackProvider) Close() error {
	var last error
	for _, pr := range p.providers {
		if c, ok := pr.(interface{ Close() error }); ok {
			if err := c.Close(); err != nil {
				last = err
			}
		}
	}
	return last
}
