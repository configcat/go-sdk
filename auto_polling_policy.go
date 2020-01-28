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
	configChanged    func(config string, parser *ConfigParser)
	parser           *ConfigParser
}

// autoPollConfig describes the configuration for auto polling.
type autoPollConfig struct {
	// The auto polling interval.
	autoPollInterval time.Duration
}

// getModeIdentifier returns the mode identifier sent in User-Agent.
func (config autoPollConfig) getModeIdentifier() string {
	return "a"
}

// Creates an auto polling refresh mode.
func AutoPoll(interval time.Duration) RefreshMode {
	return autoPollConfig{ autoPollInterval: interval }
}

// newAutoPollingPolicy initializes a new autoPollingPolicy.
func newAutoPollingPolicy(
	configFetcher configProvider,
	store *configStore,
	logger Logger,
	autoPollConfig autoPollConfig) *autoPollingPolicy {
	return newAutoPollingPolicyWithChangeListener(
		configFetcher,
		store,
		logger,
		autoPollConfig.autoPollInterval,
		nil)
}

// newAutoPollingPolicyWithChangeListener initializes a new autoPollingPolicy.
// An optional configuration change listener callback can be passed.
func newAutoPollingPolicyWithChangeListener(
	configFetcher configProvider,
	store *configStore,
	logger Logger,
	autoPollInterval time.Duration,
	configChanged func(config string, parser *ConfigParser)) *autoPollingPolicy {

	policy := &autoPollingPolicy{
		configRefresher:  configRefresher{configFetcher: configFetcher, store: store, logger: logger},
		autoPollInterval: autoPollInterval,
		init:             newAsync(),
		initialized:      no,
		stop:             make(chan struct{}),
		configChanged:    configChanged,
		parser:           newParser(logger),
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
		return policy.store.get()
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
	cached := policy.store.get()
	if response.isFetched() && cached != response.body {
		policy.store.set(response.body)
		if policy.configChanged != nil {
			policy.configChanged(response.body, policy.parser)
		}
	}

	if atomic.CompareAndSwapUint32(&policy.initialized, no, yes) {
		policy.init.complete()
	}
}

func (policy *autoPollingPolicy) readCache() *asyncResult {
	policy.logger.Debugln("Reading from cache.")
	return asCompletedAsyncResult(policy.store.get())
}
