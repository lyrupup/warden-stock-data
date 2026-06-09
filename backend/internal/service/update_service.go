package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"github.com/warden-stock/warden-stock-data/internal/indicator"
	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"gorm.io/datatypes"
	"github.com/warden-stock/warden-stock-data/internal/repository"
)

// defaultSnapshotTypes 复用指标包统一维护的默认逐日快照指标集合（回测友好），避免重复定义。
var defaultSnapshotTypes = indicator.DefaultSnapshotTypes

// ErrNoKline 表示该标的当前无可用日 K 数据，指标无法计算。指标类作业据此把该标的
// 计入「未成功/未完整」，提示运维先补 K 线再单独重跑。
var ErrNoKline = errors.New("无可用日K数据，请先回补日K")

// ErrNoMarketData 表示数据源对该标的「既无历史日 K，又无有效实时快照（量价全 0）」，
// 属于「已登记代码但暂无任何成交行情」的正常状态（典型为新股尚未上市交易 / 退市清空 /
// 长期停牌无量价）。这类标的并非拉取失败，作业据此把它计入「跳过(skipped)」而非「失败」。
var ErrNoMarketData = errors.New("数据源无行情数据（疑似未上市/停牌无量价）")

// untradedFromQuotes 依据实时快照判定标的是否「无行情」：无任何快照行，或首行现价为 0，
// 即视为暂无成交行情。纯函数，便于单测。
func untradedFromQuotes(quotes []model.StockQuote) bool {
	if len(quotes) == 0 {
		return true
	}
	return quotes[0].Price.IsZero()
}

// isUntraded 在日 K 为空时二次确认：再取一次实时快照，若同样无有效量价，则判定为「无行情/
// 未上市」。探测本身出错时返回 false（不轻易判定无行情，回退为普通失败更安全，避免误吞）。
func (s *UpdateService) isUntraded(ctx context.Context, code string) bool {
	quotes, err := s.provider.Quotes(ctx, []string{code})
	if err != nil {
		return false
	}
	return untradedFromQuotes(quotes)
}

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

// IncrementalOne 增量更新单只标的的日 K 线：拉取数据源全量日 K，但仅落库「水位日及之后」
// 的部分（含水位日本身，以覆盖更新最新一日已有数据），从而既补齐缺口又满足「最新一日覆盖」。
// marketLatest 为全市场最新交易日（可为 nil，如库为空），用于对齐水位、避免停牌/次新股反复无效拉取。
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
	if len(bars) == 0 {
		// 数据源无任何日 K：先把水位对齐到全市场最新交易日，避免后续增量反复重选该标的；
		// 再二次探测实时快照，若同样无量价则按「无行情/未上市」上报（计入跳过而非失败）。
		if err := s.advanceWatermark(ctx, code, bars, wm.LastTradeDate, marketLatest); err != nil {
			return err
		}
		if s.isUntraded(ctx, code) {
			return ErrNoMarketData
		}
		return nil
	}
	newBars := filterFromWatermark(bars, wm.LastTradeDate)
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
	return s.advanceWatermark(ctx, code, bars, wm.LastTradeDate, marketLatest)
}

// SyncKlineFull 全量回补单只标的的日 K 线：拉取数据源全部历史日 K 并整体覆盖落库
// （已存在日期一并 UPSERT 覆盖更新），随后推进水位到最新交易日。用于「全量日K数据回补」作业。
func (s *UpdateService) SyncKlineFull(ctx context.Context, code string) error {
	bars, err := s.provider.KlineFull(ctx, code, "day", "qfq")
	if err != nil {
		return err
	}
	if len(bars) == 0 {
		// 数据源无任何日 K：二次探测实时快照，量价全 0 则判为「无行情/未上市」（计入跳过），
		// 否则按「无可用日 K」失败处理。
		if s.isUntraded(ctx, code) {
			return ErrNoMarketData
		}
		return ErrNoKline
	}
	if err := s.klineRepo.UpsertBatch(ctx, bars); err != nil {
		return err
	}
	// 仅按最新一日补登交易日历（避免全量逐日在多标的间重复写入数百万条冗余日历）。
	if err := s.calRepo.UpsertInferred(ctx, s.market, []time.Time{bars[len(bars)-1].TradeDate}); err != nil {
		return err
	}
	return s.advanceWatermark(ctx, code, bars, nil, nil)
}

// advanceWatermark 推进水位到「数据源可得最新」与「全市场最新交易日」中的较新者：即便本轮无新
// K 线（停牌 / 次新股尚未复牌，gotdx 可能返回旧数据甚至空），也将水位对齐到全市场最新交易日，
// 避免这些个股因永远落后于全市场而被每轮增量反复选中、无效拉取。
func (s *UpdateService) advanceWatermark(ctx context.Context, code string, bars []model.StockDailyKline, current, marketLatest *time.Time) error {
	var target time.Time
	if len(bars) > 0 {
		target = bars[len(bars)-1].TradeDate
	}
	if marketLatest != nil && marketLatest.After(target) {
		target = *marketLatest
	}
	if !target.IsZero() && (current == nil || target.After(*current)) {
		return s.wmRepo.Upsert(ctx, s.market, code, target)
	}
	return nil
}

// filterFromWatermark 保留水位日及之后的 K 线（wm 为 nil 时返回全部）。包含水位日本身，
// 以便增量作业覆盖更新最新一日的已有数据。
func filterFromWatermark(bars []model.StockDailyKline, wm *time.Time) []model.StockDailyKline {
	out := make([]model.StockDailyKline, 0)
	for _, b := range bars {
		if wm == nil || !b.TradeDate.Before(*wm) {
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
	if err != nil {
		return err
	}
	if len(bars) == 0 {
		if s.isUntraded(ctx, code) {
			return ErrNoMarketData
		}
		return ErrNoKline
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
	if err := ctx.Err(); err != nil {
		return err
	}
	bars, err := s.klineRepo.List(ctx, s.market, code, "qfq", nil, nil, 0)
	if err != nil {
		return err
	}
	if len(bars) == 0 {
		if s.isUntraded(ctx, code) {
			return ErrNoMarketData
		}
		return ErrNoKline
	}
	series := klinesToSeries(bars)

	const batchSize = 500
	pending := make([]model.StockIndicatorSnapshot, 0, batchSize)
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		if err := s.indiSvc.WriteSnapshotBatch(ctx, code, pending); err != nil {
			return err
		}
		pending = pending[:0]
		return nil
	}

	err = indicator.StreamSnapshotSeries(ctx, series, types, func(i int, vals map[string]decimal.Decimal) error {
		if len(vals) == 0 {
			return nil
		}
		b, err := indicator.SnapshotValuesJSON(vals)
		if err != nil {
			return err
		}
		pending = append(pending, model.StockIndicatorSnapshot{
			TradeDate: bars[i].TradeDate,
			Values:    datatypes.JSON(b),
		})
		if len(pending) >= batchSize {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	return flush()
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
