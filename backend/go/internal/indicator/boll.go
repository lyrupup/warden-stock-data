package indicator

import (
	"math"

	"github.com/shopspring/decimal"
)

// BOLL 默认参数（20,2）：中轨 = MA20；上/下轨 = 中轨 ± 2 倍收盘价总体标准差。
const (
	bollPeriod = 20
	bollMult   = 2
)

func init() {
	Register(bollComponent{typ: "boll_mid", part: "mid"})
	Register(bollComponent{typ: "boll_upper", part: "upper"})
	Register(bollComponent{typ: "boll_lower", part: "lower"})
}

// bollBands 返回最后一根的中轨 / 上轨 / 下轨。
func bollBands(s Series) (mid, upper, lower decimal.Decimal, err error) {
	mid, err = MA(s, bollPeriod)
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, err
	}
	slice := s.Bars[len(s.Bars)-bollPeriod:]
	variance := decimal.Zero
	for _, b := range slice {
		diff := b.Close.Sub(mid)
		variance = variance.Add(diff.Mul(diff))
	}
	variance = variance.Div(decimal.NewFromInt(int64(bollPeriod)))
	vf, _ := variance.Float64()
	std := decimal.NewFromFloat(math.Sqrt(vf))
	band := std.Mul(decimal.NewFromInt(int64(bollMult)))
	return mid, mid.Add(band), mid.Sub(band), nil
}

type bollComponent struct {
	typ  string
	part string
}

func (b bollComponent) Type() string { return b.typ }

func (b bollComponent) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	mid, upper, lower, err := bollBands(s)
	if err != nil {
		return decimal.Zero, err
	}
	switch b.part {
	case "upper":
		return upper, nil
	case "lower":
		return lower, nil
	default:
		return mid, nil
	}
}
