package indicator_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
)

func TestBiasAndMaAlign(t *testing.T) {
	bars := buildBars([]float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70})
	bias, err := indicator.Compute("bias5", bars, nil)
	require.NoError(t, err)
	require.True(t, bias.GreaterThan(decimal.Zero))

	align, err := indicator.Compute("ma_align", bars, nil)
	require.NoError(t, err)
	require.True(t, align.Equal(decimal.NewFromInt(1)))
}

func barsWithSpread(closes []float64) indicator.Series {
	base := buildBars(closes)
	for i := range base.Bars {
		base.Bars[i].High = base.Bars[i].Close.Add(decimal.NewFromFloat(0.5))
		base.Bars[i].Low = base.Bars[i].Close.Sub(decimal.NewFromFloat(0.3))
	}
	return base
}

func TestAmplitudeAndPctChange(t *testing.T) {
	bars := barsWithSpread([]float64{10, 10.5, 11, 11.5, 12})
	amp, err := indicator.Compute("amplitude", bars, nil)
	require.NoError(t, err)
	require.True(t, amp.GreaterThan(decimal.Zero))

	pct, err := indicator.Compute("pct_change", bars, nil)
	require.NoError(t, err)
	require.True(t, pct.GreaterThan(decimal.Zero))
}
