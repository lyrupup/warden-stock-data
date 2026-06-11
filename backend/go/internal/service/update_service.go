package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/integration/quant"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
)

// ErrNoKline 表示该标的当前无可用日 K 数据。
var ErrNoKline = errors.New("无可用日K数据，请先回补日K")

// ErrNoMarketData 表示数据源对该标的无行情（未上市/退市/长期停牌无量价），计入跳过而非失败。
var ErrNoMarketData = errors.New("数据源无行情数据（疑似未上市/停牌无量价）")

// ErrQuantUnavailable 表示 Python quant 服务未配置或不可用。
var ErrQuantUnavailable = errors.New("quant 采集服务不可用")

func untradedFromQuotes(quotes []model.StockQuote) bool {
	if len(quotes) == 0 {
		return true
	}
	return quotes[0].Price.IsZero()
}

type UpdateService struct {
	provider market.IMarketProvider
	quant    quant.IQuantClient
	klineRepo *repository.KlineRepository
	wmRepo    *repository.WatermarkRepository
	calRepo   *repository.CalendarRepository
	secRepo   *repository.SecurityRepository
	market    string
}

func NewUpdateService(
	provider market.IMarketProvider,
	quantClient quant.IQuantClient,
	klineRepo *repository.KlineRepository,
	wmRepo *repository.WatermarkRepository,
	calRepo *repository.CalendarRepository,
	secRepo *repository.SecurityRepository,
) *UpdateService {
	return &UpdateService{
		provider: provider, quant: quantClient, klineRepo: klineRepo, wmRepo: wmRepo,
		calRepo: calRepo, secRepo: secRepo, market: "CN",
	}
}

func (s *UpdateService) isUntraded(ctx context.Context, code string) bool {
	if s.provider == nil {
		return false
	}
	quotes, err := s.provider.Quotes(ctx, []string{code})
	if err != nil {
		return false
	}
	return untradedFromQuotes(quotes)
}

// IncrementalOne 增量回补单只日 K（经 Python baostock 采集），并推进水位。
func (s *UpdateService) IncrementalOne(ctx context.Context, code string, marketLatest *time.Time) error {
	if s.quant == nil {
		return ErrQuantUnavailable
	}
	wm, err := s.wmRepo.Get(ctx, s.market, code)
	if err != nil {
		return err
	}
	from := ""
	if wm.LastTradeDate != nil {
		from = wm.LastTradeDate.Format("2006-01-02")
	}
	return s.collectOne(ctx, code, "incremental", from, marketLatest)
}

// SyncKlineFull 全量回补单只日 K（经 Python baostock）。
func (s *UpdateService) SyncKlineFull(ctx context.Context, code string) error {
	if s.quant == nil {
		return ErrQuantUnavailable
	}
	return s.collectOne(ctx, code, "full", "", nil)
}

func (s *UpdateService) collectOne(ctx context.Context, code, mode, from string, marketLatest *time.Time) error {
	resp, err := s.quant.CollectKline(ctx, quant.CollectKlineRequest{
		Codes:    []string{code},
		Mode:     mode,
		FromDate: from,
		Market:   s.market,
	})
	if err != nil {
		return err
	}
	if len(resp.Results) == 0 {
		return fmt.Errorf("quant empty result for %s", code)
	}
	return s.applyResult(ctx, code, resp.Results[0], marketLatest)
}

// applyResult 处理 quant 返回的单只结果：ok 推进水位/交易日历；skipped/failed 归类为错误。
func (s *UpdateService) applyResult(ctx context.Context, code string, r quant.CollectResult, marketLatest *time.Time) error {
	switch r.Status {
	case "ok":
		return s.afterCollectOK(ctx, code, r.LatestTradeDate, marketLatest)
	case "skipped":
		if s.isUntraded(ctx, code) {
			return ErrNoMarketData
		}
		return ErrNoKline
	default:
		if r.Reason != "" {
			return errors.New(r.Reason)
		}
		return fmt.Errorf("collect failed for %s", code)
	}
}

// CollectKlineBatch 批量回补一组代码的日 K：一次 HTTP 把整组交给 Python（baostock 内部串行处理）。
//
// mode: "full"（忽略水位，从 BACKFILL_START_DATE 起）/ "incremental"（按各自水位起点，自动按 from 分组合批）。
// fromOverride/toOverride：显式回补日期区间（YYYY-MM-DD，留空表示不限）。一旦 fromOverride 非空，
// 所有代码统一以该起点合批（忽略各自水位），用于「按指定区间回补 / 拉取最新一个交易日」。
//
// 返回每只代码的处理结果（nil=成功；ErrNoMarketData=跳过；其它=失败）；
// 外层 error 为整批传输级失败（如 context 取消 / 网络错误），调用方据此整批归类或终止。
func (s *UpdateService) CollectKlineBatch(ctx context.Context, codes []string, mode, fromOverride, toOverride string, marketLatest *time.Time) (map[string]error, error) {
	if s.quant == nil {
		return nil, ErrQuantUnavailable
	}
	out := make(map[string]error, len(codes))
	if len(codes) == 0 {
		return out, nil
	}
	var wmMap map[string]time.Time
	if mode == "incremental" && fromOverride == "" && s.wmRepo != nil {
		wmMap, _ = s.wmRepo.MapByMarket(ctx, s.market)
	}
	for from, group := range s.groupByFrom(codes, mode, fromOverride, wmMap) {
		resp, err := s.quant.CollectKline(ctx, quant.CollectKlineRequest{
			Codes:    group,
			Mode:     mode,
			FromDate: from,
			ToDate:   toOverride,
			Market:   s.market,
		})
		if err != nil {
			return out, err
		}
		byCode := make(map[string]quant.CollectResult, len(resp.Results))
		for _, r := range resp.Results {
			byCode[r.Code] = r
		}
		for _, c := range group {
			r, ok := byCode[c]
			if !ok {
				out[c] = fmt.Errorf("quant empty result for %s", c)
				continue
			}
			out[c] = s.applyResult(ctx, c, r, marketLatest)
		}
	}
	return out, nil
}

// groupByFrom 把代码按起点分组以便合批：
//   - fromOverride 非空：全部代码同组、统一用该起点（显式区间回补）；
//   - full 模式：全部代码同组、起点留空（Python 用 BACKFILL_START_DATE）；
//   - incremental：按各自水位日分组（稳态下绝大多数水位相同，通常只 1~2 组，合批高效）。
func (s *UpdateService) groupByFrom(codes []string, mode, fromOverride string, wmMap map[string]time.Time) map[string][]string {
	groups := make(map[string][]string)
	if fromOverride != "" {
		groups[fromOverride] = codes
		return groups
	}
	if mode != "incremental" {
		groups[""] = codes
		return groups
	}
	for _, c := range codes {
		from := ""
		if wmMap != nil {
			if last, ok := wmMap[c]; ok && !last.IsZero() {
				from = last.Format("2006-01-02")
			}
		}
		groups[from] = append(groups[from], c)
	}
	return groups
}

func (s *UpdateService) afterCollectOK(ctx context.Context, code, latestStr string, marketLatest *time.Time) error {
	var latest time.Time
	if latestStr != "" {
		t, err := time.Parse("2006-01-02", latestStr)
		if err == nil {
			latest = t
		}
	}
	if !latest.IsZero() {
		if err := s.calRepo.UpsertInferred(ctx, s.market, []time.Time{latest}); err != nil {
			return err
		}
	}
	wm, err := s.wmRepo.Get(ctx, s.market, code)
	if err != nil {
		return err
	}
	current := wm.LastTradeDate
	if !latest.IsZero() {
		bars := []model.StockDailyKline{{TradeDate: latest}}
		return s.advanceWatermark(ctx, code, bars, current, marketLatest)
	}
	return nil
}

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

func (s *UpdateService) SyncSecurities(ctx context.Context) error {
	if s.quant == nil {
		return ErrQuantUnavailable
	}
	_, err := s.quant.CollectSecurities(ctx, s.market)
	return err
}

// RecentTradingRange 计算「最近 n 个交易日」的日期区间 [from, to]（YYYY-MM-DD 字符串）。
// end 当天为交易日则含当天。优先用交易日历表；无数据时按工作日（跳过周末）回退估算。
// 供周级对齐作业在用户/定时未指定区间时缺省使用。
func (s *UpdateService) RecentTradingRange(ctx context.Context, end time.Time, n int) (string, string) {
	if n <= 0 {
		n = 7
	}
	if s.calRepo != nil {
		if days, err := s.calRepo.RecentTradingDays(ctx, s.market, end, n); err == nil && len(days) > 0 {
			return days[0].Format("2006-01-02"), days[len(days)-1].Format("2006-01-02")
		}
	}
	// 回退：交易日历无数据时，从 end 起向历史取 n 个工作日（跳过周末，不含节假日修正）。
	to := end.Truncate(24 * time.Hour)
	for wd := to.Weekday(); wd == time.Saturday || wd == time.Sunday; wd = to.Weekday() {
		to = to.AddDate(0, 0, -1)
	}
	var earliest time.Time
	count := 0
	for cur := to; count < n; cur = cur.AddDate(0, 0, -1) {
		if wd := cur.Weekday(); wd != time.Saturday && wd != time.Sunday {
			earliest = cur
			count++
		}
	}
	return earliest.Format("2006-01-02"), to.Format("2006-01-02")
}

// SyncCalendar 同步 baostock 官方交易日历到 trading_calendars，返回写入天数。
// fromDate/toDate 留空：起点用 Python 侧 BACKFILL_START_DATE，终点默认到「当年年底」
// （由 Python 侧补默认值，确保拉全当年节假日，而非只到最近一个交易日）。
func (s *UpdateService) SyncCalendar(ctx context.Context, fromDate, toDate string) (int, error) {
	if s.quant == nil {
		return 0, ErrQuantUnavailable
	}
	return s.quant.CollectCalendar(ctx, s.market, fromDate, toDate)
}

func (s *UpdateService) MarketLatestTradeDate(ctx context.Context) (*time.Time, error) {
	if s.klineRepo == nil {
		return nil, nil
	}
	return s.klineRepo.LatestTradeDate(ctx, s.market)
}

func (s *UpdateService) PendingCodes(ctx context.Context, codes []string) ([]string, error) {
	if s.klineRepo == nil || s.wmRepo == nil {
		return codes, nil
	}
	// 基准交易日 = max(库内 K 线最新日, 行情源指数日 K 最新日)。
	// 不用指数快照里的「日历今天」——会与真实成交日错位，导致已同步日仍被判落后或空跑。
	latest := s.marketLatestTradeDate(ctx)
	if latest == nil {
		return codes, nil
	}
	wmMap, err := s.wmRepo.MapByMarket(ctx, s.market)
	if err != nil {
		return codes, nil
	}
	return filterPendingCodes(codes, wmMap, latest), nil
}

// marketLatestTradeDate 取全市场「应有数据」的最新交易日：库内 MAX(trade_date) 与
// 行情源上证指数日 K 最后一根日期的较大值（捕捉库内尚未落库的新一日）。
func (s *UpdateService) marketLatestTradeDate(ctx context.Context) *time.Time {
	var latest *time.Time
	if s.klineRepo != nil {
		if d, err := s.klineRepo.LatestTradeDate(ctx, s.market); err == nil && d != nil {
			latest = d
		}
	}
	if s.provider != nil {
		if d := s.providerIndexLatestKlineDate(ctx); d != nil {
			if latest == nil || d.After(*latest) {
				latest = d
			}
		}
	}
	return latest
}

// providerIndexLatestKlineDate 拉上证指数 000001 日 K 最后一根日期，代表市场真实最新成交日。
func (s *UpdateService) providerIndexLatestKlineDate(ctx context.Context) *time.Time {
	bars, err := s.provider.Kline(ctx, "000001", "day", "qfq")
	if err != nil || len(bars) == 0 {
		return nil
	}
	d := bars[len(bars)-1].TradeDate
	return &d
}

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

func filterFromWatermark(bars []model.StockDailyKline, wm *time.Time) []model.StockDailyKline {
	out := make([]model.StockDailyKline, 0)
	for _, b := range bars {
		if wm == nil || !b.TradeDate.Before(*wm) {
			out = append(out, b)
		}
	}
	return out
}
