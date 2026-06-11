package service

import "github.com/shopspring/decimal"

// 涨跌停价自算（与 Python core/limit_price.py 口径一致），用于 gotdx 增量采集时补全
// limit_up/limit_down（gotdx 行情不直接给涨跌停价，依板块/ST 规则现算）。
//
// A 股涨跌幅制度（注册制后）：
//   - 主板（沪 60*/深 000*/001*/002*/003*）  ±10%，ST ±5%
//   - 创业板（300*/301*）、科创板（688*/689*） ±20%（ST 同 20%）
//   - 北交所（8**/4**/920*）                  ±30%（ST 同 30%）
//   - 新股上市首日 / 不设限                    → (zero, zero, false)

const (
	boardMain = "MAIN"
	boardGEM  = "GEM"
	boardSTAR = "STAR"
	boardBSE  = "BSE"
)

// boardOf 按代码段判定板块（兼容 "sh.600000" 与纯代码 "600000"）。
func boardOf(code string) string {
	c := code
	if i := lastDotIndex(c); i >= 0 {
		c = c[i+1:]
	}
	switch {
	case hasPrefix(c, "688") || hasPrefix(c, "689"):
		return boardSTAR
	case hasPrefix(c, "300") || hasPrefix(c, "301"):
		return boardGEM
	case hasPrefix(c, "920") || hasPrefix(c, "8") || hasPrefix(c, "4"):
		return boardBSE
	default:
		return boardMain
	}
}

// limitPct 返回该标的单边涨跌幅比例。
func limitPct(code string, isST bool) decimal.Decimal {
	switch boardOf(code) {
	case boardBSE:
		return decimal.NewFromFloat(0.30)
	case boardGEM, boardSTAR:
		return decimal.NewFromFloat(0.20) // 创业/科创：ST 也按 20%
	default:
		if isST {
			return decimal.NewFromFloat(0.05)
		}
		return decimal.NewFromFloat(0.10)
	}
}

// computeLimitPrices 计算涨跌停价（四舍五入到分）；不设限 / 无有效昨收时返回 (0, 0, false)。
func computeLimitPrices(code string, preClose decimal.Decimal, isST, isFirstDay bool) (up, down decimal.Decimal, ok bool) {
	if isFirstDay || preClose.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero, false
	}
	pct := limitPct(code, isST)
	one := decimal.NewFromInt(1)
	up = preClose.Mul(one.Add(pct)).Round(2)
	down = preClose.Mul(one.Sub(pct)).Round(2)
	return up, down, true
}

func lastDotIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
