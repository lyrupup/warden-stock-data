package market

import (
	"os"
	"strconv"
)

// NewProvider 返回行情源 provider。目前仅支持 gotdx（通达信），不再有 stub 回退。
// providerName 仅作多源扩展占位，当前未参与选型。
func NewProvider(providerName string) IMarketProvider {
	maxConn := 10
	if v := os.Getenv("MARKET_GOTDX_MAX_CONN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConn = n
		}
	}
	return NewGotdxProvider(maxConn)
}
