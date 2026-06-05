package market

import (
	"testing"

	"github.com/bensema/gotdx/types"
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
