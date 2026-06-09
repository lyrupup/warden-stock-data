package indicator

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

// SnapshotValueDecimals 指标快照落库小数位数，与 API 展示口径一致。
const SnapshotValueDecimals = 4

// SnapshotValuesJSON 将指标值序列化为 JSONB 载荷，固定小数位，避免 decimal 默认 JSON 高精度导致存储膨胀。
func SnapshotValuesJSON(vals map[string]decimal.Decimal) ([]byte, error) {
	if len(vals) == 0 {
		return []byte("{}"), nil
	}
	compact := make(map[string]string, len(vals))
	for k, v := range vals {
		compact[k] = v.StringFixed(SnapshotValueDecimals)
	}
	return json.Marshal(compact)
}
