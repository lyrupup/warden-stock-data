package market

import (
	"context"
	"sort"
	"time"

	"github.com/bensema/gotdx"
	"github.com/bensema/gotdx/proto"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

const (
	// klinePageSize 通达信单次 K 线请求稳定上限（count>600 易触发异常短帧 / gotdx panic）。
	klinePageSize = 600
	// maxKlinePages 分页安全上限（约 6 万根），防止异常数据导致死循环。
	maxKlinePages = 100
)

// mapSecurityBars 将 gotdx K 线原始列表映射为领域日 K 模型。
// gotdx 日 K 自带昨收(Last)/成交额(Amount)/换手率(Turnover)/涨跌幅(RiseRate)，一并映射，
// 供 Go 原生增量采集直接落库（涨跌停/ST/停牌由上层补全）。
func mapSecurityBars(list []proto.SecurityBar, code, adjust string) []model.StockDailyKline {
	bars := make([]model.StockDailyKline, 0, len(list))
	for _, b := range list {
		td := b.DateTime
		if td.IsZero() {
			td = time.Date(b.Year, time.Month(b.Month), b.Day, 0, 0, 0, 0, time.Local)
		}
		bars = append(bars, model.StockDailyKline{
			Market: "CN", Source: "gotdx", StockCode: code, TradeDate: td,
			Open: priceFromFloat(b.Open), High: priceFromFloat(b.High),
			Low: priceFromFloat(b.Low), Close: priceFromFloat(b.Close),
			PreClose:     priceFromFloat(b.Last),
			Volume:       volumeFromFloat(b.Vol),
			Amount:       priceFromFloat(b.Amount),
			TurnoverRate: priceFromFloat(b.Turnover),
			PctChg:       priceFromFloat(b.RiseRate),
			Adjust:       adjust,
		})
	}
	return bars
}

// sortDedupeKlineBars 合并分页结果：按交易日升序，同日去重（保留后出现者）。
func sortDedupeKlineBars(bars []model.StockDailyKline) []model.StockDailyKline {
	if len(bars) == 0 {
		return bars
	}
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].TradeDate.Before(bars[j].TradeDate)
	})
	out := make([]model.StockDailyKline, 0, len(bars))
	var last time.Time
	for _, b := range bars {
		if len(out) > 0 && b.TradeDate.Equal(last) {
			out[len(out)-1] = b
			continue
		}
		out = append(out, b)
		last = b.TradeDate
	}
	return out
}

// fetchKlinePage 拉取单页 K 线；start 为自最新向历史的偏移（0=最近一页）。
func fetchKlinePage(c *gotdx.Client, klineType uint16, mkt uint8, code string, start, count uint16) ([]proto.SecurityBar, error) {
	reply, err := c.GetSecurityBars(klineType, mkt, code, start, count)
	if err != nil {
		return nil, err
	}
	if reply == nil {
		return nil, nil
	}
	return reply.List, nil
}

// fetchKlineAll 分页回溯拉取标的全部历史 K 线（自最新向更早逐页拼接）。
func fetchKlineAll(ctx context.Context, c *gotdx.Client, klineType uint16, mkt uint8, code, adjust string) ([]model.StockDailyKline, error) {
	var all []model.StockDailyKline
	start := uint16(0)
	for page := 0; page < maxKlinePages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := fetchKlinePage(c, klineType, mkt, code, start, klinePageSize)
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			break
		}
		mapped := mapSecurityBars(raw, code, adjust)
		// start 递增取更早一页，拼到已有结果前面。
		all = append(mapped, all...)
		if len(raw) < klinePageSize {
			break
		}
		start += uint16(len(raw))
	}
	return sortDedupeKlineBars(all), nil
}
