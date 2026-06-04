package indicator

import (
	"fmt"

	"github.com/shopspring/decimal"
)

var ErrInsufficientData = fmt.Errorf("insufficient data for indicator")

// MA computes simple moving average of close prices for the last `period` bars.
func MA(s Series, period int) (decimal.Decimal, error) {
	if period <= 0 || len(s.Bars) < period {
		return decimal.Zero, ErrInsufficientData
	}
	slice := s.Bars[len(s.Bars)-period:]
	sum := decimal.Zero
	for _, b := range slice {
		sum = sum.Add(b.Close)
	}
	return sum.Div(decimal.NewFromInt(int64(period))), nil
}

type maIndicator struct {
	period int
	typ    string
}

func (m maIndicator) Type() string { return m.typ }

func (m maIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	return MA(s, m.period)
}

func registerMA(typ string, period int) {
	Register(maIndicator{period: period, typ: typ})
}

func init() {
	registerMA("ma5", 5)
	registerMA("ma10", 10)
	registerMA("ma20", 20)
	registerMA("ma30", 30)
	registerMA("ma60", 60)
}
