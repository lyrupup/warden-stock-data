package service

import (
	"context"

	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
)

type QuoteService struct {
	provider market.IMarketProvider
}

func NewQuoteService(provider market.IMarketProvider) *QuoteService {
	return &QuoteService{provider: provider}
}

func (s *QuoteService) Indices(ctx context.Context, marketCode string) ([]model.IndexQuote, error) {
	return s.provider.Indices(ctx)
}

func (s *QuoteService) Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error) {
	if len(codes) == 0 {
		return nil, errcodeBiz{code: errcode.ErrParam}
	}
	return s.provider.Quotes(ctx, codes)
}

func (s *QuoteService) Quote(ctx context.Context, code string) (*model.StockQuote, error) {
	quotes, err := s.provider.Quotes(ctx, []string{code})
	if err != nil {
		return nil, err
	}
	if len(quotes) == 0 {
		return nil, errcodeBiz{code: errcode.ErrStockNotFound}
	}
	return &quotes[0], nil
}

func (s *QuoteService) Search(ctx context.Context, kw string) ([]model.Security, error) {
	if kw == "" {
		return nil, errcodeBiz{code: errcode.ErrParam}
	}
	return s.provider.Search(ctx, kw)
}

func (s *QuoteService) Securities(ctx context.Context) ([]model.Security, error) {
	return s.provider.StockList(ctx)
}

func (s *QuoteService) HealthCheck(ctx context.Context) error {
	return s.provider.HealthCheck(ctx)
}
