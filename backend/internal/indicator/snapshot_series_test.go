package indicator_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
)

// TestComputeSnapshotSeriesMatchesPrefix 验证流式 O(n) 结果与逐日前缀重算（旧回补口径）一致。
func TestComputeSnapshotSeriesMatchesPrefix(t *testing.T) {
	s := risingCloses(120)
	types := indicator.DefaultSnapshotTypes
	fast := indicator.ComputeSnapshotSeries(s, types)

	for _, idx := range []int{34, 59, 89, 119} {
		prefix := indicator.Series{Bars: s.Bars[:idx+1]}
		for _, typ := range types {
			want, err := indicator.Compute(typ, prefix, nil)
			if err != nil {
				if fast[idx] == nil {
					continue
				}
				_, ok := fast[idx][typ]
				require.False(t, ok, "index=%d type=%s should be unavailable", idx, typ)
				continue
			}
			got, ok := fast[idx][typ]
			require.True(t, ok, "index=%d type=%s missing", idx, typ)
			require.True(t, want.Sub(got).Abs().LessThan(decimal.NewFromFloat(0.0001)),
				"index=%d type=%s want=%s got=%s", idx, typ, want, got)
		}
	}
}

func BenchmarkComputeSnapshotSeries(b *testing.B) {
	s := risingCloses(8000)
	types := indicator.DefaultSnapshotTypes
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = indicator.ComputeSnapshotSeries(s, types)
	}
}

func BenchmarkStreamSnapshotSeries(b *testing.B) {
	s := risingCloses(8000)
	types := indicator.DefaultSnapshotTypes
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = indicator.StreamSnapshotSeries(context.Background(), s, types, func(int, map[string]decimal.Decimal) error {
			return nil
		})
	}
}

func BenchmarkComputeSnapshotSeriesLegacy(b *testing.B) {
	s := risingCloses(8000)
	types := indicator.DefaultSnapshotTypes
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range s.Bars {
			_, _ = indicator.ComputeAll(indicator.Series{Bars: s.Bars[:j+1]}, types)
		}
	}
}
