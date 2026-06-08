package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
)

type IndicatorService struct {
	kline    *KlineService
	indiRepo *repository.IndicatorRepository
	market   string
}

func NewIndicatorService(kline *KlineService, indiRepo *repository.IndicatorRepository) *IndicatorService {
	return &IndicatorService{kline: kline, indiRepo: indiRepo, market: "CN"}
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
	if s.indiRepo != nil {
		snaps, err := s.indiRepo.GetSnapshots(ctx, s.market, []string{code}, nil)
		if err == nil && len(snaps) > 0 {
			vals, err := parseSnapshotValues(snaps[0], types)
			if err == nil && len(vals) > 0 {
				return &IndicatorResult{
					StockCode: code,
					TradeDate: snaps[0].TradeDate.Format("2006-01-02"),
					Values:    vals,
				}, nil
			}
		}
	}
	return s.computeRealtime(ctx, code, types)
}

func (s *IndicatorService) BatchIndicators(ctx context.Context, codes, types []string, tradeDate *time.Time) ([]IndicatorResult, error) {
	if s.indiRepo != nil {
		snaps, err := s.indiRepo.GetSnapshots(ctx, s.market, codes, tradeDate)
		if err == nil && len(snaps) > 0 {
			out := make([]IndicatorResult, 0, len(snaps))
			for _, snap := range snaps {
				vals, err := parseSnapshotValues(snap, types)
				if err != nil {
					continue
				}
				out = append(out, IndicatorResult{
					StockCode: snap.StockCode,
					TradeDate: snap.TradeDate.Format("2006-01-02"),
					Values:    vals,
				})
			}
			if len(out) > 0 {
				return out, nil
			}
		}
	}
	out := make([]IndicatorResult, 0, len(codes))
	for _, code := range codes {
		r, err := s.computeRealtime(ctx, code, types)
		if err != nil {
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}

func (s *IndicatorService) computeRealtime(ctx context.Context, code string, types []string) (*IndicatorResult, error) {
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

func parseSnapshotValues(snap model.StockIndicatorSnapshot, types []string) (map[string]string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(snap.Values, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, t := range types {
		if v, ok := raw[t]; ok {
			out[t] = decimal.NewFromFloat(toFloat(v)).StringFixed(4)
		}
	}
	return out, nil
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case string:
		d, _ := decimal.NewFromString(n)
		f, _ := d.Float64()
		return f
	default:
		return 0
	}
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

// KlineIndicatorsResponse 为 K 线接口带指标返回结构：bars 为 OHLCV 序列，
// indicators 为与 bars 按 trade_date 对齐的逐 bar 指标（point-in-time）。
type KlineIndicatorsResponse struct {
	Bars       []model.StockDailyKline `json:"bars"`
	Indicators []IndicatorResult       `json:"indicators"`
}

// KlineIndicators 计算与 bars 对齐的逐 bar 指标序列，采用「快照优先 + 实时补齐」混合策略：
//   - 日 K + 前复权 且指标在默认快照集合内 → 读 stock_indicator_snapshots 区间快照（含完整历史预热、口径统一）；
//   - 其余（周/月 K、后复权、非默认指标如 ma120、或快照缺口）→ 在返回的 bars 上逐 bar point-in-time 实时计算补齐。
//
// 单个指标数据不足（如序列开头算不出 MA60）时该 bar 跳过该指标，不影响其它指标，无未来函数。
func (s *IndicatorService) KlineIndicators(ctx context.Context, code, period, adjust string, bars []model.StockDailyKline, types []string) []IndicatorResult {
	if len(bars) == 0 || len(types) == 0 {
		return nil
	}

	// 快照可用条件：日 K + 前复权（adjust 空串经上游归一为 qfq）+ 至少一个请求指标在默认快照集合内。
	snapMap := make(map[string]map[string]string)
	useSnapshot := s.indiRepo != nil && period == "day" && (adjust == "qfq" || adjust == "")
	if useSnapshot {
		snapTypes := make([]string, 0, len(types))
		for _, t := range types {
			if indicator.InDefaultSnapshot(t) {
				snapTypes = append(snapTypes, t)
			}
		}
		if len(snapTypes) > 0 {
			from := bars[0].TradeDate
			to := bars[len(bars)-1].TradeDate
			if snaps, err := s.indiRepo.GetSnapshotsRange(ctx, s.market, code, from, to); err == nil {
				for _, snap := range snaps {
					if vals, err := parseSnapshotValues(snap, snapTypes); err == nil && len(vals) > 0 {
						snapMap[snap.TradeDate.Format("2006-01-02")] = vals
					}
				}
			}
		}
	}

	all := klinesToSeries(bars).Bars
	out := make([]IndicatorResult, 0, len(bars))
	for i := range bars {
		date := bars[i].TradeDate.Format("2006-01-02")
		vals := make(map[string]string, len(types))
		if m, ok := snapMap[date]; ok {
			for k, v := range m {
				vals[k] = v
			}
		}
		// 对快照未覆盖的请求指标（非默认指标 / 快照缺口）实时补齐。
		var sub indicator.Series
		for _, t := range types {
			if _, ok := vals[t]; ok {
				continue
			}
			if sub.Bars == nil {
				sub = indicator.Series{Bars: all[:i+1]}
			}
			if v, err := indicator.Compute(t, sub, nil); err == nil {
				vals[t] = v.StringFixed(4)
			}
		}
		if len(vals) > 0 {
			out = append(out, IndicatorResult{StockCode: code, TradeDate: date, Values: vals})
		}
	}
	return out
}

// WriteSnapshot saves computed indicator values for a trade date.
func (s *IndicatorService) WriteSnapshot(ctx context.Context, code string, tradeDate time.Time, vals map[string]decimal.Decimal) error {
	if s.indiRepo == nil {
		return nil
	}
	return s.indiRepo.UpsertSnapshot(ctx, s.market, code, tradeDate, vals)
}
