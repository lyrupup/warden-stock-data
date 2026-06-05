//go:build gotdx

package market

import "os"

func initGotdxIfNeeded(name string) IMarketProvider {
	if name != "gotdx" {
		return nil
	}
	maxConn := 10
	if v := os.Getenv("MARKET_GOTDX_MAX_CONN"); v != "" {
		// simplified: use default 10
		_ = v
	}
	return NewGotdxProvider(maxConn)
}
