package service

import (
	"context"
	"sort"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
)

type KlineService struct {
	provider  market.IMarketProvider
	klineRepo *repository.KlineRepository
	market    string
}

func NewKlineService(provider market.IMarketProvider, klineRepo *repository.KlineRepository) *KlineService {
	return &KlineService{provider: provider, klineRepo: klineRepo, market: "CN"}
}

type KlineQuery struct {
	Code   string
	Period string
	Adjust string
	Limit  int
	// Offset 为自最新交易日向历史方向跳过的根数，配合 Limit 做分页加载更早 K 线。
	Offset int
	From   *time.Time
	To     *time.Time
	Market string
}

func (s *KlineService) Kline(ctx context.Context, q KlineQuery) ([]model.StockDailyKline, error) {
	if q.Code == "" {
		return nil, errcodeBiz{code: errcode.ErrParam}
	}
	marketCode := q.Market
	if marketCode == "" {
		marketCode = s.market
	}
	adjust := q.Adjust
	if adjust == "" {
		adjust = "qfq"
	}
	if q.Period == "" {
		q.Period = "day"
	}

	if s.klineRepo != nil && q.Period == "day" {
		bars, err := s.klineRepo.List(ctx, marketCode, q.Code, adjust, q.From, q.To, q.Limit)
		if err == nil && len(bars) > 0 {
			return bars, nil
		}
	}

	bars, err := s.provider.Kline(ctx, q.Code, q.Period, adjust)
	if err != nil {
		return nil, err
	}
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].TradeDate.Before(bars[j].TradeDate)
	})
	if q.From != nil || q.To != nil {
		filtered := make([]model.StockDailyKline, 0, len(bars))
		for _, b := range bars {
			if q.From != nil && b.TradeDate.Before(*q.From) {
				continue
			}
			if q.To != nil && b.TradeDate.After(*q.To) {
				continue
			}
			filtered = append(filtered, b)
		}
		bars = filtered
	} else if q.Limit > 0 && len(bars) > q.Limit {
		bars = bars[len(bars)-q.Limit:]
	}
	return bars, nil
}

// KlinePage 分页拉取 K 线：跳过最近 Offset 根、取 Limit 根历史窗口，并返回 hasMore
// 表示该窗口左侧是否还有更早数据。日 K 优先走 DB 分页；其余周期或未落库时回源后在
// 内存按窗口切片。供前端左滑「加载更多」分页用，避免每次重复返回全量数据。
func (s *KlineService) KlinePage(ctx context.Context, q KlineQuery) ([]model.StockDailyKline, bool, error) {
	if q.Code == "" {
		return nil, false, errcodeBiz{code: errcode.ErrParam}
	}
	marketCode := q.Market
	if marketCode == "" {
		marketCode = s.market
	}
	adjust := q.Adjust
	if adjust == "" {
		adjust = "qfq"
	}
	if q.Period == "" {
		q.Period = "day"
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 120
	}

	if s.klineRepo != nil && q.Period == "day" {
		bars, hasMore, err := s.klineRepo.ListPage(ctx, marketCode, q.Code, adjust, q.Offset, limit)
		// Offset>0 表示已在分页翻历史，即便回空也不回源（避免越界翻页误触发回源全量）。
		if err == nil && (len(bars) > 0 || q.Offset > 0) {
			return bars, hasMore, nil
		}
	}

	bars, err := s.provider.Kline(ctx, q.Code, q.Period, adjust)
	if err != nil {
		return nil, false, err
	}
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].TradeDate.Before(bars[j].TradeDate)
	})
	window, hasMore := windowBars(bars, q.Offset, limit)
	return window, hasMore, nil
}

// windowBars 从按交易日升序排列的全量 bars 中，跳过最近 offset 根、取 limit 根，
// 返回该历史窗口（仍升序）及 hasMore（窗口左侧是否还有更早数据）。纯函数，便于单测。
func windowBars(bars []model.StockDailyKline, offset, limit int) ([]model.StockDailyKline, bool) {
	if limit <= 0 {
		limit = 120
	}
	if offset < 0 {
		offset = 0
	}
	end := len(bars) - offset
	if end <= 0 {
		return nil, false
	}
	start := end - limit
	hasMore := start > 0
	if start < 0 {
		start = 0
	}
	return bars[start:end], hasMore
}
