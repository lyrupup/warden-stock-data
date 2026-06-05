package market

import "os"

// NewProvider returns the configured market provider. Build with `-tags gotdx` for real gotdx.
func NewProvider(providerName string) IMarketProvider {
	if name := os.Getenv("MARKET_PROVIDER"); name != "" {
		providerName = name
	}
	if p := initGotdxIfNeeded(providerName); p != nil {
		return NewFallbackProvider(p, NewStubProvider())
	}
	return NewStubProvider()
}
