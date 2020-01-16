package configcat

// ManualPollingPolicy describes a RefreshPolicy which fetches the latest configuration over HTTP every time when a get configuration is called.
type ManualPollingPolicy struct {
	ConfigRefresher
}

// NewManualPollingPolicy initializes a new ManualPollingPolicy.
func NewManualPollingPolicy(
	configProvider ConfigProvider,
	store *ConfigStore,
	logger Logger) *ManualPollingPolicy {

	fetcher, ok := configProvider.(*ConfigFetcher)
	if ok {
		fetcher.mode = "m"
	}

	return &ManualPollingPolicy{ConfigRefresher: ConfigRefresher{ConfigProvider: configProvider, Store: store, Logger: logger}}
}

// GetConfigurationAsync reads the current configuration value.
func (policy *ManualPollingPolicy) GetConfigurationAsync() *AsyncResult {
	return AsCompletedAsyncResult(policy.Store.Get())
}

// Close shuts down the policy.
func (policy *ManualPollingPolicy) Close() {
}
