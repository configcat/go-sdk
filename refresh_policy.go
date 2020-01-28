package configcat

// refreshPolicy is the public interface of a refresh policy which's implementors should describe the configuration update rules.
type refreshPolicy interface {
	// getConfigurationAsync reads the current configuration value.
	getConfigurationAsync() *asyncResult
	// refreshAsync initiates a force refresh on the cached configuration.
	refreshAsync() *async
	// close shuts down the policy.
	close()
}

// configRefresher describes a configuration refresher, holds a shared implementation of the refreshAsync method on refreshPolicy.
type configRefresher struct {
	// The configuration provider implementation used to collect the latest configuration.
	configFetcher configProvider
	// The configuration store used to maintain the cached configuration.
	store *configStore
	// The logger instance.
	logger Logger
}

// RefreshMode is a base for refresh mode configurations.
type RefreshMode interface {
	// Returns the identifier sent in User-Agent.
	getModeIdentifier() string
}


// refreshAsync initiates a force refresh on the cached configuration.
func (refresher *configRefresher) refreshAsync() *async {
	return refresher.configFetcher.getConfigurationAsync().accept(func(result interface{}) {
		response := result.(fetchResponse)
		if result.(fetchResponse).isFetched() {
			refresher.store.set(response.body)
		}
	})
}
