package configcat

// manualPollingPolicy describes a refreshPolicy which fetches the latest configuration over HTTP every time when a get configuration is called.
type manualPollingPolicy struct {
	refresher *configRefresher
}

type manualPollConfig struct {
}

func (config manualPollConfig) getModeIdentifier() string {
	return "m"
}

func (config manualPollConfig) refreshPolicy(rconfig refreshPolicyConfig) refreshPolicy {
	return newManualPollingPolicy(rconfig)
}

// ManualPoll creates a manual loading refresh mode.
func ManualPoll() RefreshMode {
	return manualPollConfig{}
}

// newManualPollingPolicy initializes a new manualPollingPolicy.
func newManualPollingPolicy(rconfig refreshPolicyConfig) *manualPollingPolicy {

	return &manualPollingPolicy{
		refresher: newConfigRefresher(rconfig),
	}
}

// getConfigurationAsync reads the current configuration value.
func (policy *manualPollingPolicy) getConfigurationAsync() *asyncResult {
	return asCompletedAsyncResult(policy.refresher.get())
}

// close shuts down the policy.
func (policy *manualPollingPolicy) close() {
}

func (policy *manualPollingPolicy) getLastCachedConfig() *config {
	return policy.refresher.getLastCachedConfig()
}

func (policy *manualPollingPolicy) refreshAsync() *async {
	return policy.refresher.refreshAsync()
}
