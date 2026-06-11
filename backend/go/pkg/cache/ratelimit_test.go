package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/pkg/cache"
)

func TestMemoryRateLimiter(t *testing.T) {
	lim := cache.NewMemoryRateLimiter()
	ctx := context.Background()
	key := "test"
	for i := 0; i < 5; i++ {
		ok, err := lim.Allow(ctx, key, 5, time.Second)
		require.NoError(t, err)
		require.True(t, ok)
	}
	ok, err := lim.Allow(ctx, key, 5, time.Second)
	require.NoError(t, err)
	require.False(t, ok)
}
