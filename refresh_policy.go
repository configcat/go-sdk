package configcat

// RefreshPolicy is the public interface of a refresh policy which's implementors should describe the configuration update rules.
type RefreshPolicy interface {
	// GetConfigurationAsync reads the current configuration value.
	GetConfigurationAsync() *AsyncResult
	// RefreshAsync initiates a force refresh on the cached configuration.
	RefreshAsync() *Async
	// Close shuts down the policy.
	Close()
}

// ConfigRefresher describes a configuration refresher, holds a shared implementation of the RefreshAsync method on RefreshPolicy.
type ConfigRefresher struct {
	// The configuration provider implementation used to collect the latest configuration.
	Fetcher ConfigProvider
	// The configuration store used to maintain the cached configuration.
	Store *ConfigStore
}

// RefreshAsync initiates a force refresh on the cached configuration.
func (refresher *ConfigRefresher) RefreshAsync() *Async {
	return refresher.Fetcher.GetConfigurationAsync().Accept(func(result interface{}) {
		response := result.(FetchResponse)
		if result.(FetchResponse).IsFetched() {
			refresher.Store.Set(response.Body)
		}
	})
}
