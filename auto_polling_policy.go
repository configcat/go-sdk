package configcat

import (
	"log"
	"os"
	"sync/atomic"
	"time"
)

// AutoPollingPolicy describes a RefreshPolicy which polls the latest configuration over HTTP and updates the local cache repeatedly.
type AutoPollingPolicy struct {
	ConfigRefresher
	autoPollInterval time.Duration
	logger           *log.Logger
	init             *Async
	initialized      uint32
	stop             chan struct{}
	closed           uint32
	configChanged    func(config string, parser *ConfigParser)
	parser           *ConfigParser
}

// NewAutoPollingPolicy initializes a new AutoPollingPolicy.
func NewAutoPollingPolicy(
	fetcher ConfigProvider,
	store *ConfigStore,
	autoPollInterval time.Duration) *AutoPollingPolicy {
	policy := NewAutoPollingPolicyWithChangeListener(fetcher, store, autoPollInterval, nil)
	policy.startPolling()
	return policy
}

// NewAutoPollingPolicyWithChangeListener initializes a new AutoPollingPolicy.
// An optional configuration change listener callback can be passed.
func NewAutoPollingPolicyWithChangeListener(
	fetcher ConfigProvider,
	store *ConfigStore,
	autoPollInterval time.Duration,
	configChanged func(config string, parser *ConfigParser)) *AutoPollingPolicy {
	policy := &AutoPollingPolicy{ConfigRefresher: ConfigRefresher{Fetcher: fetcher, Store: store},
		autoPollInterval: autoPollInterval,
		logger:           log.New(os.Stderr, "[ConfigCat - Auto Polling Policy]", log.LstdFlags),
		init:             NewAsync(),
		initialized:      no,
		stop:             make(chan struct{}),
		configChanged:    configChanged,
		parser:           newParser()}
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
				policy.logger.Println("Auto polling stopped")
				return
			case <-ticker.C:
				policy.poll()
			}
		}
	}()
}

func (policy *AutoPollingPolicy) poll() {
	policy.logger.Println("Polling the latest configuration")
	response := policy.Fetcher.GetConfigurationAsync().Get().(FetchResponse)
	cached := policy.Store.Get()
	if response.IsFetched() && cached != response.Body {
		policy.Store.Set(response.Body)
		if policy.configChanged != nil {
			policy.configChanged(response.Body, policy.parser)
		}
	}

	if atomic.CompareAndSwapUint32(&policy.initialized, no, yes) && !response.IsFailed() {
		policy.init.Complete()
	}
}

func (policy *AutoPollingPolicy) readCache() *AsyncResult {
	policy.logger.Println("Reading from cache")
	return AsCompletedAsyncResult(policy.Store.Get())
}
