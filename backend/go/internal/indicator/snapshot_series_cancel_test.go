package indicator_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
)

func TestStreamSnapshotSeriesCanceledBeforeStart(t *testing.T) {
	s := risingCloses(8000)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var emitted int
	err := indicator.StreamSnapshotSeries(ctx, s, indicator.DefaultSnapshotTypes, func(int, map[string]decimal.Decimal) error {
		emitted++
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 0, emitted)
}

func TestStreamSnapshotSeriesCanceledMidStream(t *testing.T) {
	s := risingCloses(8000)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var emitted int
	err := indicator.StreamSnapshotSeries(ctx, s, indicator.DefaultSnapshotTypes, func(i int, _ map[string]decimal.Decimal) error {
		emitted++
		if i > 50 {
			cancel()
		}
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, emitted, 8000)
}
