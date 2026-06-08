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
	wmRepo    *repository.WatermarkRepository
	jobRepo   *repository.JobRepository
}

func NewMetaService(
	provider market.IMarketProvider,
	indiRepo *repository.IndicatorRepository,
	secRepo *repository.SecurityRepository,
	klineRepo *repository.KlineRepository,
	wmRepo *repository.WatermarkRepository,
	jobRepo *repository.JobRepository,
) *MetaService {
	return &MetaService{
		provider: provider, indiRepo: indiRepo, secRepo: secRepo,
		klineRepo: klineRepo, wmRepo: wmRepo, jobRepo: jobRepo,
	}
}

type Meta struct {
	Markets    []map[string]string      `json:"markets"`
	Indicators []map[string]interface{} `json:"indicators"`
	// DefaultSnapshotTypes 为盘后全市场扫描默认落库的指标集合，即可经
	// /open/v1/indicators 批量按交易日（point-in-time）读取的指标，供接入方构建量化回测信号。
	DefaultSnapshotTypes []string  `json:"default_snapshot_types"`
	Freshness            Freshness `json:"freshness"`
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
	// 指标快照（stock_indicator_snapshots）完整性统计，供数据源管理页运维查看。
	IndicatorSnapshotLatestDate   string `json:"indicator_snapshot_latest_date"`
	IndicatorSnapshotEarliestDate string `json:"indicator_snapshot_earliest_date"`
	// IndicatorSnapshotStockCount 为「最新快照交易日」有指标快照的股票数（衡量当日快照覆盖完整性）。
	IndicatorSnapshotStockCount int64 `json:"indicator_snapshot_stock_count"`
	// DefaultSnapshotTypes 为盘后逐日落库（可批量按日回放）的默认指标集合；其余指标走实时计算。
	DefaultSnapshotTypes []string `json:"default_snapshot_types"`
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
		Indicators:           indicator.Catalog(),
		DefaultSnapshotTypes: indicator.DefaultSnapshotTypes,
		Freshness:            *f,
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
	if s.wmRepo != nil {
		if n, err := s.wmRepo.CountByMarket(ctx, market); err == nil {
			f.KlineStockCount = n
		}
	}
	if s.jobRepo != nil {
		if t, err := s.jobRepo.LatestRunAt(ctx); err == nil && t != nil {
			f.LastScanAt = t.Format(time.RFC3339)
		}
	}
	// 指标快照完整性：最新/最早快照交易日 + 最新交易日的快照覆盖股数。
	f.DefaultSnapshotTypes = indicator.DefaultSnapshotTypes
	if s.indiRepo != nil {
		if t, err := s.indiRepo.LatestTradeDate(ctx, market); err == nil && t != nil {
			f.IndicatorSnapshotLatestDate = t.Format("2006-01-02")
			if n, err := s.indiRepo.SnapshotStockCountAt(ctx, market, *t); err == nil {
				f.IndicatorSnapshotStockCount = n
			}
		}
		if t, err := s.indiRepo.EarliestTradeDate(ctx, market); err == nil && t != nil {
			f.IndicatorSnapshotEarliestDate = t.Format("2006-01-02")
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
