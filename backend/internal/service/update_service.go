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

// defaultSnapshotTypes 复用指标包统一维护的默认逐日快照指标集合（回测友好），避免重复定义。
var defaultSnapshotTypes = indicator.DefaultSnapshotTypes

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

// IncrementalOne 增量更新单只标的的日 K 线。marketLatest 为全市场最新交易日
// （可为 nil，如首次全量回补时库为空），用于对齐水位、避免停牌/次新股反复无效拉取。
func (s *UpdateService) IncrementalOne(ctx context.Context, code string, marketLatest *time.Time) error {
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
	if len(newBars) > 0 {
		if err := s.klineRepo.UpsertBatch(ctx, newBars); err != nil {
			return err
		}
		dates := make([]time.Time, len(newBars))
		for i, b := range newBars {
			dates[i] = b.TradeDate
		}
		if err := s.calRepo.UpsertInferred(ctx, s.market, dates); err != nil {
			return err
		}
	}
	// 推进水位到「数据源可得最新」与「全市场最新交易日」中的较新者：即便本轮无新
	// K 线（停牌 / 次新股尚未复牌，gotdx 可能返回旧数据甚至空），也将水位对齐到全市场
	// 最新交易日，避免这些个股因永远落后于全市场而被每轮增量反复选中、无效拉取。
	// 数据源报错（err != nil）时已提前返回、不推进，留待下轮重试。
	var target time.Time
	if len(bars) > 0 {
		target = bars[len(bars)-1].TradeDate
	}
	if marketLatest != nil && marketLatest.After(target) {
		target = *marketLatest
	}
	if !target.IsZero() && (wm.LastTradeDate == nil || target.After(*wm.LastTradeDate)) {
		return s.wmRepo.Upsert(ctx, s.market, code, target)
	}
	return nil
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
		types = defaultSnapshotTypes
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
		types = defaultSnapshotTypes
	}
	bars, err := s.klineRepo.List(ctx, s.market, code, "qfq", nil, nil, 0)
	if err != nil || len(bars) == 0 {
		return err
	}
	for i := range bars {
		series := klinesToSeries(bars[:i+1])
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

// MarketLatestTradeDate 返回全市场库内 K 线的最新交易日（库空时为 nil），
// 供增量更新对齐个股水位使用。
func (s *UpdateService) MarketLatestTradeDate(ctx context.Context) (*time.Time, error) {
	if s.klineRepo == nil {
		return nil, nil
	}
	return s.klineRepo.LatestTradeDate(ctx, s.market)
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
