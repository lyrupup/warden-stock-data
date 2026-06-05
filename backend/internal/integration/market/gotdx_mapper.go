//go:build gotdx

package market

import (
	"github.com/shopspring/decimal"
)

// priceFromGotdx converts gotdx price integer (cents or 1/1000 yuan) to decimal yuan.
func priceFromGotdx(v int) decimal.Decimal {
	return decimal.NewFromInt(int64(v)).Div(decimal.NewFromInt(100))
}

func volumeFromGotdx(v int64) decimal.Decimal {
	return decimal.NewFromInt(v)
}

func turnoverFromGotdx(v int) decimal.Decimal {
	return decimal.NewFromInt(int64(v)).Div(decimal.NewFromInt(10000))
}
