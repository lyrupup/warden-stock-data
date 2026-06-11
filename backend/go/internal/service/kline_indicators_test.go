package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/model"
	"github.com/warden-stock/warden-stock-data/internal/service"
)

func TestKlineIndicatorsEmpty(t *testing.T) {
	svc := service.NewIndicatorService(nil)
	require.Nil(t, svc.KlineIndicators(context.Background(), "600000", "day", "qfq", nil, []string{"ma5"}, 0, 0, nil, nil))
	bars := []model.StockDailyKline{{StockCode: "600000"}}
	require.Nil(t, svc.KlineIndicators(context.Background(), "600000", "day", "qfq", bars, nil, 0, 0, nil, nil))
}
