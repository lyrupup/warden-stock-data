package service

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
)

type IndicatorService struct {
	kline *KlineService
}

func NewIndicatorService(kline *KlineService) *IndicatorService {
	return &IndicatorService{kline: kline}
}

type IndicatorResult struct {
	StockCode string            `json:"stock_code"`
	TradeDate string            `json:"trade_date,omitempty"`
	Values    map[string]string `json:"values"`
}

func (s *IndicatorService) ComputeForStock(ctx context.Context, code string, types []string) (*IndicatorResult, error) {
	if len(types) == 0 {
		types = []string{"ma5", "ma10", "ma20", "ma30", "ma60"}
	}
	bars, err := s.kline.Kline(ctx, KlineQuery{Code: code, Period: "day", Adjust: "qfq", Limit: 120})
	if err != nil {
		return nil, err
	}
	series := klinesToSeries(bars)
	vals, err := indicator.ComputeAll(series, types)
	if err != nil {
		return nil, errcodeBiz{code: errcode.ErrIndicatorParam}
	}
	tradeDate := ""
	if len(bars) > 0 {
		tradeDate = bars[len(bars)-1].TradeDate.Format("2006-01-02")
	}
	return &IndicatorResult{
		StockCode: code,
		TradeDate: tradeDate,
		Values:    decimalMapToString(vals),
	}, nil
}

func (s *IndicatorService) BatchIndicators(ctx context.Context, codes, types []string) ([]IndicatorResult, error) {
	out := make([]IndicatorResult, 0, len(codes))
	for _, code := range codes {
		r, err := s.ComputeForStock(ctx, code, types)
		if err != nil {
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}

func klinesToSeries(bars []model.StockDailyKline) indicator.Series {
	series := indicator.Series{Bars: make([]indicator.Bar, len(bars))}
	for i, b := range bars {
		series.Bars[i] = indicator.Bar{
			TradeDate: b.TradeDate,
			Open:      b.Open,
			High:      b.High,
			Low:       b.Low,
			Close:     b.Close,
			Volume:    b.Volume,
		}
	}
	return series
}

func decimalMapToString(m map[string]decimal.Decimal) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v.StringFixed(4)
	}
	return out
}
