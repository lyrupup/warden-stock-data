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
