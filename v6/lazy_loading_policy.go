package configcat

import (
	"sync/atomic"
	"time"
)

// lazyLoadingPolicy describes a refreshPolicy which uses an expiring cache to maintain the internally stored configuration.
type lazyLoadingPolicy struct {
	configRefresher
	cacheInterval   time.Duration
	isFetching      uint32
	initialized     uint32
	useAsyncRefresh bool
	lastRefreshTime time.Time
	fetching        *asyncResult
	init            *async
}

// lazyLoadConfig describes the configuration for auto polling.
type lazyLoadConfig struct {
	// The cache invalidation interval.
	cacheInterval time.Duration
	// If you use the asynchronous refresh then when a request is being made on the cache while it's expired,
	// the previous value will be returned immediately until the fetching of the new configuration is completed
	useAsyncRefresh bool
}

func (config lazyLoadConfig) getModeIdentifier() string {
	return "l"
}

func (config lazyLoadConfig) accept(visitor pollingModeVisitor) refreshPolicy {
	return visitor.visitLazyLoad(config)
}

// LazyLoad creates a lazy loading refresh mode.
func LazyLoad(cacheInterval time.Duration, useAsyncRefresh bool) RefreshMode {
	return lazyLoadConfig{cacheInterval: cacheInterval, useAsyncRefresh: useAsyncRefresh}
}

// newLazyLoadingPolicy initializes a new lazyLoadingPolicy.
func newLazyLoadingPolicy(
	configFetcher configProvider,
	cache ConfigCache,
	logger Logger,
	sdkKey string,
	config lazyLoadConfig) *lazyLoadingPolicy {
	return &lazyLoadingPolicy{configRefresher: newConfigRefresher(configFetcher, cache, logger, sdkKey),
		cacheInterval:   config.cacheInterval,
		isFetching:      no,
		initialized:     no,
		useAsyncRefresh: config.useAsyncRefresh,
		lastRefreshTime: time.Time{},
		init:            newAsync()}
}

// getConfigurationAsync reads the current configuration value.
func (policy *lazyLoadingPolicy) getConfigurationAsync() *asyncResult {
	if time.Since(policy.lastRefreshTime) > policy.cacheInterval {
		initialized := policy.init.isCompleted()

		if initialized && !atomic.CompareAndSwapUint32(&policy.isFetching, no, yes) {
			if policy.useAsyncRefresh {
				return policy.readCache()
			}
			return policy.fetching
		}

		policy.logger.Debugln("Cache expired, refreshing.")
		if initialized {
			policy.fetching = policy.fetch()
			if policy.useAsyncRefresh {
				return policy.readCache()
			}
			return policy.fetching
		}

		if atomic.CompareAndSwapUint32(&policy.isFetching, no, yes) {
			policy.fetching = policy.fetch()
		}
		return policy.init.apply(func() interface{} {
			return policy.get()
		})
	}

	return policy.readCache()
}

// close shuts down the policy.
func (policy *lazyLoadingPolicy) close() {
}

func (policy *lazyLoadingPolicy) fetch() *asyncResult {
	return policy.configFetcher.getConfigurationAsync().applyThen(func(result interface{}) interface{} {
		defer atomic.StoreUint32(&policy.isFetching, no)

		response := result.(fetchResponse)
		cached := policy.get()
		fetched := response.isFetched()

		if fetched && response.body != cached {
			policy.set(response.body)
		}

		if !response.isFailed() {
			policy.lastRefreshTime = time.Now()
		}

		if atomic.CompareAndSwapUint32(&policy.initialized, no, yes) {
			policy.init.complete()
		}

		if fetched {
			return response.body
		}

		return cached
	})
}

func (policy *lazyLoadingPolicy) readCache() *asyncResult {
	policy.logger.Debugln("Reading from cache.")
	return asCompletedAsyncResult(policy.get())
}
