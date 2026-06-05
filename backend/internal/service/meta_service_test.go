package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/integration/market"
	"github.com/warden-stock/warden-stock-data/internal/service"
)

func TestFreshnessProviderFallback(t *testing.T) {
	svc := service.NewMetaService(market.NewStubProvider(), nil, nil, nil, nil)
	f, err := svc.Freshness(context.Background(), "CN")
	require.NoError(t, err)
	require.Equal(t, "CN", f.Market)
	require.Equal(t, "stub", f.ProviderSource)
	require.Equal(t, int64(3), f.SecuritiesCount)
	require.NotEmpty(t, f.LatestTradeDate)
	require.Equal(t, f.LatestTradeDate, f.KlineUpdatedTo)
	require.Empty(t, f.LastScanAt)
}
