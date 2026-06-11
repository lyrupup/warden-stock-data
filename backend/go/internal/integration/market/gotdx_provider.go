package market

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/bensema/gotdx"
	"github.com/bensema/gotdx/proto"
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
	catalog := cnIndexCatalog()
	codes := make([]string, len(catalog))
	mkts := make([]uint8, len(catalog))
	for i, idx := range catalog {
		codes[i] = idx.code
		mkts[i] = idx.market
	}
	var out []model.IndexQuote
	err := p.withClient(ctx, func(c *gotdx.Client) error {
		list, err := c.StockQuotes(mkts, codes)
		if err != nil {
			return err
		}
		now := time.Now()
		today := now.Truncate(24 * time.Hour)
		// 数据源返回顺序不保证与请求一致，按代码建索引后，按目录顺序稳定输出。
		byCode := make(map[string]proto.QuoteListItem, len(list))
		for _, q := range list {
			byCode[q.Code] = q
		}
		for _, idx := range catalog {
			q, ok := byCode[idx.code]
			if !ok {
				continue
			}
			out = append(out, mapIndexQuote(q, idx.name, today, now))
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
		raw, err := fetchKlinePage(c, klineType, mkt, code, 0, klinePageSize)
		if err != nil {
			return err
		}
		bars = mapSecurityBars(raw, code, adjust)
		return nil
	})
	return bars, err
}

// KlineFull 分页拉取数据源全部历史 K 线（日/周/月），供全量回补作业使用。
func (p *GotdxProvider) KlineFull(ctx context.Context, code, period, adjust string) ([]model.StockDailyKline, error) {
	klineType, err := klineTypeOf(period)
	if err != nil {
		return nil, err
	}
	var bars []model.StockDailyKline
	err = p.withClient(ctx, func(c *gotdx.Client) error {
		mkt := marketOf(code)
		var ferr error
		bars, ferr = fetchKlineAll(ctx, c, klineType, mkt, code, adjust)
		return ferr
	})
	return bars, err
}

// Intraday 拉取分时走势（实时透传）。分时图本身只含价格/均价/成交量、无时间戳，
// 这里按索引推算交易分钟；同时取一次盘口快照补全昨收（涨跌基准线）。
// 最近交易日由日 K 最后一根反推：当日分时为空（非交易日/盘前）时，回退到该交易日的历史分时。
func (p *GotdxProvider) Intraday(ctx context.Context, code string) (model.StockIntraday, error) {
	out := model.StockIntraday{Market: "CN", StockCode: code}
	var tradeDate time.Time
	err := p.withClient(ctx, func(c *gotdx.Client) error {
		mkt := marketOf(code)
		tradeDate = p.latestTradeDate(c, mkt, code)

		points, err := c.StockTickChart(mkt, code, 0, types.DefaultTickChartCount)
		if err != nil {
			return err
		}
		if len(points) > 0 {
			out.Points = mapIntradayPoints(points, tradeDate)
		} else {
			// 当日无分时（非交易日 / 盘前），回退最近交易日历史分时。
			dateInt := uint32(tradeDate.Year()*10000 + int(tradeDate.Month())*100 + tradeDate.Day())
			hist, herr := c.StockHistoryTickChart(dateInt, mkt, code)
			if herr != nil {
				return herr
			}
			out.Points = mapHistoryIntradayPoints(hist, tradeDate)
		}
		if qs, qerr := c.StockQuotesDetail([]uint8{mkt}, []string{code}); qerr == nil && len(qs) > 0 {
			out.PreClose = priceFromFloat(qs[0].PreClose)
		}
		return nil
	})
	if err != nil {
		return model.StockIntraday{}, err
	}
	out.TradeDate = tradeDate.Format("2006-01-02")
	return out, nil
}

// latestTradeDate 以日 K 最后一根的日期作为最近交易日；取不到时退回当前东八区日期。
func (p *GotdxProvider) latestTradeDate(c *gotdx.Client, mkt uint8, code string) time.Time {
	if bars, err := c.GetSecurityBars(types.KLINE_TYPE_RI_K, mkt, code, 0, 2); err == nil && bars != nil && len(bars.List) > 0 {
		b := bars.List[len(bars.List)-1]
		if !b.DateTime.IsZero() {
			return b.DateTime
		}
		return time.Date(b.Year, time.Month(b.Month), b.Day, 0, 0, 0, 0, cnLoc)
	}
	return time.Now().In(cnLoc)
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

// cnIndex 描述一只 A 股大盘指数的代码、所属市场与展示名。
// 指数代码与个股代码的市场归属规则不同，需显式声明市场，不能用 marketOf 前缀推断。
type cnIndex struct {
	code   string
	market uint8
	name   string
}

// cnIndexCatalog 返回对外展示的大盘核心指数目录（沪 / 深 / 北）。
// 覆盖宽基与板块代表指数；后续新增指数只需在此追加。
func cnIndexCatalog() []cnIndex {
	sh := types.MarketSH.Uint8()
	sz := types.MarketSZ.Uint8()
	bj := types.MarketBJ.Uint8()
	return []cnIndex{
		{"000001", sh, "上证指数"},
		{"000016", sh, "上证50"},
		{"000300", sh, "沪深300"},
		{"000680", sh, "科创综指"},
		{"000688", sh, "科创50"},
		{"000905", sh, "中证500"},
		{"000852", sh, "中证1000"},
		{"399001", sz, "深证成指"},
		{"399006", sz, "创业板指"},
		{"399005", sz, "中小100"},
		{"399330", sz, "深证100"},
		{"399673", sz, "创业板50"},
		{"899050", bj, "北证50"},
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
