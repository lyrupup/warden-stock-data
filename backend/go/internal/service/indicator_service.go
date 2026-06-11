package service

import (
	"context"
	"time"

	"github.com/warden-stock/warden-stock-data/internal/integration/quant"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
)

type IndicatorService struct {
	quant  quant.IQuantClient
	market string
}

func NewIndicatorService(quantClient quant.IQuantClient) *IndicatorService {
	return &IndicatorService{quant: quantClient, market: "CN"}
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
	if s.quant == nil {
		return nil, errcodeBiz{code: errcode.ErrIndicatorParam}
	}
	resp, err := s.quant.BatchIndicators(ctx, quant.IndicatorRequest{
		Codes:  []string{code},
		Types:  types,
		Period: "day",
		Adjust: "qfq",
		Market: s.market,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, errcodeBiz{code: errcode.ErrIndicatorParam}
	}
	r := resp.Results[0]
	return &IndicatorResult{StockCode: r.StockCode, TradeDate: r.TradeDate, Values: r.Values}, nil
}

func (s *IndicatorService) BatchIndicators(ctx context.Context, codes, types []string, tradeDate *string) ([]IndicatorResult, error) {
	if s.quant == nil {
		return nil, errcodeBiz{code: errcode.ErrIndicatorParam}
	}
	td := ""
	if tradeDate != nil {
		td = *tradeDate
	}
	resp, err := s.quant.BatchIndicators(ctx, quant.IndicatorRequest{
		Codes:     codes,
		Types:     types,
		Period:    "day",
		Adjust:    "qfq",
		TradeDate: td,
		Market:    s.market,
	})
	if err != nil {
		return nil, err
	}
	out := make([]IndicatorResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		out = append(out, IndicatorResult{StockCode: r.StockCode, TradeDate: r.TradeDate, Values: r.Values})
	}
	return out, nil
}

type KlineIndicatorsResponse struct {
	Bars       []model.StockDailyKline `json:"bars"`
	Indicators []IndicatorResult       `json:"indicators"`
	HasMore    bool                    `json:"has_more"`
}

// KlineIndicators 经 Python quant 实时计算与 bars 对齐的逐 bar 指标序列。
func (s *IndicatorService) KlineIndicators(
	ctx context.Context, code, period, adjust string, bars []model.StockDailyKline, types []string,
	limit, offset int, from, to *time.Time,
) []IndicatorResult {
	if len(bars) == 0 || len(types) == 0 || s.quant == nil {
		return nil
	}
	if period != "day" && period != "" {
		return nil
	}
	if adjust == "" {
		adjust = "qfq"
	}
	req := quant.IndicatorSeriesRequest{
		Code:   code,
		Types:  types,
		Period: "day",
		Adjust: adjust,
		Market: s.market,
	}
	if limit > 0 {
		req.Limit = limit
		req.Offset = offset
	}
	if from != nil {
		req.FromDate = from.Format("2006-01-02")
	}
	if to != nil {
		req.ToDate = to.Format("2006-01-02")
	}
	if req.Limit == 0 && len(bars) > 0 {
		req.FromDate = bars[0].TradeDate.Format("2006-01-02")
		req.ToDate = bars[len(bars)-1].TradeDate.Format("2006-01-02")
	}
	resp, err := s.quant.SeriesIndicators(ctx, req)
	if err != nil {
		return nil
	}
	out := make([]IndicatorResult, 0, len(resp.Indicators))
	for _, r := range resp.Indicators {
		out = append(out, IndicatorResult{StockCode: r.StockCode, TradeDate: r.TradeDate, Values: r.Values})
	}
	return out
}
