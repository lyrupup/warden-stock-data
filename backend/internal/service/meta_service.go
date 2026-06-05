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
	secRepo   *repository.SecurityRepository
	klineRepo *repository.KlineRepository
	jobRepo   *repository.JobRepository
}

func NewMetaService(
	provider market.IMarketProvider,
	indiRepo *repository.IndicatorRepository,
	secRepo *repository.SecurityRepository,
	klineRepo *repository.KlineRepository,
	jobRepo *repository.JobRepository,
) *MetaService {
	return &MetaService{
		provider: provider, indiRepo: indiRepo, secRepo: secRepo,
		klineRepo: klineRepo, jobRepo: jobRepo,
	}
}

type Meta struct {
	Markets    []map[string]string      `json:"markets"`
	Indicators []map[string]interface{} `json:"indicators"`
	Freshness  Freshness                `json:"freshness"`
}

type Freshness struct {
	Market          string `json:"market"`
	LatestTradeDate string `json:"latest_trade_date"`
	KlineUpdatedTo  string `json:"kline_updated_to"`
	LastScanAt      string `json:"last_scan_at"`
	SecuritiesCount int64  `json:"securities_count"`
	// KlineStockCount 为库中已落库行情数据的股票数量，配合 SecuritiesCount 衡量拉取覆盖度。
	KlineStockCount int64  `json:"kline_stock_count"`
	ProviderSource  string `json:"provider_source"`
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
		ProviderSource: s.provider.Source(),
	}
	if s.klineRepo != nil {
		if t, err := s.klineRepo.LatestTradeDate(ctx, market); err == nil && t != nil {
			f.KlineUpdatedTo = t.Format("2006-01-02")
			f.LatestTradeDate = t.Format("2006-01-02")
		}
	}
	if f.LatestTradeDate == "" && s.indiRepo != nil {
		if t, err := s.indiRepo.LatestTradeDate(ctx, market); err == nil && t != nil {
			f.LatestTradeDate = t.Format("2006-01-02")
		}
	}
	if f.LatestTradeDate == "" {
		if t := s.probeLatestKlineDate(ctx); t != "" {
			f.LatestTradeDate = t
			if f.KlineUpdatedTo == "" {
				f.KlineUpdatedTo = t
			}
		}
	}
	if s.secRepo != nil {
		if n, err := s.secRepo.Count(ctx, market); err == nil {
			f.SecuritiesCount = n
		}
	}
	if f.SecuritiesCount == 0 {
		if n := s.probeSecuritiesCount(ctx); n > 0 {
			f.SecuritiesCount = n
		}
	}
	if s.klineRepo != nil {
		if n, err := s.klineRepo.DistinctStockCount(ctx, market); err == nil {
			f.KlineStockCount = n
		}
	}
	if s.jobRepo != nil {
		if t, err := s.jobRepo.LatestRunAt(ctx); err == nil && t != nil {
			f.LastScanAt = t.Format(time.RFC3339)
		}
	}
	return f, nil
}

// probeLatestKlineDate 在库内尚无 K 线时，用基准股向行情源探测最新交易日。
func (s *MetaService) probeLatestKlineDate(ctx context.Context) string {
	bars, err := s.provider.Kline(ctx, "600519", "day", "qfq")
	if err != nil || len(bars) == 0 {
		return ""
	}
	return bars[len(bars)-1].TradeDate.Format("2006-01-02")
}

func (s *MetaService) probeSecuritiesCount(ctx context.Context) int64 {
	list, err := s.provider.StockList(ctx)
	if err != nil {
		return 0
	}
	return int64(len(list))
}
