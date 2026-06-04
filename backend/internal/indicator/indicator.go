package indicator

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type IIndicator interface {
	Type() string
	Compute(s Series, params map[string]interface{}) (decimal.Decimal, error)
}

var registry = map[string]IIndicator{}

func Register(i IIndicator) {
	registry[i.Type()] = i
}

func Compute(typ string, s Series, params map[string]interface{}) (decimal.Decimal, error) {
	i, ok := registry[typ]
	if !ok {
		return decimal.Zero, fmt.Errorf("unknown indicator: %s", typ)
	}
	return i.Compute(s, params)
}

func ComputeAll(s Series, types []string) (map[string]decimal.Decimal, error) {
	out := make(map[string]decimal.Decimal, len(types))
	for _, typ := range types {
		v, err := Compute(typ, s, nil)
		if err != nil {
			return nil, err
		}
		out[typ] = v
	}
	return out, nil
}

// Catalog returns registered indicator metadata for /open/v1/meta.
func Catalog() []map[string]interface{} {
	return []map[string]interface{}{
		{"type": "ma5", "name": "MA5", "implemented": true},
		{"type": "ma10", "name": "MA10", "implemented": true},
		{"type": "ma20", "name": "MA20", "implemented": true},
		{"type": "ma30", "name": "MA30", "implemented": true},
		{"type": "ma60", "name": "MA60", "implemented": true},
		{"type": "bias5", "name": "乖离率5", "implemented": true},
		{"type": "bias10", "name": "乖离率10", "implemented": true},
		{"type": "bias20", "name": "乖离率20", "implemented": true},
		{"type": "ma_align", "name": "均线多头排列", "implemented": true},
		{"type": "amplitude", "name": "振幅", "implemented": true},
		{"type": "amplitude_streak", "name": "连续振幅", "implemented": true},
		{"type": "pct_change", "name": "涨跌幅", "implemented": true},
		{"type": "pct_change5", "name": "5日涨跌幅", "implemented": true},
		{"type": "vol_ratio", "name": "量比", "implemented": true},
		{"type": "macd", "name": "MACD", "implemented": false},
		{"type": "kdj", "name": "KDJ", "implemented": false},
		{"type": "rsi", "name": "RSI", "implemented": false},
		{"type": "boll", "name": "BOLL", "implemented": false},
	}
}
