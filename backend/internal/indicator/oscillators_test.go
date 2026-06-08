package indicator_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
)

// ohlcBars 按 [open, high, low, close] 行构造带高低价差的 K 线序列。
func ohlcBars(rows [][4]float64) indicator.Series {
	bars := make([]indicator.Bar, len(rows))
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, r := range rows {
		bars[i] = indicator.Bar{
			TradeDate: base.AddDate(0, 0, i),
			Open:      decimal.NewFromFloat(r[0]),
			High:      decimal.NewFromFloat(r[1]),
			Low:       decimal.NewFromFloat(r[2]),
			Close:     decimal.NewFromFloat(r[3]),
			Volume:    decimal.NewFromInt(1000),
		}
	}
	return indicator.Series{Bars: bars}
}

func constCloses(v float64, n int) indicator.Series {
	closes := make([]float64, n)
	for i := range closes {
		closes[i] = v
	}
	return buildBars(closes)
}

func risingCloses(n int) indicator.Series {
	closes := make([]float64, n)
	for i := range closes {
		closes[i] = float64(10 + i)
	}
	return buildBars(closes)
}

func TestMACD(t *testing.T) {
	// 常数价格：EMA 快慢线相等，DIF/DEA/柱 均为 0。
	flat := constCloses(10, 50)
	for _, typ := range []string{"macd_dif", "macd_dea", "macd_bar"} {
		v, err := indicator.Compute(typ, flat, nil)
		require.NoError(t, err)
		require.True(t, v.Abs().LessThan(decimal.NewFromFloat(0.0001)), "%s should be ~0, got %s", typ, v)
	}

	// 上升趋势：快线在慢线之上，DIF > 0。
	up := risingCloses(60)
	dif, err := indicator.Compute("macd_dif", up, nil)
	require.NoError(t, err)
	require.True(t, dif.IsPositive(), "uptrend DIF should be positive, got %s", dif)

	// 柱 = (DIF - DEA) * 2。
	dea, _ := indicator.Compute("macd_dea", up, nil)
	bar, _ := indicator.Compute("macd_bar", up, nil)
	require.True(t, bar.Sub(dif.Sub(dea).Mul(decimal.NewFromInt(2))).Abs().LessThan(decimal.NewFromFloat(0.0001)))

	// 数据不足。
	_, err = indicator.Compute("macd_bar", constCloses(10, 10), nil)
	require.ErrorIs(t, err, indicator.ErrInsufficientData)
}

func TestRSI(t *testing.T) {
	// 单调上涨：无下跌，RSI = 100。
	up, err := indicator.RSI(risingCloses(20), 6)
	require.NoError(t, err)
	require.Equal(t, "100", up.StringFixed(0))

	// 单调下跌：无上涨，RSI = 0。
	down := buildBars([]float64{20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10})
	v, err := indicator.RSI(down, 6)
	require.NoError(t, err)
	require.Equal(t, "0", v.StringFixed(0))

	// 注册类型可用，且取值在 [0,100]。
	mixed := buildBars([]float64{10, 11, 10.5, 11.5, 11, 12, 11.8, 12.5, 12, 13, 12.7, 13.4, 13, 13.8, 13.2})
	r12, err := indicator.Compute("rsi12", mixed, nil)
	require.NoError(t, err)
	require.True(t, r12.GreaterThanOrEqual(decimal.Zero) && r12.LessThanOrEqual(decimal.NewFromInt(100)))

	// 数据不足。
	_, err = indicator.RSI(buildBars([]float64{10, 11}), 6)
	require.ErrorIs(t, err, indicator.ErrInsufficientData)
}

func TestKDJ(t *testing.T) {
	// 收盘贴近最高的强势上行：K 应明显高于 50。
	rows := make([][4]float64, 0, 15)
	for i := 0; i < 15; i++ {
		base := float64(10 + i)
		rows = append(rows, [4]float64{base, base + 0.2, base - 0.5, base + 0.15})
	}
	s := ohlcBars(rows)
	k, err := indicator.Compute("kdj_k", s, nil)
	require.NoError(t, err)
	require.True(t, k.GreaterThan(decimal.NewFromInt(50)), "strong uptrend K should be >50, got %s", k)

	// J = 3K - 2D。
	d, _ := indicator.Compute("kdj_d", s, nil)
	j, _ := indicator.Compute("kdj_j", s, nil)
	want := k.Mul(decimal.NewFromInt(3)).Sub(d.Mul(decimal.NewFromInt(2)))
	require.True(t, j.Sub(want).Abs().LessThan(decimal.NewFromFloat(0.0001)))

	// 数据不足。
	_, err = indicator.Compute("kdj_k", constCloses(10, 5), nil)
	require.ErrorIs(t, err, indicator.ErrInsufficientData)
}

func TestBOLL(t *testing.T) {
	// 常数价格：标准差为 0，上中下轨重合。
	flat := constCloses(10, 25)
	mid, _ := indicator.Compute("boll_mid", flat, nil)
	up, _ := indicator.Compute("boll_upper", flat, nil)
	low, _ := indicator.Compute("boll_lower", flat, nil)
	require.Equal(t, "10.0000", mid.StringFixed(4))
	require.Equal(t, mid.StringFixed(4), up.StringFixed(4))
	require.Equal(t, mid.StringFixed(4), low.StringFixed(4))

	// 波动序列：上轨 > 中轨 > 下轨。
	vary := risingCloses(25)
	mid2, _ := indicator.Compute("boll_mid", vary, nil)
	up2, _ := indicator.Compute("boll_upper", vary, nil)
	low2, _ := indicator.Compute("boll_lower", vary, nil)
	require.True(t, up2.GreaterThan(mid2) && mid2.GreaterThan(low2))

	// 数据不足。
	_, err := indicator.Compute("boll_mid", constCloses(10, 10), nil)
	require.ErrorIs(t, err, indicator.ErrInsufficientData)
}

func TestATR(t *testing.T) {
	// 收盘恒定、每日高低各偏离 1：TR 恒为 2，ATR=2。
	rows := make([][4]float64, 0, 25)
	for i := 0; i < 25; i++ {
		rows = append(rows, [4]float64{10, 11, 9, 10})
	}
	s := ohlcBars(rows)
	atr, err := indicator.ATR(s, 14)
	require.NoError(t, err)
	require.Equal(t, "2.0000", atr.StringFixed(4))

	v, err := indicator.Compute("atr20", s, nil)
	require.NoError(t, err)
	require.True(t, v.IsPositive())

	// 数据不足。
	_, err = indicator.ATR(constCloses(10, 10), 14)
	require.ErrorIs(t, err, indicator.ErrInsufficientData)
}

func TestMomentumRegistered(t *testing.T) {
	up := risingCloses(70)
	for _, typ := range []string{"pct_change20", "pct_change60"} {
		v, err := indicator.Compute(typ, up, nil)
		require.NoError(t, err)
		require.True(t, v.IsPositive(), "%s should be positive in uptrend", typ)
	}
}

func TestExpandedCatalogImplemented(t *testing.T) {
	cat := indicator.Catalog()
	want := map[string]bool{
		"macd_bar": false, "kdj_k": false, "rsi6": false,
		"boll_upper": false, "atr14": false,
	}
	for _, item := range cat {
		typ, _ := item["type"].(string)
		if _, ok := want[typ]; ok {
			require.Equal(t, true, item["implemented"], "%s should be implemented", typ)
			want[typ] = true
		}
	}
	for typ, found := range want {
		require.True(t, found, "catalog missing %s", typ)
	}
}
