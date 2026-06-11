package indicator

import (
	"time"

	"github.com/shopspring/decimal"
)

type Bar struct {
	TradeDate time.Time
	Close     decimal.Decimal
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Volume    decimal.Decimal
}

type Series struct {
	Bars []Bar
}

func (s Series) Closes() []decimal.Decimal {
	out := make([]decimal.Decimal, len(s.Bars))
	for i, b := range s.Bars {
		out[i] = b.Close
	}
	return out
}
