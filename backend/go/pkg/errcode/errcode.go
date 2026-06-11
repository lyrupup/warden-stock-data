package errcode

// 错误码段位：10xxx 通用、20xxx 行情、21xxx 指标、22xxx 作业、23xxx 数据源、
// 40xxx 管理员鉴权、41xxx 开放 API 凭证、42xxx 限流/配额。
const (
	OK = 0

	ErrParam       = 10001
	ErrNotFound    = 10002
	ErrTimeout     = 10408
	ErrInternal    = 10000

	ErrProvider    = 20001
	ErrStockNotFound = 20002

	ErrIndicatorParam = 21001

	ErrJobConflict = 22001

	ErrDataSource = 23001

	ErrAdminUnauthorized = 40001

	ErrMissingSignature = 41001
	ErrCredentialInvalid  = 41002
	ErrSignatureMismatch  = 41003
	ErrReplayOrExpired    = 41004
	ErrScopeInsufficient  = 41005

	ErrRateLimited = 42001
	ErrQuotaExceeded = 42002
)

var messages = map[int]string{
	OK: "ok",

	ErrParam:    "参数错误",
	ErrNotFound: "资源不存在",
	ErrTimeout:  "请求超时",
	ErrInternal: "内部错误",

	ErrProvider:      "行情数据源异常",
	ErrStockNotFound: "股票不存在",

	ErrIndicatorParam: "指标参数非法",

	ErrJobConflict: "更新作业冲突",

	ErrDataSource: "数据源异常",

	ErrAdminUnauthorized: "未登录或 token 非法",

	ErrMissingSignature: "缺少凭证签名头",
	ErrCredentialInvalid:  "凭证无效或已吊销",
	ErrSignatureMismatch:  "签名校验失败",
	ErrReplayOrExpired:    "时间戳过期或 nonce 重放",
	ErrScopeInsufficient:  "凭证 scope 不足",

	ErrRateLimited:   "触发限流",
	ErrQuotaExceeded: "超出日调用配额",
}

func Message(code int) string {
	if msg, ok := messages[code]; ok {
		return msg
	}
	return "未知错误"
}
