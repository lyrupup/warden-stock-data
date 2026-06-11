package service

import (
	"context"
	"sort"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

// SecurityMap 读取在市证券，返回 code -> Security（含 IsST/Board/ListDate），
// 供 gotdx 增量采集补全涨跌停/ST/新股首日判定。空库时返回空 map。
func (s *UpdateService) SecurityMap(ctx context.Context) (map[string]model.Security, error) {
	if s.secRepo == nil {
		return map[string]model.Security{}, nil
	}
	list, err := s.secRepo.List(ctx, s.market)
	if err != nil {
		return nil, err
	}
	m := make(map[string]model.Security, len(list))
	for _, sec := range list {
		m[sec.Code] = sec
	}
	return m, nil
}

// IncrementalKlineGotdx 经 gotdx（Go 原生并发连接池）增量采集单只日 K 并落库。
//
// 相比 baostock 串行，gotdx 走连接池可真正并发，全市场当日增量分钟级完成。
// gotdx 日 K 自带昨收/换手/涨跌幅；本函数补全 涨跌停（按板块/ST 现算）、ST（取证券表）、
// 停牌（量=0 记 0）后以 adjust="qfq" UPSERT，续接既有序列，并推进水位、补登交易日历。
//
// 取数范围：gotdx 单页约 600 根（≈2.4 年），按「水位日及之后」（含水位日覆盖最新一日）或
// 显式区间 [from,to] 过滤。更早的历史缺口请用 full（baostock）回补。
func (s *UpdateService) IncrementalKlineGotdx(
	ctx context.Context, code string, sec *model.Security,
	fromOverride, toOverride string, marketLatest *time.Time,
) error {
	bars, err := s.provider.Kline(ctx, code, "day", "qfq")
	if err != nil {
		return err
	}
	if len(bars) == 0 {
		if s.isUntraded(ctx, code) {
			return ErrNoMarketData
		}
		return ErrNoKline
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].TradeDate.Before(bars[j].TradeDate) })

	from := s.incrementalFrom(ctx, code, fromOverride)
	to := parseDatePtr(toOverride)

	var isST bool
	var listDate *time.Time
	if sec != nil {
		isST = sec.IsST
		listDate = sec.ListDate
	}

	out := make([]model.StockDailyKline, 0, len(bars))
	for _, b := range bars {
		if from != nil && b.TradeDate.Before(*from) { // 含水位日：>= from（覆盖最新一日）
			continue
		}
		if to != nil && b.TradeDate.After(*to) {
			continue
		}
		b.Source = "gotdx"
		b.Adjust = "qfq"
		b.IsST = isST
		b.TradeStatus = 1
		if b.Volume.IsZero() {
			b.TradeStatus = 0
		}
		firstDay := listDate != nil && !b.TradeDate.After(*listDate)
		if up, down, ok := computeLimitPrices(code, b.PreClose, isST, firstDay); ok {
			b.LimitUp = up
			b.LimitDown = down
		}
		out = append(out, b)
	}

	wm, _ := s.wmRepo.Get(ctx, s.market, code)
	current := wm.LastTradeDate
	if len(out) == 0 {
		// 本轮无新 K（停牌/次新），仍把水位推到全市场最新交易日，避免每轮反复无效拉取。
		return s.advanceWatermark(ctx, code, nil, current, marketLatest)
	}
	if err := s.klineRepo.UpsertBatch(ctx, out); err != nil {
		return err
	}
	latest := out[len(out)-1].TradeDate
	if err := s.calRepo.UpsertInferred(ctx, s.market, []time.Time{latest}); err != nil {
		return err
	}
	return s.advanceWatermark(ctx, code, out, current, marketLatest)
}

// incrementalFrom 计算增量起点：显式 fromOverride 优先，否则取该标的水位日（含水位日覆盖）。
func (s *UpdateService) incrementalFrom(ctx context.Context, code, fromOverride string) *time.Time {
	if fromOverride != "" {
		return parseDatePtr(fromOverride)
	}
	if s.wmRepo == nil {
		return nil
	}
	wm, err := s.wmRepo.Get(ctx, s.market, code)
	if err != nil {
		return nil
	}
	return wm.LastTradeDate
}

func parseDatePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}
