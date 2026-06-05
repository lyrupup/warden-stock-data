package service

import (
	"context"
	"sort"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
)

var defaultMATypes = []string{"ma5", "ma10", "ma20", "ma30", "ma60"}

type UpdateService struct {
	provider  market.IMarketProvider
	klineRepo *repository.KlineRepository
	wmRepo    *repository.WatermarkRepository
	calRepo   *repository.CalendarRepository
	secRepo   *repository.SecurityRepository
	indiSvc   *IndicatorService
	market    string
}

func NewUpdateService(
	provider market.IMarketProvider,
	klineRepo *repository.KlineRepository,
	wmRepo *repository.WatermarkRepository,
	calRepo *repository.CalendarRepository,
	secRepo *repository.SecurityRepository,
	indiSvc *IndicatorService,
) *UpdateService {
	return &UpdateService{
		provider: provider, klineRepo: klineRepo, wmRepo: wmRepo,
		calRepo: calRepo, secRepo: secRepo, indiSvc: indiSvc, market: "CN",
	}
}

func (s *UpdateService) IncrementalOne(ctx context.Context, code string) error {
	wm, err := s.wmRepo.Get(ctx, s.market, code)
	if err != nil {
		return err
	}
	bars, err := s.provider.Kline(ctx, code, "day", "qfq")
	if err != nil {
		return err
	}
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].TradeDate.Before(bars[j].TradeDate)
	})
	newBars := filterAfterWatermark(bars, wm.LastTradeDate)
	if len(newBars) == 0 {
		return nil
	}
	if err := s.klineRepo.UpsertBatch(ctx, newBars); err != nil {
		return err
	}
	last := newBars[len(newBars)-1].TradeDate
	if err := s.wmRepo.Upsert(ctx, s.market, code, last); err != nil {
		return err
	}
	dates := make([]time.Time, len(newBars))
	for i, b := range newBars {
		dates[i] = b.TradeDate
	}
	return s.calRepo.UpsertInferred(ctx, s.market, dates)
}

func filterAfterWatermark(bars []model.StockDailyKline, wm *time.Time) []model.StockDailyKline {
	out := make([]model.StockDailyKline, 0)
	for _, b := range bars {
		if wm == nil || b.TradeDate.After(*wm) {
			out = append(out, b)
		}
	}
	return out
}

func (s *UpdateService) ScanIndicators(ctx context.Context, code string, types []string) error {
	if len(types) == 0 {
		types = defaultMATypes
	}
	bars, err := s.loadBars(ctx, code, 120)
	if err != nil || len(bars) == 0 {
		return err
	}
	series := klinesToSeries(bars)
	vals, err := indicator.ComputeAll(series, types)
	if err != nil {
		return err
	}
	return s.indiSvc.WriteSnapshot(ctx, code, bars[len(bars)-1].TradeDate, vals)
}

func (s *UpdateService) BackfillIndicators(ctx context.Context, code string, types []string) error {
	if len(types) == 0 {
		types = defaultMATypes
	}
	bars, err := s.klineRepo.List(ctx, s.market, code, "qfq", nil, nil, 0)
	if err != nil || len(bars) == 0 {
		return err
	}
	for i := range bars {
		series := klinesToSeries(bars[: i+1])
		vals, err := indicator.ComputeAll(series, types)
		if err != nil {
			continue
		}
		_ = s.indiSvc.WriteSnapshot(ctx, code, bars[i].TradeDate, vals)
	}
	return nil
}

func (s *UpdateService) loadBars(ctx context.Context, code string, limit int) ([]model.StockDailyKline, error) {
	bars, err := s.klineRepo.List(ctx, s.market, code, "qfq", nil, nil, limit)
	if err == nil && len(bars) > 0 {
		return bars, nil
	}
	return s.provider.Kline(ctx, code, "day", "qfq")
}

func (s *UpdateService) SyncSecurities(ctx context.Context) error {
	list, err := s.provider.StockList(ctx)
	if err != nil {
		return err
	}
	return s.secRepo.UpsertBatch(ctx, list)
}

// PendingCodes 在给定代码集合中筛出「需要增量更新」的标的：
// 以全市场最新交易日（库内 K 线 MAX(trade_date)）为基准，
// 保留无水位（新股/从未拉取）或水位早于最新交易日的股票。
// 库为空（latest 为 nil）时视为首次全量，返回全部代码。
func (s *UpdateService) PendingCodes(ctx context.Context, codes []string) ([]string, error) {
	if s.klineRepo == nil || s.wmRepo == nil {
		return codes, nil
	}
	latest, err := s.klineRepo.LatestTradeDate(ctx, s.market)
	if err != nil || latest == nil {
		return codes, nil
	}
	wmMap, err := s.wmRepo.MapByMarket(ctx, s.market)
	if err != nil {
		return codes, nil
	}
	return filterPendingCodes(codes, wmMap, latest), nil
}

// filterPendingCodes 纯函数：保留无水位或水位早于 latest 的代码。
func filterPendingCodes(codes []string, wmMap map[string]time.Time, latest *time.Time) []string {
	if latest == nil {
		return codes
	}
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		last, ok := wmMap[c]
		if !ok || last.Before(*latest) {
			out = append(out, c)
		}
	}
	return out
}

// LatestIndicatorDate returns max snapshot trade date for freshness.
func (s *UpdateService) LatestIndicatorDate(ctx context.Context, indiRepo *repository.IndicatorRepository) (*time.Time, error) {
	if indiRepo == nil {
		return nil, nil
	}
	return indiRepo.LatestTradeDate(ctx, s.market)
}
