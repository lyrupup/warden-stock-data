package indicator

import (
	"fmt"

	"github.com/shopspring/decimal"
)

func init() {
	Register(biasIndicator{typ: "bias5", period: 5})
	Register(biasIndicator{typ: "bias10", period: 10})
	Register(biasIndicator{typ: "bias20", period: 20})
	Register(maAlignIndicator{typ: "ma_align"})
	Register(amplitudeIndicator{typ: "amplitude"})
	Register(amplitudeStreakIndicator{typ: "amplitude_streak", threshold: decimal.NewFromFloat(5)})
	Register(pctChangeIndicator{typ: "pct_change", days: 1})
	Register(pctChangeIndicator{typ: "pct_change5", days: 5})
	Register(pctChangeIndicator{typ: "pct_change20", days: 20})
	Register(pctChangeIndicator{typ: "pct_change60", days: 60})
	Register(fieldIndicator{typ: "close", field: "close"})
	Register(fieldIndicator{typ: "volume", field: "volume"})
	Register(volRatioIndicator{typ: "vol_ratio", period: 5})
}

type biasIndicator struct {
	typ    string
	period int
}

func (b biasIndicator) Type() string { return b.typ }

func (b biasIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	ma, err := MA(s, b.period)
	if err != nil {
		return decimal.Zero, err
	}
	if ma.IsZero() {
		return decimal.Zero, ErrInsufficientData
	}
	last := s.Bars[len(s.Bars)-1].Close
	return last.Sub(ma).Div(ma).Mul(decimal.NewFromInt(100)), nil
}

type maAlignIndicator struct{ typ string }

func (m maAlignIndicator) Type() string { return m.typ }

func (m maAlignIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	periods := []int{5, 10, 20, 30, 60}
	vals := make([]decimal.Decimal, 0, len(periods))
	for _, p := range periods {
		v, err := MA(s, p)
		if err != nil {
			return decimal.Zero, nil
		}
		vals = append(vals, v)
	}
	aligned := true
	for i := 1; i < len(vals); i++ {
		if !vals[i-1].GreaterThan(vals[i]) {
			aligned = false
			break
		}
	}
	if aligned {
		return decimal.NewFromInt(1), nil
	}
	return decimal.Zero, nil
}

type amplitudeIndicator struct{ typ string }

func (a amplitudeIndicator) Type() string { return a.typ }

func (a amplitudeIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	if len(s.Bars) < 2 {
		return decimal.Zero, ErrInsufficientData
	}
	last := s.Bars[len(s.Bars)-1]
	prev := s.Bars[len(s.Bars)-2].Close
	if prev.IsZero() {
		return decimal.Zero, ErrInsufficientData
	}
	return last.High.Sub(last.Low).Div(prev).Mul(decimal.NewFromInt(100)), nil
}

type amplitudeStreakIndicator struct {
	typ       string
	threshold decimal.Decimal
}

func (a amplitudeStreakIndicator) Type() string { return a.typ }

func (a amplitudeStreakIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	if len(s.Bars) < 2 {
		return decimal.Zero, nil
	}
	streak := 0
	for i := len(s.Bars) - 1; i >= 1; i-- {
		bar := s.Bars[i]
		prev := s.Bars[i-1].Close
		if prev.IsZero() {
			break
		}
		amp := bar.High.Sub(bar.Low).Div(prev).Mul(decimal.NewFromInt(100))
		if amp.GreaterThanOrEqual(a.threshold) {
			streak++
		} else {
			break
		}
	}
	return decimal.NewFromInt(int64(streak)), nil
}

type pctChangeIndicator struct {
	typ  string
	days int
}

func (p pctChangeIndicator) Type() string { return p.typ }

func (p pctChangeIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	if len(s.Bars) <= p.days {
		return decimal.Zero, ErrInsufficientData
	}
	last := s.Bars[len(s.Bars)-1].Close
	base := s.Bars[len(s.Bars)-1-p.days].Close
	if base.IsZero() {
		return decimal.Zero, ErrInsufficientData
	}
	return last.Sub(base).Div(base).Mul(decimal.NewFromInt(100)), nil
}

type fieldIndicator struct {
	typ   string
	field string
}

func (f fieldIndicator) Type() string { return f.typ }

func (f fieldIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	if len(s.Bars) == 0 {
		return decimal.Zero, ErrInsufficientData
	}
	last := s.Bars[len(s.Bars)-1]
	switch f.field {
	case "close":
		return last.Close, nil
	case "open":
		return last.Open, nil
	case "high":
		return last.High, nil
	case "low":
		return last.Low, nil
	case "volume":
		return last.Volume, nil
	default:
		return decimal.Zero, fmt.Errorf("unknown field: %s", f.field)
	}
}

type volRatioIndicator struct {
	typ    string
	period int
}

func (v volRatioIndicator) Type() string { return v.typ }

func (v volRatioIndicator) Compute(s Series, _ map[string]interface{}) (decimal.Decimal, error) {
	if len(s.Bars) < v.period {
		return decimal.Zero, ErrInsufficientData
	}
	slice := s.Bars[len(s.Bars)-v.period:]
	sum := decimal.Zero
	for _, b := range slice {
		sum = sum.Add(b.Volume)
	}
	avg := sum.Div(decimal.NewFromInt(int64(v.period)))
	if avg.IsZero() {
		return decimal.Zero, ErrInsufficientData
	}
	return slice[len(slice)-1].Volume.Div(avg), nil
}
