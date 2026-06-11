package market

// NewProviderChain builds primary + fallback providers.
func NewProviderChain(providerName string, fallbacks ...IMarketProvider) IMarketProvider {
	primary := NewProvider(providerName)
	chain := []IMarketProvider{primary}
	chain = append(chain, fallbacks...)
	if len(chain) == 1 {
		return primary
	}
	return NewFallbackProvider(chain...)
}
