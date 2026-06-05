//go:build gotdx

package market

import (
	"os"
	"strconv"
)

func initGotdxIfNeeded(name string) IMarketProvider {
	if name != "gotdx" {
		return nil
	}
	maxConn := 10
	if v := os.Getenv("MARKET_GOTDX_MAX_CONN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConn = n
		}
	}
	return NewGotdxProvider(maxConn)
}
