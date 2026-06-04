package service

import (
	"context"
	"strings"

	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/pkg/cache"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
)

type QuoteService struct {
	provider   market.IMarketProvider
	quoteRepo  *repository.QuoteRepository
	quoteCache *cache.QuoteCache
	market     string
}

func NewQuoteService(provider market.IMarketProvider, quoteRepo *repository.QuoteRepository, quoteCache *cache.QuoteCache) *QuoteService {
	return &QuoteService{
		provider: provider, quoteRepo: quoteRepo, quoteCache: quoteCache, market: "CN",
	}
}

func (s *QuoteService) Indices(ctx context.Context, marketCode string) ([]model.IndexQuote, error) {
	if marketCode == "" {
		marketCode = s.market
	}
	key := cache.QuoteKey(marketCode, "indices", "all")
	var cached []model.IndexQuote
	if s.quoteCache != nil {
		if ok, _ := s.quoteCache.Get(ctx, key, &cached); ok && len(cached) > 0 {
			return cached, nil
		}
	}
	quotes, err := s.provider.Indices(ctx)
	if err != nil {
		return nil, errcodeBiz{code: errcode.ErrProvider}
	}
	if s.quoteRepo != nil {
		_ = s.quoteRepo.SaveIndexQuotes(ctx, quotes)
	}
	if s.quoteCache != nil {
		_ = s.quoteCache.Set(ctx, key, quotes)
	}
	return quotes, nil
}

func (s *QuoteService) Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error) {
	if len(codes) == 0 {
		return nil, errcodeBiz{code: errcode.ErrParam}
	}
	marketCode := s.market
	key := cache.QuotesKey(marketCode, strings.Join(codes, ","))
	var cached []model.StockQuote
	if s.quoteCache != nil {
		if ok, _ := s.quoteCache.Get(ctx, key, &cached); ok {
			return cached, nil
		}
	}
	quotes, err := s.provider.Quotes(ctx, codes)
	if err != nil {
		if s.quoteRepo != nil {
			stale, dbErr := s.quoteRepo.LatestByCodes(ctx, marketCode, codes)
			if dbErr == nil && len(stale) > 0 {
				for i := range stale {
					stale[i].Stale = true
				}
				return stale, nil
			}
		}
		return nil, errcodeBiz{code: errcode.ErrProvider}
	}
	if s.quoteCache != nil {
		_ = s.quoteCache.Set(ctx, key, quotes)
	}
	return quotes, nil
}

func (s *QuoteService) Quote(ctx context.Context, code string) (*model.StockQuote, error) {
	quotes, err := s.Quotes(ctx, []string{code})
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
