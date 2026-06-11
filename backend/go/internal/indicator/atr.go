package indicator

import "github.com/shopspring/decimal"

func init() {
	Register(atrIndicator{typ: "atr14", period: 14})
	Register(atrIndicator{typ: "atr20", period: 20})
}

// ATR（真实波幅均值，Wilder 平滑）：
// TR = max(高-低, |高-前收|, |低-前收|)；首值取前 period 根 TR 的简单平均，
// 之后按 ATR = (ATR*(period-1) + TR) / period 平滑。用于动态止损 / 轨道宽度等。
func ATR(s Series, period int) (decimal.Decimal, error) {
	if period <= 0 || len(s.Bars) < period+1 {
		return decimal.Zero, ErrInsufficientData
	}
	bars := s.Bars
	tr := make([]decimal.Decimal, len(bars))
	tr[0] = bars[0].High.Sub(bars[0].Low)
	for i := 1; i < len(bars); i++ {
		prevClose := bars[i-1].Close
		hl := bars[i].High.Sub(bars[i].Low)
		hc := bars[i].High.Sub(prevClose).Abs()
		lc := bars[i].Low.Sub(prevClose).Abs()
		tr[i] = decimal.Max(hl, hc, lc)
	}
	pd := decimal.NewFromInt(int64(period))
	atr := decimal.Zero
	for i := 1; i <= period; i++ {
		atr = atr.Add(tr[i])
	}
	atr = atr.Div(pd)
	pm1 := decimal.NewFromInt(int64(period) - 1)
	for i := period + 1; i < len(bars); i++ {
		atr = atr.Mul(pm1).Add(tr[i]).Div(pd)
	}
	return atr, nil
}

type atrIndicator struct {
	typ    string
	period int
}

func (a atrIndicator) Type() string { return a.typ }

func (a atrIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	return ATR(s, a.period)
}
