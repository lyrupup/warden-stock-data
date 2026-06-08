package indicator

import "github.com/shopspring/decimal"

// MACD 默认参数（通达信口径）：快线 EMA12、慢线 EMA26、信号线 DEA 为 DIF 的 EMA9，
// MACD 柱（bar）= (DIF - DEA) * 2。
const (
	macdFast   = 12
	macdSlow   = 26
	macdSignal = 9
)

func init() {
	Register(macdComponent{typ: "macd_dif", part: "dif"})
	Register(macdComponent{typ: "macd_dea", part: "dea"})
	Register(macdComponent{typ: "macd_bar", part: "bar"})
}

// ema 按收盘价序列计算指数移动均线（递归口径，首根以自身收盘价播种，与通达信一致）。
func ema(values []decimal.Decimal, period int) []decimal.Decimal {
	out := make([]decimal.Decimal, len(values))
	if len(values) == 0 {
		return out
	}
	k := decimal.NewFromInt(2).Div(decimal.NewFromInt(int64(period) + 1))
	oneMinusK := decimal.NewFromInt(1).Sub(k)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = values[i].Mul(k).Add(out[i-1].Mul(oneMinusK))
	}
	return out
}

// macdSeries 返回逐根的 DIF / DEA / MACD 柱序列。
func macdSeries(closes []decimal.Decimal) (dif, dea, bar []decimal.Decimal) {
	emaFast := ema(closes, macdFast)
	emaSlow := ema(closes, macdSlow)
	dif = make([]decimal.Decimal, len(closes))
	for i := range closes {
		dif[i] = emaFast[i].Sub(emaSlow[i])
	}
	dea = ema(dif, macdSignal)
	bar = make([]decimal.Decimal, len(closes))
	two := decimal.NewFromInt(2)
	for i := range closes {
		bar[i] = dif[i].Sub(dea[i]).Mul(two)
	}
	return dif, dea, bar
}

type macdComponent struct {
	typ  string
	part string
}

func (m macdComponent) Type() string { return m.typ }

func (m macdComponent) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	if len(s.Bars) < macdSlow+macdSignal {
		return decimal.Zero, ErrInsufficientData
	}
	dif, dea, bar := macdSeries(s.Closes())
	last := len(s.Bars) - 1
	switch m.part {
	case "dif":
		return dif[last], nil
	case "dea":
		return dea[last], nil
	default:
		return bar[last], nil
	}
}
