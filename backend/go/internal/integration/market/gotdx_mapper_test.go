package market

import (
	"testing"
	"time"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestKlineTypeOf(t *testing.T) {
	cases := []struct {
		period string
		want   uint16
		err    bool
	}{
		{"", types.KLINE_TYPE_RI_K, false},
		{"day", types.KLINE_TYPE_RI_K, false},
		{"week", types.KLINE_TYPE_WEEKLY, false},
		{"month", types.KLINE_TYPE_MONTHLY, false},
		{"year", 0, true},
	}
	for _, c := range cases {
		got, err := klineTypeOf(c.period)
		if c.err {
			require.Error(t, err, c.period)
			continue
		}
		require.NoError(t, err, c.period)
		require.Equal(t, c.want, got, c.period)
	}
}

func TestIsAStock(t *testing.T) {
	sh := types.MarketSH.Uint8()
	sz := types.MarketSZ.Uint8()
	cases := []struct {
		mkt  uint8
		code string
		want bool
	}{
		{sh, "600519", true},  // 贵州茅台 主板
		{sh, "601318", true},  // 中国平安 主板
		{sh, "688981", true},  // 中芯国际 科创板
		{sh, "000001", false}, // 上证指数
		{sh, "510300", false}, // 沪市 ETF
		{sh, "113008", false}, // 可转债
		{sh, "204001", false}, // 国债逆回购
		{sz, "000001", true},  // 平安银行 主板
		{sz, "002594", true},  // 比亚迪 中小板
		{sz, "300750", true},  // 宁德时代 创业板
		{sz, "159915", false}, // 深市 ETF
		{sz, "128036", false}, // 深市可转债
		{sz, "399001", false}, // 深证成指
		{sh, "60051", false},  // 长度不足
	}
	for _, c := range cases {
		require.Equal(t, c.want, isAStock(c.mkt, c.code), "%d/%s", c.mkt, c.code)
	}
}

func TestChangeAmount(t *testing.T) {
	// 正常涨跌：现价 - 昨收。
	require.True(t, changeAmount(3030, 3000).Equal(decimal.NewFromInt(30)))
	require.True(t, changeAmount(2970, 3000).Equal(decimal.NewFromFloat(-30)))
	// 昨收缺失（停牌/数据异常）时不应输出伪涨跌额。
	require.True(t, changeAmount(3000, 0).Equal(decimal.Zero))
}

func TestMapIndexQuote(t *testing.T) {
	now := time.Date(2026, 6, 5, 15, 0, 0, 0, time.Local)
	today := now.Truncate(24 * time.Hour)
	q := proto.QuoteListItem{
		Code:     "000001",
		Price:    4027.74,
		PreClose: 4057.79,
		Vol:      123456,
		Amount:   987654321.5,
	}
	got := mapIndexQuote(q, "上证指数", today, now)

	require.Equal(t, "CN", got.Market)
	require.Equal(t, "000001", got.IndexCode)
	require.Equal(t, "上证指数", got.IndexName)
	require.True(t, got.Price.Equal(decimal.NewFromFloat(4027.74)))
	// 涨跌额与成交量额必须完整映射，不能为 0。
	require.True(t, got.ChangeAmount.Equal(decimal.NewFromFloat(4027.74-4057.79)))
	require.True(t, got.Volume.Equal(decimal.NewFromInt(123456)))
	require.True(t, got.Amount.Equal(decimal.NewFromFloat(987654321.5)))
	require.False(t, got.ChangePercent.IsZero())
	require.Equal(t, today, got.TradeDate)
	require.Equal(t, now, got.SnapshotAt)
}

func TestMapIndexQuoteFallbackName(t *testing.T) {
	got := mapIndexQuote(proto.QuoteListItem{Code: "999999"}, "", time.Now().Truncate(24*time.Hour), time.Now())
	require.Equal(t, "999999", got.IndexName)
}

func TestCNIndexCatalog(t *testing.T) {
	catalog := cnIndexCatalog()
	// 指数数量应明显多于早期的 3 个。
	require.GreaterOrEqual(t, len(catalog), 10)

	sh := types.MarketSH.Uint8()
	sz := types.MarketSZ.Uint8()
	bj := types.MarketBJ.Uint8()
	wantMarket := map[string]uint8{
		"000001": sh, "000300": sh, "000688": sh,
		"399001": sz, "399006": sz,
		"899050": bj,
	}
	seen := make(map[string]bool, len(catalog))
	for _, idx := range catalog {
		require.NotEmpty(t, idx.name, idx.code)
		require.False(t, seen[idx.code], "重复指数代码: %s", idx.code)
		seen[idx.code] = true
		if m, ok := wantMarket[idx.code]; ok {
			require.Equal(t, m, idx.market, "指数 %s 市场归属错误", idx.code)
		}
	}
}
