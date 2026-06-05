//go:build gotdx

package market

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bensema/gotdx"
	"github.com/shopspring/decimal"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type GotdxProvider struct {
	maxConn int
	mu      sync.Mutex
}

func NewGotdxProvider(maxConn int) *GotdxProvider {
	if maxConn <= 0 {
		maxConn = 10
	}
	return &GotdxProvider{maxConn: maxConn}
}

func (p *GotdxProvider) Market() string { return "CN" }
func (p *GotdxProvider) Source() string { return "gotdx" }

func (p *GotdxProvider) withClient(fn func(*gotdx.Client) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	client := gotdx.New()
	if _, err := client.Connect(); err != nil {
		return err
	}
	defer client.Disconnect()
	return fn(client)
}

func (p *GotdxProvider) HealthCheck(ctx context.Context) error {
	return p.withClient(func(c *gotdx.Client) error {
		_, err := c.GetSecurityCount(gotdx.MarketSh)
		return err
	})
}

func (p *GotdxProvider) Indices(ctx context.Context) ([]model.IndexQuote, error) {
	codes := []string{"000001", "399001", "399006"}
	var out []model.IndexQuote
	err := p.withClient(func(c *gotdx.Client) error {
		for _, code := range codes {
			mkt := marketOf(code)
			reply, err := c.GetIndexQuotes([]uint8{mkt}, []string{code})
			if err != nil || len(reply.List) == 0 {
				continue
			}
			q := reply.List[0]
			out = append(out, model.IndexQuote{
				Market: "CN", IndexCode: code, IndexName: q.Name,
				Price: priceFromGotdx(int(q.Price)), ChangePercent: turnoverFromGotdx(int(q.Price)),
				TradeDate: time.Now().Truncate(24 * time.Hour), SnapshotAt: time.Now(),
			})
		}
		return nil
	})
	return out, err
}

func (p *GotdxProvider) Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error) {
	var out []model.StockQuote
	err := p.withClient(func(c *gotdx.Client) error {
		mkts := make([]uint8, len(codes))
		for i, code := range codes {
			mkts[i] = marketOf(code)
		}
		reply, err := c.GetSecurityQuotes(mkts, codes)
		if err != nil {
			return err
		}
		today := time.Now().Truncate(24 * time.Hour)
		for _, q := range reply.List {
			out = append(out, model.StockQuote{
				Market: "CN", StockCode: q.Code, StockName: q.Name,
				Price: priceFromGotdx(int(q.Price)), PrevClose: priceFromGotdx(int(q.LastClose)),
				ChangePercent: turnoverFromGotdx(int(q.Price)),
				Volume: volumeFromGotdx(int64(q.Vol)),
				TradeDate: today, SnapshotAt: time.Now(),
			})
		}
		return nil
	})
	return out, err
}

func (p *GotdxProvider) Kline(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error) {
	if period != "day" && period != "" {
		return nil, fmt.Errorf("gotdx v1 only supports day kline")
	}
	var bars []model.StockDailyKline
	err := p.withClient(func(c *gotdx.Client) error {
		mkt := marketOf(code)
		reply, err := c.GetSecurityBars(mkt, code, gotdx.KLINE_TYPE_RI_K, 0, 800)
		if err != nil {
			return err
		}
		for _, b := range reply.List {
			td, _ := time.Parse("20060102", fmt.Sprintf("%d", b.Date))
			bars = append(bars, model.StockDailyKline{
				Market: "CN", Source: "gotdx", StockCode: code, TradeDate: td,
				Open: priceFromGotdx(int(b.Open)), High: priceFromGotdx(int(b.High)),
				Low: priceFromGotdx(int(b.Low)), Close: priceFromGotdx(int(b.Close)),
				Volume: volumeFromGotdx(int64(b.Vol)), Adjust: adjust,
			})
		}
		return nil
	})
	return bars, err
}

func (p *GotdxProvider) Search(ctx context.Context, kw string) ([]model.Security, error) {
	all, err := p.StockList(ctx)
	if err != nil {
		return nil, err
	}
	var out []model.Security
	for _, s := range all {
		if contains(s.Code, kw) || contains(s.Name, kw) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (p *GotdxProvider) StockList(ctx context.Context) ([]model.Security, error) {
	var out []model.Security
	err := p.withClient(func(c *gotdx.Client) error {
		for _, mkt := range []uint8{gotdx.MarketSh, gotdx.MarketSz} {
			cnt, err := c.GetSecurityCount(mkt)
			if err != nil {
				continue
			}
			reply, err := c.GetSecurityList(mkt, 0, uint16(cnt))
			if err != nil {
				continue
			}
			for _, s := range reply.List {
				out = append(out, model.Security{Market: "CN", Code: s.Code, Name: s.Name, Status: 1})
			}
		}
		return nil
	})
	return out, err
}

func marketOf(code string) uint8 {
	if len(code) > 0 && (code[0] == '6' || code[0] == '5') {
		return gotdx.MarketSh
	}
	return gotdx.MarketSz
}

func contains(s, kw string) bool {
	return len(kw) > 0 && (s == kw || len(s) >= len(kw))
}
