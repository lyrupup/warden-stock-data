package indicator_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
)

func buildBars(closes []float64) indicator.Series {
	bars := make([]indicator.Bar, len(closes))
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, c := range closes {
		d := decimal.NewFromFloat(c)
		bars[i] = indicator.Bar{
			TradeDate: base.AddDate(0, 0, i),
			Close:     d,
			Open:      d,
			High:      d,
			Low:       d,
		}
	}
	return indicator.Series{Bars: bars}
}

func TestMA(t *testing.T) {
	bars := buildBars([]float64{10, 11, 12, 13, 14})
	cases := []struct {
		period int
		want   string
		err    bool
	}{
		{5, "12", false},
		{6, "", true},
	}
	for _, c := range cases {
		v, err := indicator.MA(bars, c.period)
		if c.err {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, c.want, v.StringFixed(0))
	}
}

func TestComputeRegistered(t *testing.T) {
	bars := buildBars([]float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
	vals, err := indicator.ComputeAll(bars, []string{"ma5", "ma10"})
	require.NoError(t, err)
	require.Contains(t, vals, "ma5")
	require.Contains(t, vals, "ma10")
}

func TestUnknownIndicator(t *testing.T) {
	_, err := indicator.ComputeAll(buildBars([]float64{1}), []string{"unknown"})
	require.Error(t, err)
}
