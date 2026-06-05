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
