package service

import (
	"context"
	"errors"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/integration/quant"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/pkg/cache"
)

// sourceStatsTTL 为「按 source 聚合」结果缓存时长。该统计需扫描全库（数千万行，约数秒），
// 不宜每次打开页面都重算；数据更新（盘后增量/周级对齐）频率低，缓存较久即可，
// 运维需要最新值时由前端「刷新」按钮强制重算。
const sourceStatsTTL = 30 * time.Minute

type MetaService struct {
	provider  market.IMarketProvider
	quant     quant.IQuantClient
	secRepo   *repository.SecurityRepository
	klineRepo *repository.KlineRepository
	wmRepo    *repository.WatermarkRepository
	jobRepo   *repository.JobRepository
	calRepo   *repository.CalendarRepository
	cache     *cache.QuoteCache
}

func NewMetaService(
	provider market.IMarketProvider,
	quantClient quant.IQuantClient,
	secRepo *repository.SecurityRepository,
	klineRepo *repository.KlineRepository,
	wmRepo *repository.WatermarkRepository,
	jobRepo *repository.JobRepository,
	calRepo *repository.CalendarRepository,
	c *cache.QuoteCache,
) *MetaService {
	return &MetaService{
		provider: provider, quant: quantClient, secRepo: secRepo,
		klineRepo: klineRepo, wmRepo: wmRepo, jobRepo: jobRepo, calRepo: calRepo, cache: c,
	}
}

type Meta struct {
	Markets              []map[string]string        `json:"markets"`
	Indicators           []map[string]interface{}   `json:"indicators"`
	DefaultIndicatorTypes  []string                   `json:"default_indicator_types"`
	Freshness            Freshness                  `json:"freshness"`
}

type Freshness struct {
	Market          string `json:"market"`
	LatestTradeDate string `json:"latest_trade_date"`
	KlineUpdatedTo  string `json:"kline_updated_to"`
	LastScanAt      string `json:"last_scan_at"`
	SecuritiesCount int64  `json:"securities_count"`
	KlineStockCount int64  `json:"kline_stock_count"`
	CalendarDays    int64  `json:"calendar_days"`
	CalendarLatest  string `json:"calendar_latest"`
	ProviderSource  string `json:"provider_source"`
	QuantSource     string `json:"quant_source"`
}

func (s *MetaService) Meta(ctx context.Context) (*Meta, error) {
	f, _ := s.Freshness(ctx, "CN")
	if f == nil {
		f = &Freshness{Market: "CN", ProviderSource: s.provider.Source(), QuantSource: "baostock"}
	}
	indicators, defaultTypes := s.loadCatalog(ctx)
	return &Meta{
		Markets: []map[string]string{
			{"code": "CN", "name": "A股", "enabled": "true"},
		},
		Indicators:            indicators,
		DefaultIndicatorTypes: defaultTypes,
		Freshness:             *f,
	}, nil
}

func (s *MetaService) loadCatalog(ctx context.Context) ([]map[string]interface{}, []string) {
	if s.quant == nil {
		return []map[string]interface{}{}, nil
	}
	cat, err := s.quant.Catalog(ctx)
	if err != nil || cat == nil {
		return []map[string]interface{}{}, nil
	}
	out := make([]map[string]interface{}, 0, len(cat.Indicators))
	for _, item := range cat.Indicators {
		out = append(out, map[string]interface{}{
			"type": item.Type, "name": item.Name, "group": item.Group,
			"value_type": item.ValueType, "params": item.Params, "implemented": item.Implemented,
		})
	}
	return out, cat.DefaultTypes
}

func (s *MetaService) Freshness(ctx context.Context, market string) (*Freshness, error) {
	f := &Freshness{
		Market:         market,
		ProviderSource: s.provider.Source(),
		QuantSource:    "baostock",
	}
	if s.klineRepo != nil {
		if t, err := s.klineRepo.LatestTradeDate(ctx, market); err == nil && t != nil {
			f.KlineUpdatedTo = t.Format("2006-01-02")
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
	if s.calRepo != nil {
		if n, latest, err := s.calRepo.OpenStats(ctx, market); err == nil {
			f.CalendarDays = n
			if latest != nil {
				f.CalendarLatest = latest.Format("2006-01-02")
			}
		}
	}
	return f, nil
}

// ProbeQuant 探测 Python quant 服务健康（baostock 日 K 采集源经其访问）。
func (s *MetaService) ProbeQuant(ctx context.Context) error {
	if s.quant == nil {
		return errors.New("quant client not configured")
	}
	return s.quant.Health(ctx)
}

// SourceStatsResult 为「按 source 聚合」对外结果：统计明细 + 生成时间 + 是否命中缓存，
// 便于前端展示数据时效并区分「实时重算 / 缓存」。
type SourceStatsResult struct {
	Stats       []repository.KlineSourceStat `json:"stats"`
	GeneratedAt time.Time                    `json:"generated_at"`
	Cached      bool                         `json:"cached"`
}

// SourceStats 返回某市场日 K 线按 source（gotdx / baostock）聚合的覆盖情况：
// 每个 source 的行数、覆盖股票数、日期区间，以及股票数较少时（<=100）的代码清单。
//
// 该统计扫描全库代价较高，结果按 sourceStatsTTL 缓存到 Redis：refresh=false 时优先返回缓存，
// refresh=true（前端手动刷新）时强制重算并回写缓存。Redis 不可用时退化为每次直查 DB。
func (s *MetaService) SourceStats(ctx context.Context, market string, refresh bool) (*SourceStatsResult, error) {
	if s.klineRepo == nil {
		return &SourceStatsResult{Stats: []repository.KlineSourceStat{}, GeneratedAt: time.Now()}, nil
	}
	key := cache.SourceStatsKey(market)
	if !refresh && s.cache != nil {
		var cached SourceStatsResult
		if ok, err := s.cache.Get(ctx, key, &cached); err == nil && ok {
			cached.Cached = true
			return &cached, nil
		}
	}
	stats, err := s.klineRepo.SourceStats(ctx, market, 100)
	if err != nil {
		return nil, err
	}
	res := &SourceStatsResult{Stats: stats, GeneratedAt: time.Now()}
	if s.cache != nil {
		_ = s.cache.SetWithTTL(ctx, key, res, sourceStatsTTL)
	}
	return res, nil
}

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
