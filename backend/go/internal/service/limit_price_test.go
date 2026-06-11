package service

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBoardOf(t *testing.T) {
	cases := map[string]string{
		"600000": boardMain, "000001": boardMain, "002415": boardMain, "001358": boardMain,
		"300750": boardGEM, "301000": boardGEM,
		"688981": boardSTAR, "689009": boardSTAR,
		"830799": boardBSE, "430047": boardBSE, "920099": boardBSE,
		"sh.600519": boardMain, "sz.300750": boardGEM,
	}
	for code, want := range cases {
		require.Equalf(t, want, boardOf(code), "code=%s", code)
	}
}

func TestComputeLimitPrices(t *testing.T) {
	// 主板非 ST ±10%
	up, down, ok := computeLimitPrices("600000", decimal.NewFromFloat(10), false, false)
	require.True(t, ok)
	require.True(t, up.Equal(decimal.NewFromFloat(11)))
	require.True(t, down.Equal(decimal.NewFromFloat(9)))

	// 主板 ST ±5%
	up, down, ok = computeLimitPrices("600000", decimal.NewFromFloat(10), true, false)
	require.True(t, ok)
	require.True(t, up.Equal(decimal.NewFromFloat(10.5)))
	require.True(t, down.Equal(decimal.NewFromFloat(9.5)))

	// 创业板 ±20%，ST 同 20%
	up, down, _ = computeLimitPrices("300750", decimal.NewFromFloat(100), true, false)
	require.True(t, up.Equal(decimal.NewFromFloat(120)))
	require.True(t, down.Equal(decimal.NewFromFloat(80)))

	// 北交所 ±30%
	up, down, _ = computeLimitPrices("830799", decimal.NewFromFloat(10), false, false)
	require.True(t, up.Equal(decimal.NewFromFloat(13)))
	require.True(t, down.Equal(decimal.NewFromFloat(7)))

	// 四舍五入到分
	up, _, _ = computeLimitPrices("600000", decimal.NewFromFloat(10.005), false, false)
	require.Equal(t, "11.01", up.StringFixed(2))

	// 新股首日不设限
	_, _, ok = computeLimitPrices("600000", decimal.NewFromFloat(10), false, true)
	require.False(t, ok)

	// 无效昨收
	_, _, ok = computeLimitPrices("600000", decimal.Zero, false, false)
	require.False(t, ok)
}
