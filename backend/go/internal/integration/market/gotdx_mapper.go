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

// cnLoc 固定东八区时区，避免容器缺少 tzdata 时 time.LoadLocation 失败。
var cnLoc = time.FixedZone("CST", 8*3600)

// intradayMinute 返回当日第 i 个（0 基）A 股交易分钟的墙钟时间。
// 上午 09:30–11:30 共 120 分钟，下午 13:00–15:00 共 120 分钟，合计 240 个点。
func intradayMinute(date time.Time, i int) time.Time {
	y, m, d := date.Date()
	if i < 120 {
		return time.Date(y, m, d, 9, 30, 0, 0, cnLoc).Add(time.Duration(i) * time.Minute)
	}
	return time.Date(y, m, d, 13, 0, 0, 0, cnLoc).Add(time.Duration(i-120) * time.Minute)
}

// mapIntradayPoints 将 gotdx 分时图原始点映射为领域分时点；按索引推算交易分钟时间。
func mapIntradayPoints(list []proto.MinuteTimeData, date time.Time) []model.IntradayPoint {
	out := make([]model.IntradayPoint, 0, len(list))
	for i, p := range list {
		out = append(out, model.IntradayPoint{
			Time:     intradayMinute(date, i).Format(time.RFC3339),
			Price:    priceFromFloat(p.Price),
			AvgPrice: priceFromFloat(p.Avg),
			Volume:   volumeFromFloat(float64(p.Vol)),
		})
	}
	return out
}

// mapHistoryIntradayPoints 复用分时点映射处理历史分时（结构同当日分时，仅类型不同）。
func mapHistoryIntradayPoints(list []proto.HistoryMinuteTimeData, date time.Time) []model.IntradayPoint {
	conv := make([]proto.MinuteTimeData, len(list))
	for i, p := range list {
		conv[i] = proto.MinuteTimeData{Price: p.Price, Avg: p.Avg, Vol: p.Vol}
	}
	return mapIntradayPoints(conv, date)
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

// changeAmount 计算涨跌额（现价 - 昨收）。昨收缺失时返回 0，避免出现等同现价的伪涨跌额。
func changeAmount(price, prevClose float64) decimal.Decimal {
	if prevClose == 0 {
		return decimal.Zero
	}
	return decimal.NewFromFloat(price - prevClose)
}

func turnoverFromGotdx(v float64) decimal.Decimal {
	// gotdx Turnover = Vol(手)×10000/流通股(万股)，量纲为「真实换手率% × 10000」
	return decimal.NewFromFloat(v / 10000)
}

// mapIndexQuote 将 gotdx 指数快照（与个股共用 QuoteListItem 结构）映射为标准指数行情。
// QuoteListItem 已携带 Vol/Amount/PreClose，需完整映射，避免成交量额与涨跌额丢失。
func mapIndexQuote(q proto.QuoteListItem, name string, tradeDate, now time.Time) model.IndexQuote {
	if name == "" {
		name = q.Code
	}
	return model.IndexQuote{
		Market: "CN", IndexCode: q.Code, IndexName: name,
		Price:         priceFromFloat(q.Price),
		ChangeAmount:  changeAmount(q.Price, q.PreClose),
		ChangePercent: changePercent(q.Price, q.PreClose),
		Volume:        volumeFromFloat(float64(q.Vol)),
		Amount:        priceFromFloat(q.Amount),
		TradeDate:     tradeDate, SnapshotAt: now,
	}
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
