package scheduler

import (
	"fmt"
	"strings"
)

// 作业类型常量：行情数据中台的更新作业被细分为「证券列表 / 日K（全量·增量）/ 指标（全量·增量）」五类，
// K 线拉取与指标计算彻底解耦，便于按需单独触发与补数。
const (
	// JobSecurities 证券列表同步：把 gotdx 证券列表（代码 / 名称 / 板块）同步入库。
	JobSecurities = "securities"
	// JobKlineFull 全量日K数据回补：按证券列表整体覆盖回补全部历史日 K（已有日期一并覆盖）。
	JobKlineFull = "full"
	// JobKlineIncremental 增量日K数据回补：按证券列表补齐并覆盖最新一日日 K。
	JobKlineIncremental = "incremental"
	// JobIndicatorFull 全量日K技术数据回补：基于已入库全量日 K 逐日重算全部指标快照（全量覆盖）。
	JobIndicatorFull = "indicator_full"
	// JobIndicatorIncremental 增量日K技术数据回补：基于已入库日 K 计算最新一日指标快照（覆盖）。
	JobIndicatorIncremental = "indicator_incremental"
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
