package configcat

import (
	"sync/atomic"
	"time"
)

// autoPollingPolicy describes a refreshPolicy which polls the latest configuration over HTTP and updates the local cache repeatedly.
type autoPollingPolicy struct {
	configRefresher
	autoPollInterval time.Duration
	init             *async
	initialized      uint32
	stop             chan struct{}
	closed           uint32
	configChanged    func()
}

// autoPollConfig describes the configuration for auto polling.
type autoPollConfig struct {
	// The auto polling interval.
	autoPollInterval time.Duration
	// The configuration change listener.
	changeListener func()
}

func (config autoPollConfig) getModeIdentifier() string {
	return "a"
}

func (config autoPollConfig) refreshPolicy(rconfig refreshPolicyConfig) refreshPolicy {
	return newAutoPollingPolicy(config, rconfig)
}

// AutoPoll creates an auto polling refresh mode.
func AutoPoll(interval time.Duration) RefreshMode {
	return autoPollConfig{autoPollInterval: interval}
}

// AutoPollWithChangeListener creates an auto polling refresh mode with change listener callback.
func AutoPollWithChangeListener(
	interval time.Duration,
	changeListener func()) RefreshMode {
	return autoPollConfig{autoPollInterval: interval, changeListener: changeListener}
}

// newAutoPollingPolicy initializes a new autoPollingPolicy.
func newAutoPollingPolicy(
	autoPollConfig autoPollConfig,
	config refreshPolicyConfig,
) *autoPollingPolicy {
	policy := &autoPollingPolicy{
		configRefresher:  newConfigRefresher(config),
		autoPollInterval: autoPollConfig.autoPollInterval,
		init:             newAsync(),
		initialized:      no,
		stop:             make(chan struct{}),
		configChanged:    autoPollConfig.changeListener,
	}
	policy.startPolling()
	return policy
}

// getConfigurationAsync reads the current configuration value.
func (policy *autoPollingPolicy) getConfigurationAsync() *asyncResult {
	if policy.init.isCompleted() {
		return policy.readCache()
	}

	return policy.init.apply(func() interface{} {
		return policy.get()
	})
}

// close shuts down the policy.
func (policy *autoPollingPolicy) close() {
	if atomic.CompareAndSwapUint32(&policy.closed, no, yes) {
		close(policy.stop)
	}
}

func (policy *autoPollingPolicy) startPolling() {
	policy.logger.Debugf("Auto polling started with %+v interval.", policy.autoPollInterval)

	ticker := time.NewTicker(policy.autoPollInterval)

	go func() {
		defer ticker.Stop()
		policy.poll()
		for {
			select {
			case <-policy.stop:
				policy.logger.Debugf("Auto polling stopped.")
				return
			case <-ticker.C:
				policy.poll()
			}
		}
	}()
}

func (policy *autoPollingPolicy) poll() {
	policy.logger.Debugln("Polling the latest configuration.")
	response := policy.configFetcher.getConfigurationAsync().get().(fetchResponse)
	cached := policy.get()
	if response.isFetched() && cached.body() != response.config.body() {
		policy.set(response.config)
		if policy.configChanged != nil {
			policy.configChanged()
		}
	}

	if atomic.CompareAndSwapUint32(&policy.initialized, no, yes) {
		policy.init.complete()
	}
}

func (policy *autoPollingPolicy) readCache() *asyncResult {
	policy.logger.Debugln("Reading from cache.")
	return asCompletedAsyncResult(policy.get())
}
