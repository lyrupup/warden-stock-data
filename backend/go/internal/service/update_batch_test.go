package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/integration/quant"
	"github.com/warden-stock/warden-stock-data/internal/service"
)

// fakeQuant 为单元测试提供可控的 quant 采集返回，仅实现 CollectKline，其余方法置空。
type fakeQuant struct {
	resp *quant.CollectKlineResponse
	err  error
	// lastReq 记录最近一次请求，便于断言合批的 codes。
	lastReq quant.CollectKlineRequest
	// calCount/calErr 控制 CollectCalendar 返回；calCalled 记录是否被调用；calFrom/calTo 记录透传区间。
	calCount  int
	calErr    error
	calCalled bool
	calFrom   string
	calTo     string
}

func (f *fakeQuant) Health(context.Context) error                 { return nil }
func (f *fakeQuant) Catalog(context.Context) (*quant.CatalogResponse, error) {
	return &quant.CatalogResponse{}, nil
}
func (f *fakeQuant) CollectSecurities(context.Context, string) (int, error) { return 0, nil }
func (f *fakeQuant) CollectCalendar(_ context.Context, _, fromDate, toDate string) (int, error) {
	f.calCalled = true
	f.calFrom = fromDate
	f.calTo = toDate
	return f.calCount, f.calErr
}
func (f *fakeQuant) CollectKline(_ context.Context, req quant.CollectKlineRequest) (*quant.CollectKlineResponse, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}
func (f *fakeQuant) BatchIndicators(context.Context, quant.IndicatorRequest) (*quant.IndicatorResponse, error) {
	return &quant.IndicatorResponse{}, nil
}
func (f *fakeQuant) SeriesIndicators(context.Context, quant.IndicatorSeriesRequest) (*quant.IndicatorSeriesResponse, error) {
	return &quant.IndicatorSeriesResponse{}, nil
}

// full 模式合批：所有代码一次请求；failed/缺失结果分别归类为错误，skipped 经回退判定为无 K 线错误。
func TestCollectKlineBatchClassify(t *testing.T) {
	fq := &fakeQuant{resp: &quant.CollectKlineResponse{Results: []quant.CollectResult{
		{Code: "600519", Status: "failed", Reason: "boom"},
		{Code: "000001", Status: "skipped"},
		// 故意不返回 "300750" 的结果，验证「缺失结果」归类
	}}}
	svc := service.NewUpdateService(newFakeProvider(), fq, nil, nil, nil, nil)

	out, err := svc.CollectKlineBatch(context.Background(), []string{"600519", "000001", "300750"}, "full", "", "", nil)
	require.NoError(t, err)

	// full 模式：一次请求带全部代码。
	require.ElementsMatch(t, []string{"600519", "000001", "300750"}, fq.lastReq.Codes)
	require.Equal(t, "full", fq.lastReq.Mode)

	require.EqualError(t, out["600519"], "boom")
	// fakeProvider 现价>0 → 非未上市 → skipped 归类为「无可用日K」。
	require.ErrorIs(t, out["000001"], service.ErrNoKline)
	require.Error(t, out["300750"]) // 缺失结果
}

// 显式区间：所有代码统一以 fromOverride 合批（忽略各自水位），并透传 to_date。
// 用 failed 结果避免触达需要 DB 仓储的水位推进逻辑，专注断言请求分组与日期透传。
func TestCollectKlineBatchExplicitRange(t *testing.T) {
	fq := &fakeQuant{resp: &quant.CollectKlineResponse{Results: []quant.CollectResult{
		{Code: "600519", Status: "failed", Reason: "x"},
		{Code: "000001", Status: "failed", Reason: "x"},
	}}}
	svc := service.NewUpdateService(newFakeProvider(), fq, nil, nil, nil, nil)

	_, err := svc.CollectKlineBatch(
		context.Background(), []string{"600519", "000001"}, "incremental", "2026-06-10", "2026-06-10", nil,
	)
	require.NoError(t, err)
	// 显式区间下两只代码合为单组单次请求，且透传 from/to。
	require.ElementsMatch(t, []string{"600519", "000001"}, fq.lastReq.Codes)
	require.Equal(t, "2026-06-10", fq.lastReq.FromDate)
	require.Equal(t, "2026-06-10", fq.lastReq.ToDate)
}

// SyncCalendar：经 quant 拉日历，透传写入天数。
func TestSyncCalendar(t *testing.T) {
	fq := &fakeQuant{calCount: 1234}
	svc := service.NewUpdateService(newFakeProvider(), fq, nil, nil, nil, nil)

	n, err := svc.SyncCalendar(context.Background(), "2026-01-01", "2026-12-31")
	require.NoError(t, err)
	require.True(t, fq.calCalled)
	require.Equal(t, 1234, n)
	// 透传日期区间到 quant。
	require.Equal(t, "2026-01-01", fq.calFrom)
	require.Equal(t, "2026-12-31", fq.calTo)
}

// SyncCalendar：quant 不可用时返回错误。
func TestSyncCalendarQuantUnavailable(t *testing.T) {
	svc := service.NewUpdateService(newFakeProvider(), nil, nil, nil, nil, nil)
	_, err := svc.SyncCalendar(context.Background(), "", "")
	require.ErrorIs(t, err, service.ErrQuantUnavailable)
}

// 传输级错误：返回外层 error，调用方据此整批归类。
func TestCollectKlineBatchTransportError(t *testing.T) {
	fq := &fakeQuant{err: context.DeadlineExceeded}
	svc := service.NewUpdateService(newFakeProvider(), fq, nil, nil, nil, nil)

	_, err := svc.CollectKlineBatch(context.Background(), []string{"600519"}, "full", "", "", nil)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
