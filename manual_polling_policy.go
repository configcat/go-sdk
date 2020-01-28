package configcat

// manualPollingPolicy describes a refreshPolicy which fetches the latest configuration over HTTP every time when a get configuration is called.
type manualPollingPolicy struct {
	configRefresher
}

// manualPollConfig describes the configuration for manual polling.
type manualPollConfig struct {
}

// getModeIdentifier returns the mode identifier sent in User-Agent.
func (config manualPollConfig) getModeIdentifier() string {
	return "m"
}

// Creates a lazy loading refresh mode.
func ManualPoll() RefreshMode {
	return manualPollConfig{}
}

// newManualPollingPolicy initializes a new manualPollingPolicy.
func newManualPollingPolicy(
	configFetcher configProvider,
	store *configStore,
	logger Logger) *manualPollingPolicy {

	return &manualPollingPolicy{configRefresher: configRefresher{configFetcher: configFetcher, store: store, logger: logger}}
}

// getConfigurationAsync reads the current configuration value.
func (policy *manualPollingPolicy) getConfigurationAsync() *asyncResult {
	return asCompletedAsyncResult(policy.store.get())
}

// close shuts down the policy.
func (policy *manualPollingPolicy) close() {
}
