package configcat

// manualPollingPolicy describes a refreshPolicy which fetches the latest configuration over HTTP every time when a get configuration is called.
type manualPollingPolicy struct {
	configRefresher
}

type manualPollConfig struct {
}

func (config manualPollConfig) getModeIdentifier() string {
	return "m"
}

func (config manualPollConfig) accept(visitor pollingModeVisitor) refreshPolicy {
	return visitor.visitManualPoll(config)
}

// ManualPoll creates a manual loading refresh mode.
func ManualPoll() RefreshMode {
	return manualPollConfig{}
}

// newManualPollingPolicy initializes a new manualPollingPolicy.
func newManualPollingPolicy(
	configFetcher configProvider,
	cache ConfigCache,
	logger Logger) *manualPollingPolicy {

	return &manualPollingPolicy{configRefresher: configRefresher{configFetcher: configFetcher, cache: cache, logger: logger}}
}

// getConfigurationAsync reads the current configuration value.
func (policy *manualPollingPolicy) getConfigurationAsync() *asyncResult {
	return asCompletedAsyncResult(policy.get())
}

// close shuts down the policy.
func (policy *manualPollingPolicy) close() {
}
