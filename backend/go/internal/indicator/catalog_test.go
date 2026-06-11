package indicator_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/warden-stock/warden-stock-data/internal/indicator"
)

// 目录应「注册即可见」：每个能计算的指标都必须出现在 catalog，且带非空名称与 value_type。
func TestCatalogReflectsRegistry(t *testing.T) {
	cat := indicator.Catalog()
	byType := make(map[string]map[string]interface{}, len(cat))
	for _, item := range cat {
		typ, _ := item["type"].(string)
		byType[typ] = item
		require.NotEmpty(t, item["name"], "%s missing name", typ)
		require.NotEmpty(t, item["value_type"], "%s missing value_type", typ)
		require.Equal(t, true, item["implemented"], "%s should be implemented", typ)
	}
	// 抽样校验关键指标存在（覆盖均线/经典指标/原始字段）。
	for _, typ := range []string{"ma5", "macd_bar", "kdj_k", "rsi6", "boll_upper", "atr14", "close", "volume"} {
		require.Contains(t, byType, typ, "catalog missing registered type %s", typ)
	}
}

// snapshot 标记必须与 DefaultSnapshotTypes 一致：默认快照集合内为 true，集合外为 false。
func TestCatalogSnapshotFlag(t *testing.T) {
	cat := indicator.Catalog()
	inSnapshot := make(map[string]bool, len(indicator.DefaultSnapshotTypes))
	for _, t := range indicator.DefaultSnapshotTypes {
		inSnapshot[t] = true
	}
	for _, item := range cat {
		typ, _ := item["type"].(string)
		require.Equal(t, inSnapshot[typ], item["snapshot"], "%s snapshot flag mismatch", typ)
	}
	// 典型断言：ma5/macd_bar 进快照；bias5/vol_ratio 不进快照（仅实时）。
	require.True(t, inSnapshot["ma5"])
	require.True(t, inSnapshot["macd_bar"])
	require.False(t, inSnapshot["bias5"])
	require.False(t, inSnapshot["vol_ratio"])
}

// 布尔型因子的 value_type 应为 bool。
func TestCatalogValueType(t *testing.T) {
	cat := indicator.Catalog()
	for _, item := range cat {
		if item["type"] == "ma_align" {
			require.Equal(t, "bool", item["value_type"])
			return
		}
	}
	t.Fatal("ma_align not found in catalog")
}

// DefaultSnapshotTypes 中的每个类型都必须是已注册可计算的指标（避免快照集合引用未实现指标）。
func TestDefaultSnapshotTypesAllRegistered(t *testing.T) {
	cat := indicator.Catalog()
	known := make(map[string]bool, len(cat))
	for _, item := range cat {
		known[item["type"].(string)] = true
	}
	require.NotEmpty(t, indicator.DefaultSnapshotTypes)
	for _, typ := range indicator.DefaultSnapshotTypes {
		require.True(t, known[typ], "default snapshot type %s not registered", typ)
	}
}
