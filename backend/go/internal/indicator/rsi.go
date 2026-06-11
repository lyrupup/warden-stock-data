package indicator

import "github.com/shopspring/decimal"

func init() {
	Register(rsiIndicator{typ: "rsi6", period: 6})
	Register(rsiIndicator{typ: "rsi12", period: 12})
	Register(rsiIndicator{typ: "rsi24", period: 24})
}

// RSI 采用 Wilder 平滑口径：首个均值取前 period 根涨跌幅的简单平均，
// 之后按 avg = (avg*(period-1) + 当前值) / period 平滑。avgLoss 为 0 时 RSI=100。
func RSI(s Series, period int) (decimal.Decimal, error) {
	if period <= 0 || len(s.Bars) < period+1 {
		return decimal.Zero, ErrInsufficientData
	}
	closes := s.Closes()
	gains := make([]decimal.Decimal, len(closes))
	losses := make([]decimal.Decimal, len(closes))
	for i := 1; i < len(closes); i++ {
		diff := closes[i].Sub(closes[i-1])
		if diff.IsPositive() {
			gains[i] = diff
		} else {
			losses[i] = diff.Neg()
		}
	}
	pd := decimal.NewFromInt(int64(period))
	avgGain := decimal.Zero
	avgLoss := decimal.Zero
	for i := 1; i <= period; i++ {
		avgGain = avgGain.Add(gains[i])
		avgLoss = avgLoss.Add(losses[i])
	}
	avgGain = avgGain.Div(pd)
	avgLoss = avgLoss.Div(pd)
	pm1 := decimal.NewFromInt(int64(period) - 1)
	for i := period + 1; i < len(closes); i++ {
		avgGain = avgGain.Mul(pm1).Add(gains[i]).Div(pd)
		avgLoss = avgLoss.Mul(pm1).Add(losses[i]).Div(pd)
	}
	hundred := decimal.NewFromInt(100)
	if avgLoss.IsZero() {
		return hundred, nil
	}
	rs := avgGain.Div(avgLoss)
	return hundred.Sub(hundred.Div(decimal.NewFromInt(1).Add(rs))), nil
}

type rsiIndicator struct {
	typ    string
	period int
}

func (r rsiIndicator) Type() string { return r.typ }

func (r rsiIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	return RSI(s, r.period)
}
