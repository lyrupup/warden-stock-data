package service

import (
	"context"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/repository"
)

type MetaService struct {
	provider  market.IMarketProvider
	indiRepo  *repository.IndicatorRepository
}

func NewMetaService(provider market.IMarketProvider, indiRepo *repository.IndicatorRepository) *MetaService {
	return &MetaService{provider: provider, indiRepo: indiRepo}
}

type Meta struct {
	Markets    []map[string]string      `json:"markets"`
	Indicators []map[string]interface{} `json:"indicators"`
	Freshness  Freshness                `json:"freshness"`
}

type Freshness struct {
	Market         string `json:"market"`
	LastTradeDate  string `json:"last_trade_date"`
	LastScanAt     string `json:"last_scan_at"`
	ProviderSource string `json:"provider_source"`
}

func (s *MetaService) Meta(ctx context.Context) (*Meta, error) {
	f, _ := s.Freshness(ctx, "CN")
	if f == nil {
		f = &Freshness{Market: "CN", ProviderSource: s.provider.Source()}
	}
	return &Meta{
		Markets: []map[string]string{
			{"code": "CN", "name": "A股", "enabled": "true"},
		},
		Indicators: indicator.Catalog(),
		Freshness:  *f,
	}, nil
}

func (s *MetaService) Freshness(ctx context.Context, market string) (*Freshness, error) {
	f := &Freshness{
		Market:         market,
		LastScanAt:     time.Now().Format(time.RFC3339),
		ProviderSource: s.provider.Source(),
	}
	if s.indiRepo != nil {
		if t, err := s.indiRepo.LatestTradeDate(ctx, market); err == nil && t != nil {
			f.LastTradeDate = t.Format("2006-01-02")
		}
	}
	if f.LastTradeDate == "" {
		f.LastTradeDate = time.Now().Format("2006-01-02")
	}
	return f, nil
}
