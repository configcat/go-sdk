package configcat

import (
	"sync/atomic"
	"time"
)

// AutoPollingPolicy describes a RefreshPolicy which polls the latest configuration over HTTP and updates the local cache repeatedly.
type AutoPollingPolicy struct {
	ConfigRefresher
	autoPollInterval time.Duration
	logger           Logger
	init             *Async
	initialized      uint32
	stop             chan struct{}
	closed           uint32
	configChanged    func(config string, parser *ConfigParser)
	parser           *ConfigParser
}

// NewAutoPollingPolicy initializes a new AutoPollingPolicy.
func NewAutoPollingPolicy(
	configProvider ConfigProvider,
	store *ConfigStore,
	autoPollInterval time.Duration) *AutoPollingPolicy {
	return NewAutoPollingPolicyWithChangeListener(configProvider, store, autoPollInterval, nil)
}

// NewAutoPollingPolicyWithChangeListener initializes a new AutoPollingPolicy.
// An optional configuration change listener callback can be passed.
func NewAutoPollingPolicyWithChangeListener(
	configProvider ConfigProvider,
	store *ConfigStore,
	autoPollInterval time.Duration,
	configChanged func(config string, parser *ConfigParser)) *AutoPollingPolicy {

	fetcher, ok := configProvider.(*ConfigFetcher)
	if ok {
		fetcher.mode = "a"
	}

	policy := &AutoPollingPolicy{
		ConfigRefresher:  ConfigRefresher{ConfigProvider: configProvider, Store: store},
		autoPollInterval: autoPollInterval,
		logger:           store.logger.Prefix("ConfigCat - Auto Polling Policy"),
		init:             NewAsync(),
		initialized:      no,
		stop:             make(chan struct{}),
		configChanged:    configChanged,
		parser:           newParser(),
	}
	policy.startPolling()
	return policy
}

// GetConfigurationAsync reads the current configuration value.
func (policy *AutoPollingPolicy) GetConfigurationAsync() *AsyncResult {
	if policy.init.IsCompleted() {
		return policy.readCache()
	}

	return policy.init.Apply(func() interface{} {
		return policy.Store.Get()
	})
}

// Close shuts down the policy.
func (policy *AutoPollingPolicy) Close() {
	if atomic.CompareAndSwapUint32(&policy.closed, no, yes) {
		close(policy.stop)
	}
}

func (policy *AutoPollingPolicy) startPolling() {
	policy.logger.Printf("Auto polling started with %+v interval", policy.autoPollInterval)

	ticker := time.NewTicker(policy.autoPollInterval)

	go func() {
		defer ticker.Stop()
		policy.poll()
		for {
			select {
			case <-policy.stop:
				policy.logger.Print("Auto polling stopped")
				return
			case <-ticker.C:
				policy.poll()
			}
		}
	}()
}

func (policy *AutoPollingPolicy) poll() {
	policy.logger.Print("Polling the latest configuration")
	response := policy.ConfigProvider.GetConfigurationAsync().Get().(FetchResponse)
	cached := policy.Store.Get()
	if response.IsFetched() && cached != response.Body {
		policy.Store.Set(response.Body)
		if policy.configChanged != nil {
			policy.configChanged(response.Body, policy.parser)
		}
	}

	if atomic.CompareAndSwapUint32(&policy.initialized, no, yes) {
		policy.init.Complete()
	}
}

func (policy *AutoPollingPolicy) readCache() *AsyncResult {
	policy.logger.Print("Reading from cache")
	return AsCompletedAsyncResult(policy.Store.Get())
}
