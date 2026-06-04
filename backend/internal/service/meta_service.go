package service

import (
	"context"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
)

type MetaService struct {
	quote *QuoteService
}

func NewMetaService(quote *QuoteService) *MetaService {
	return &MetaService{quote: quote}
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
	return &Meta{
		Markets: []map[string]string{
			{"code": "CN", "name": "A股", "enabled": "true"},
		},
		Indicators: indicator.Catalog(),
		Freshness: Freshness{
			Market:         "CN",
			LastTradeDate:  time.Now().Format("2006-01-02"),
			LastScanAt:     time.Now().Format(time.RFC3339),
			ProviderSource: "stub",
		},
	}, nil
}

func (s *MetaService) Freshness(ctx context.Context, market string) (*Freshness, error) {
	meta, err := s.Meta(ctx)
	if err != nil {
		return nil, err
	}
	f := meta.Freshness
	f.Market = market
	return &f, nil
}
