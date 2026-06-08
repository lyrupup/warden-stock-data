package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/service"
)

// windowBars 分页切片：跳过最近 offset 根、取 limit 根，并正确判定 hasMore。
func TestWindowBars(t *testing.T) {
	bars := buildKlineBars([]float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}) // 10 根，升序

	// 第一页：最近 4 根（下标 6~9），左侧仍有更早数据。
	page, hasMore := service.WindowBarsForTest(bars, 0, 4)
	require.Len(t, page, 4)
	require.True(t, hasMore)
	require.Equal(t, "2024-01-07", page[0].TradeDate.Format("2006-01-02"))
	require.Equal(t, "2024-01-10", page[3].TradeDate.Format("2006-01-02"))

	// 第二页：跳过最近 4 根后再取 4 根（下标 2~5），左侧仍有更早数据。
	page, hasMore = service.WindowBarsForTest(bars, 4, 4)
	require.Len(t, page, 4)
	require.True(t, hasMore)
	require.Equal(t, "2024-01-03", page[0].TradeDate.Format("2006-01-02"))
	require.Equal(t, "2024-01-06", page[3].TradeDate.Format("2006-01-02"))

	// 最后一页：剩余不足 limit，hasMore=false。
	page, hasMore = service.WindowBarsForTest(bars, 8, 4)
	require.Len(t, page, 2)
	require.False(t, hasMore)
	require.Equal(t, "2024-01-01", page[0].TradeDate.Format("2006-01-02"))
	require.Equal(t, "2024-01-02", page[1].TradeDate.Format("2006-01-02"))

	// offset 越过全部数据：返回空且 hasMore=false。
	page, hasMore = service.WindowBarsForTest(bars, 10, 4)
	require.Empty(t, page)
	require.False(t, hasMore)

	// limit 覆盖全部：一页拉完，hasMore=false。
	page, hasMore = service.WindowBarsForTest(bars, 0, 50)
	require.Len(t, page, 10)
	require.False(t, hasMore)
}

// 无 DB 仓储时 KlinePage 走 provider 回源并按窗口切片，hasMore 判定正确。
func TestKlinePageFromProvider(t *testing.T) {
	svc := service.NewKlineService(newFakeProvider(), nil)
	ctx := context.Background()

	// fakeProvider 返回 120 根升序日 K。第一页最近 50 根，左侧仍有更早数据。
	bars, hasMore, err := svc.KlinePage(ctx, service.KlineQuery{Code: "600000", Period: "day", Adjust: "qfq", Limit: 50})
	require.NoError(t, err)
	require.Len(t, bars, 50)
	require.True(t, hasMore)

	// 翻到接近最旧：offset 100、limit 50 → 仅剩 20 根，hasMore=false。
	bars, hasMore, err = svc.KlinePage(ctx, service.KlineQuery{Code: "600000", Period: "day", Adjust: "qfq", Limit: 50, Offset: 100})
	require.NoError(t, err)
	require.Len(t, bars, 20)
	require.False(t, hasMore)

	// offset 越过全部：返回空且 hasMore=false。
	bars, hasMore, err = svc.KlinePage(ctx, service.KlineQuery{Code: "600000", Period: "day", Adjust: "qfq", Limit: 50, Offset: 200})
	require.NoError(t, err)
	require.Empty(t, bars)
	require.False(t, hasMore)
}
