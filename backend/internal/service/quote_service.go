package service

import (
	"context"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/repository"
	"github.com/warden-stock/warden-stock-data/pkg/cache"
	"github.com/warden-stock/warden-stock-data/pkg/errcode"
)

// 指数行情变化频率低于个股，缓存 TTL 适当加长以减少 TDX 冷连接。
const indicesCacheTTL = 5 * time.Minute

type QuoteService struct {
	provider   market.IMarketProvider
	quoteRepo  *repository.QuoteRepository
	secRepo    *repository.SecurityRepository
	quoteCache *cache.QuoteCache
	market     string
	indicesSF  singleflight.Group
}

func NewQuoteService(
	provider market.IMarketProvider,
	quoteRepo *repository.QuoteRepository,
	secRepo *repository.SecurityRepository,
	quoteCache *cache.QuoteCache,
) *QuoteService {
	return &QuoteService{
		provider: provider, quoteRepo: quoteRepo, secRepo: secRepo,
		quoteCache: quoteCache, market: "CN",
	}
}

func (s *QuoteService) Indices(ctx context.Context, marketCode string) ([]model.IndexQuote, error) {
	if marketCode == "" {
		marketCode = s.market
	}
	key := cache.QuoteKey(marketCode, "indices", "all")
	var cached []model.IndexQuote
	if s.quoteCache != nil {
		if ok, _ := s.quoteCache.Get(ctx, key, &cached); ok && len(cached) > 0 {
			return cached, nil
		}
	}
	// Redis 未命中时优先读库快速返回，后台刷新 TDX（避免每次新建连接 ~4s 阻塞首页）。
	if s.quoteRepo != nil {
		stale, err := s.quoteRepo.LatestIndexQuotes(ctx, marketCode)
		if err == nil && len(stale) > 0 {
			go s.refreshIndices(marketCode)
			return stale, nil
		}
	}
	quotes, err := s.fetchAndCacheIndices(ctx, marketCode)
	if err != nil {
		return nil, errcodeBiz{code: errcode.ErrProvider}
	}
	return quotes, nil
}

func (s *QuoteService) refreshIndices(marketCode string) {
	s.indicesSF.Do(marketCode, func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := s.fetchAndCacheIndices(ctx, marketCode)
		return nil, err
	})
}

func (s *QuoteService) fetchAndCacheIndices(ctx context.Context, marketCode string) ([]model.IndexQuote, error) {
	quotes, err := s.provider.Indices(ctx)
	if err != nil {
		return nil, err
	}
	if s.quoteRepo != nil {
		_ = s.quoteRepo.SaveIndexQuotes(ctx, quotes)
	}
	if s.quoteCache != nil {
		key := cache.QuoteKey(marketCode, "indices", "all")
		_ = s.quoteCache.SetWithTTL(ctx, key, quotes, indicesCacheTTL)
	}
	return quotes, nil
}

func (s *QuoteService) Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error) {
	if len(codes) == 0 {
		return nil, errcodeBiz{code: errcode.ErrParam}
	}
	marketCode := s.market
	key := cache.QuotesKey(marketCode, strings.Join(codes, ","))
	var cached []model.StockQuote
	if s.quoteCache != nil {
		if ok, _ := s.quoteCache.Get(ctx, key, &cached); ok {
			s.enrichQuoteNames(ctx, cached)
			return cached, nil
		}
	}
	quotes, err := s.provider.Quotes(ctx, codes)
	if err != nil {
		if s.quoteRepo != nil {
			stale, dbErr := s.quoteRepo.LatestByCodes(ctx, marketCode, codes)
			if dbErr == nil && len(stale) > 0 {
				s.enrichQuoteNames(ctx, stale)
				for i := range stale {
					stale[i].Stale = true
				}
				return stale, nil
			}
		}
		return nil, errcodeBiz{code: errcode.ErrProvider}
	}
	s.enrichQuoteNames(ctx, quotes)
	if s.quoteCache != nil {
		_ = s.quoteCache.Set(ctx, key, quotes)
	}
	return quotes, nil
}

// enrichQuoteNames 用本地 securities 表补全 stock_name。
// gotdx 行情接口不含名称，provider 层会回退为代码，此处以库内证券名为准。
func (s *QuoteService) enrichQuoteNames(ctx context.Context, quotes []model.StockQuote) {
	if s.secRepo == nil || len(quotes) == 0 {
		return
	}
	codes := make([]string, len(quotes))
	for i, q := range quotes {
		codes[i] = q.StockCode
	}
	names, err := s.secRepo.NamesByCodes(ctx, s.market, codes)
	if err != nil {
		return
	}
	enrichQuoteNames(quotes, names)
}

func enrichQuoteNames(quotes []model.StockQuote, names map[string]string) {
	for i := range quotes {
		if n := names[quotes[i].StockCode]; n != "" {
			quotes[i].StockName = n
		}
	}
}

func (s *QuoteService) Quote(ctx context.Context, code string) (*model.StockQuote, error) {
	quotes, err := s.Quotes(ctx, []string{code})
	if err != nil {
		return nil, err
	}
	if len(quotes) == 0 {
		return nil, errcodeBiz{code: errcode.ErrStockNotFound}
	}
	return &quotes[0], nil
}

// StockBrief 对齐 openapi StockBrief 契约（stock_code/stock_name/market/board）。
type StockBrief struct {
	StockCode string `json:"stock_code"`
	StockName string `json:"stock_name"`
	Market    string `json:"market"`
	Board     string `json:"board"`
}

func toBriefs(secs []model.Security) []StockBrief {
	out := make([]StockBrief, 0, len(secs))
	for _, s := range secs {
		out = append(out, StockBrief{
			StockCode: s.Code, StockName: s.Name, Market: s.Market, Board: s.Board,
		})
	}
	return out
}

func (s *QuoteService) Search(ctx context.Context, kw string) ([]StockBrief, error) {
	kw = strings.TrimSpace(kw)
	if kw == "" {
		return nil, errcodeBiz{code: errcode.ErrParam}
	}
	// 优先查本地 securities 表（trgm 索引，毫秒级），命中即返回。
	if s.secRepo != nil {
		if secs, err := s.secRepo.Search(ctx, s.market, kw); err == nil && len(secs) > 0 {
			return toBriefs(secs), nil
		}
	}
	// 本地无数据时回源行情提供方（首次启动、securities 尚未同步等场景）。
	secs, err := s.provider.Search(ctx, kw)
	if err != nil {
		return nil, err
	}
	return toBriefs(secs), nil
}

func (s *QuoteService) Securities(ctx context.Context) ([]StockBrief, error) {
	secs, err := s.provider.StockList(ctx)
	if err != nil {
		return nil, err
	}
	return toBriefs(secs), nil
}

func (s *QuoteService) HealthCheck(ctx context.Context) error {
	return s.provider.HealthCheck(ctx)
}
