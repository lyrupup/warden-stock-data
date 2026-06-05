//go:build gotdx

package market

import (
	"fmt"
	"time"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
	"github.com/shopspring/decimal"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

func klineTypeOf(period string) (uint16, error) {
	switch period {
	case "", "day":
		return types.KLINE_TYPE_RI_K, nil
	case "week":
		return types.KLINE_TYPE_WEEKLY, nil
	case "month":
		return types.KLINE_TYPE_MONTHLY, nil
	default:
		return 0, fmt.Errorf("unsupported kline period: %s", period)
	}
}

func priceFromFloat(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v)
}

func volumeFromFloat(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v)
}

func changePercent(price, prevClose float64) decimal.Decimal {
	if prevClose == 0 {
		return decimal.Zero
	}
	return decimal.NewFromFloat((price - prevClose) / prevClose * 100)
}

func turnoverFromGotdx(v float64) decimal.Decimal {
	// gotdx Turnover = Vol(手)×10000/流通股(万股)，量纲为「真实换手率% × 10000」
	return decimal.NewFromFloat(v / 10000)
}

func mapStockQuote(q proto.SecurityQuote, name string, now time.Time) model.StockQuote {
	if name == "" {
		name = q.Code
	}
	return model.StockQuote{
		Market: "CN", StockCode: q.Code, StockName: name,
		Price: priceFromFloat(q.Price), Open: priceFromFloat(q.Open),
		High: priceFromFloat(q.High), Low: priceFromFloat(q.Low),
		PrevClose: priceFromFloat(q.PreClose), ChangePercent: changePercent(q.Price, q.PreClose),
		Volume: volumeFromFloat(float64(q.Vol)), Amount: priceFromFloat(q.Amount),
		TurnoverRate: turnoverFromGotdx(q.Turnover),
		TradeDate: now.Truncate(24 * time.Hour), SnapshotAt: now,
	}
}
