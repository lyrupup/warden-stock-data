package scheduler

import (
	"fmt"
	"strings"
)

// 作业类型常量：行情数据中台更新作业（证券列表 / 日K 全量·增量）。
// 日 K 采集由 Python baostock 执行；指标实时计算、不再落库快照。
const (
	JobSecurities       = "securities"
	JobCalendar         = "calendar"
	JobFactors          = "factors"
	JobKlineFull        = "full"
	JobKlineIncremental = "incremental"
)

// maxFailedCodesStored 写入 run.FailedCodes 的最多代码数，超量截断并附计数，避免字段过大。
const maxFailedCodesStored = 500

// formatFailedCodes 把未成功标的代码列表格式化为持久化字符串：逗号分隔，超过上限截断并以
// "…(共 N 个)" 标注，便于运维复制失败代码后单独重跑补数。空列表返回空串。
func formatFailedCodes(codes []string) string {
	if len(codes) == 0 {
		return ""
	}
	if len(codes) <= maxFailedCodesStored {
		return strings.Join(codes, ",")
	}
	return fmt.Sprintf("%s,…(共 %d 个)", strings.Join(codes[:maxFailedCodesStored], ","), len(codes))
}
