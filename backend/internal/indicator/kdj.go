package indicator

import "github.com/shopspring/decimal"

// KDJ 默认参数（9,3,3，通达信口径）：
// RSV = (收盘 - N 日最低) / (N 日最高 - N 日最低) * 100；
// K = SMA(RSV,3,1)、D = SMA(K,3,1)，初值 50；J = 3K - 2D。
const (
	kdjN = 9
	kdjK = 3
	kdjD = 3
)

func init() {
	Register(kdjComponent{typ: "kdj_k", part: "k"})
	Register(kdjComponent{typ: "kdj_d", part: "d"})
	Register(kdjComponent{typ: "kdj_j", part: "j"})
}

// kdjSeriesAll 流式计算全序列 K/D/J，O(n)；单根窗口用滑动高低价。
func kdjSeriesAll(bars []Bar) (k, d, j []decimal.Decimal) {
	n := len(bars)
	k = make([]decimal.Decimal, n)
	d = make([]decimal.Decimal, n)
	j = make([]decimal.Decimal, n)
	hundred := decimal.NewFromInt(100)
	prevK := decimal.NewFromInt(50)
	prevD := decimal.NewFromInt(50)
	kw := decimal.NewFromInt(int64(kdjK))
	dw := decimal.NewFromInt(int64(kdjD))
	for i := range bars {
		rsv := decimal.Zero
		if i >= kdjN-1 {
			high := bars[i].High
			low := bars[i].Low
			for idx := i - kdjN + 1; idx <= i; idx++ {
				if bars[idx].High.GreaterThan(high) {
					high = bars[idx].High
				}
				if bars[idx].Low.LessThan(low) {
					low = bars[idx].Low
				}
			}
			rng := high.Sub(low)
			if rng.IsPositive() {
				rsv = bars[i].Close.Sub(low).Div(rng).Mul(hundred)
			}
		}
		prevK = prevK.Mul(kw.Sub(decimal.NewFromInt(1))).Add(rsv).Div(kw)
		prevD = prevD.Mul(dw.Sub(decimal.NewFromInt(1))).Add(prevK).Div(dw)
		k[i] = prevK
		d[i] = prevD
		j[i] = prevK.Mul(decimal.NewFromInt(3)).Sub(prevD.Mul(decimal.NewFromInt(2)))
	}
	return k, d, j
}

// kdjSeries 返回最后一根的 K / D / J 值。
func kdjSeries(bars []Bar) (kVal, dVal, jVal decimal.Decimal) {
	k, d, j := kdjSeriesAll(bars)
	if len(bars) == 0 {
		return decimal.Zero, decimal.Zero, decimal.Zero
	}
	last := len(bars) - 1
	return k[last], d[last], j[last]
}

type kdjComponent struct {
	typ  string
	part string
}

func (m kdjComponent) Type() string { return m.typ }

func (m kdjComponent) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	if len(s.Bars) < kdjN {
		return decimal.Zero, ErrInsufficientData
	}
	k, d, j := kdjSeries(s.Bars)
	switch m.part {
	case "k":
		return k, nil
	case "d":
		return d, nil
	default:
		return j, nil
	}
}
