//go:build !gotdx

package market

func initGotdxIfNeeded(string) IMarketProvider { return nil }
