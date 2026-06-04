package market

// NewProvider returns the configured market provider. V1 defaults to stub when gotdx is unavailable.
func NewProvider(providerName string) IMarketProvider {
	switch providerName {
	case "gotdx":
		// Real gotdx adapter is injected via build tag `-tags gotdx` in future iterations.
		return NewStubProvider()
	default:
		return NewStubProvider()
	}
}
