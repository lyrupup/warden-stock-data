package market

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/bensema/gotdx"
	"github.com/bensema/gotdx/types"

	"github.com/warden-stock/warden-stock-data/internal/model"
)

type GotdxProvider struct {
	pool      *GotdxPool
	nameMu    sync.RWMutex
	nameIndex map[string]string // code → name，懒加载自 StockList
}

func NewGotdxProvider(maxConn int) *GotdxProvider {
	return &GotdxProvider{pool: NewGotdxPool(maxConn)}
}

func (p *GotdxProvider) Market() string { return "CN" }
func (p *GotdxProvider) Source() string { return "gotdx" }

// Close 释放底层连接池（进程关停时调用）。
func (p *GotdxProvider) Close() error {
	p.pool.Close()
	return nil
}

// withClient 从连接池借一个复用连接执行 fn。连接复用避免了每次握手的 1-4s 开销；
// gotdx 节点不稳定，出错/panic 的连接由池内丢弃并自动重建（详见 GotdxPool）。
func (p *GotdxProvider) withClient(ctx context.Context, fn func(*gotdx.Client) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return p.pool.WithClient(ctx, fn)
}

func (p *GotdxProvider) HealthCheck(ctx context.Context) error {
	return p.withClient(ctx, func(c *gotdx.Client) error {
		_, err := c.GetSecurityCount(types.MarketSH.Uint8())
		return err
	})
}

func (p *GotdxProvider) Indices(ctx context.Context) ([]model.IndexQuote, error) {
	// 指数代码与股票代码的市场归属规则不同，需显式指定：
	// 000001 上证指数→沪市；399001 深证成指、399006 创业板指→深市。
	codes := []string{"000001", "399001", "399006"}
	mkts := []uint8{
		types.MarketSH.Uint8(),
		types.MarketSZ.Uint8(),
		types.MarketSZ.Uint8(),
	}
	var out []model.IndexQuote
	err := p.withClient(ctx, func(c *gotdx.Client) error {
		list, err := c.StockQuotes(mkts, codes)
		if err != nil {
			return err
		}
		today := time.Now().Truncate(24 * time.Hour)
		now := time.Now()
		for _, q := range list {
			out = append(out, model.IndexQuote{
				Market: "CN", IndexCode: q.Code, IndexName: indexNameOf(q.Code),
				Price: priceFromFloat(q.Price), ChangePercent: changePercent(q.Price, q.PreClose),
				TradeDate: today, SnapshotAt: now,
			})
		}
		return nil
	})
	return out, err
}

func (p *GotdxProvider) Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error) {
	names := p.lookupNames(codes)
	var out []model.StockQuote
	err := p.withClient(ctx, func(c *gotdx.Client) error {
		mkts := make([]uint8, len(codes))
		for i, code := range codes {
			mkts[i] = marketOf(code)
		}
		list, err := c.StockQuotesDetail(mkts, codes)
		if err != nil {
			return err
		}
		now := time.Now()
		for _, q := range list {
			out = append(out, mapStockQuote(q, names[q.Code], now))
		}
		return nil
	})
	return out, err
}

func (p *GotdxProvider) Kline(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error) {
	klineType, err := klineTypeOf(period)
	if err != nil {
		return nil, err
	}
	var bars []model.StockDailyKline
	err = p.withClient(ctx, func(c *gotdx.Client) error {
		mkt := marketOf(code)
		// 注意：通达信单次 K 线请求上限受限，count=800 会触发服务端返回异常短帧
		// 进而导致 gotdx 库解析 panic；实测 <=600 稳定，这里取 600（约 2.5 年日线）。
		reply, err := c.GetSecurityBars(klineType, mkt, code, 0, 600)
		if err != nil {
			return err
		}
		if reply == nil {
			return nil
		}
		for _, b := range reply.List {
			td := b.DateTime
			if td.IsZero() {
				td = time.Date(b.Year, time.Month(b.Month), b.Day, 0, 0, 0, 0, time.Local)
			}
			bars = append(bars, model.StockDailyKline{
				Market: "CN", Source: "gotdx", StockCode: code, TradeDate: td,
				Open: priceFromFloat(b.Open), High: priceFromFloat(b.High),
				Low: priceFromFloat(b.Low), Close: priceFromFloat(b.Close),
				Volume: volumeFromFloat(b.Vol), Adjust: adjust,
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
	kw = strings.TrimSpace(strings.ToLower(kw))
	if kw == "" {
		return nil, nil
	}
	out := make([]model.Security, 0, 50)
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Code), kw) ||
			strings.Contains(strings.ToLower(s.Name), kw) {
			out = append(out, s)
			if len(out) >= 50 {
				break
			}
		}
	}
	return out, nil
}

func (p *GotdxProvider) StockList(ctx context.Context) ([]model.Security, error) {
	var out []model.Security
	err := p.withClient(ctx, func(c *gotdx.Client) error {
		for _, mkt := range []uint8{types.MarketSH.Uint8(), types.MarketSZ.Uint8()} {
			list, err := c.StockAll(mkt)
			if err != nil {
				continue
			}
			for _, s := range list {
				// StockAll 返回沪深全部证券品种（含基金/债券/逆回购/指数/B股等），
				// 这里仅保留 A 股个股，避免证券数量虚高。
				if !isAStock(mkt, s.Code) {
					continue
				}
				out = append(out, model.Security{
					Market: "CN", Code: s.Code, Name: s.Name,
					Board: boardOf(mkt, s.Code), Status: 1,
				})
			}
		}
		return nil
	})
	return out, err
}

// isAStock 按市场 + 代码前缀判定是否为 A 股个股。
// 沪市：600/601/603/605（主板）、688/689（科创板）。
// 深市：000/001/002/003（主板/中小板）、300/301（创业板）。
func isAStock(mkt uint8, code string) bool {
	if len(code) != 6 {
		return false
	}
	switch mkt {
	case types.MarketSH.Uint8():
		return strings.HasPrefix(code, "60") ||
			strings.HasPrefix(code, "688") ||
			strings.HasPrefix(code, "689")
	case types.MarketSZ.Uint8():
		return strings.HasPrefix(code, "000") ||
			strings.HasPrefix(code, "001") ||
			strings.HasPrefix(code, "002") ||
			strings.HasPrefix(code, "003") ||
			strings.HasPrefix(code, "300") ||
			strings.HasPrefix(code, "301")
	}
	return false
}

func boardOf(mkt uint8, code string) string {
	switch mkt {
	case types.MarketSH.Uint8():
		if strings.HasPrefix(code, "688") || strings.HasPrefix(code, "689") {
			return "科创板"
		}
		return "主板"
	case types.MarketSZ.Uint8():
		if strings.HasPrefix(code, "300") || strings.HasPrefix(code, "301") {
			return "创业板"
		}
		return "主板"
	}
	return ""
}

func marketOf(code string) uint8 {
	if len(code) > 0 && (code[0] == '6' || code[0] == '5') {
		return types.MarketSH.Uint8()
	}
	return types.MarketSZ.Uint8()
}

func indexNameOf(code string) string {
	switch code {
	case "000001":
		return "上证指数"
	case "399001":
		return "深证成指"
	case "399006":
		return "创业板指"
	default:
		return code
	}
}

// lookupNames 返回代码→名称映射；首次调用时懒加载全市场证券名索引。
func (p *GotdxProvider) lookupNames(codes []string) map[string]string {
	p.ensureNameIndex()
	p.nameMu.RLock()
	defer p.nameMu.RUnlock()
	out := make(map[string]string, len(codes))
	for _, code := range codes {
		if n, ok := p.nameIndex[code]; ok && n != "" {
			out[code] = n
		}
	}
	return out
}

func (p *GotdxProvider) ensureNameIndex() {
	p.nameMu.RLock()
	if len(p.nameIndex) > 0 {
		p.nameMu.RUnlock()
		return
	}
	p.nameMu.RUnlock()

	list, err := p.StockList(context.Background())
	if err != nil {
		return
	}
	idx := make(map[string]string, len(list))
	for _, s := range list {
		if s.Name != "" {
			idx[s.Code] = s.Name
		}
	}

	p.nameMu.Lock()
	defer p.nameMu.Unlock()
	if len(p.nameIndex) == 0 {
		p.nameIndex = idx
	}
}
