package configcat

import (
	"log"
	"os"
	"sync/atomic"
	"time"
)

// LazyLoadingPolicy describes a RefreshPolicy which uses an expiring cache to maintain the internally stored configuration.
type LazyLoadingPolicy struct {
	ConfigRefresher
	cacheInterval   time.Duration
	isFetching      uint32
	initialized     uint32
	useAsyncRefresh bool
	lastRefreshTime time.Time
	logger          *log.Logger
	fetching        *AsyncResult
	init            *Async
}

// NewLazyLoadingPolicy initializes a new LazyLoadingPolicy.
func NewLazyLoadingPolicy(
	configProvider ConfigProvider,
	store *ConfigStore,
	cacheInterval time.Duration,
	useAsyncRefresh bool) *LazyLoadingPolicy {

	fetcher, ok := configProvider.(*ConfigFetcher)
	if ok {
		fetcher.mode = "l"
	}

	return &LazyLoadingPolicy{ConfigRefresher: ConfigRefresher{ConfigProvider: configProvider, Store: store},
		cacheInterval:   cacheInterval,
		isFetching:      no,
		initialized:     no,
		useAsyncRefresh: useAsyncRefresh,
		lastRefreshTime: time.Time{},
		init:            NewAsync(),
		logger:          log.New(os.Stderr, "[ConfigCat - Lazy Loading Policy]", log.LstdFlags)}
}

// GetConfigurationAsync reads the current configuration value.
func (policy *LazyLoadingPolicy) GetConfigurationAsync() *AsyncResult {
	if time.Since(policy.lastRefreshTime) > policy.cacheInterval {
		initialized := policy.init.IsCompleted()

		if initialized && !atomic.CompareAndSwapUint32(&policy.isFetching, no, yes) {
			if policy.useAsyncRefresh {
				return policy.readCache()
			}
			return policy.fetching
		}

		policy.logger.Println("Cache expired, refreshing")
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
		return policy.init.Apply(func() interface{} {
			return policy.Store.Get()
		})
	}

	return policy.readCache()
}

// Close shuts down the policy.
func (policy *LazyLoadingPolicy) Close() {
}

func (policy *LazyLoadingPolicy) fetch() *AsyncResult {
	return policy.ConfigProvider.GetConfigurationAsync().ApplyThen(func(result interface{}) interface{} {
		defer atomic.StoreUint32(&policy.isFetching, no)

		response := result.(FetchResponse)
		cached := policy.Store.Get()
		fetched := response.IsFetched()

		if fetched && response.Body != cached {
			policy.Store.Set(response.Body)
		}

		if !response.IsFailed() {
			policy.lastRefreshTime = time.Now()
		}

		if atomic.CompareAndSwapUint32(&policy.initialized, no, yes) {
			policy.init.Complete()
		}

		if fetched {
			return response.Body
		}

		return cached
	})
}

func (policy *LazyLoadingPolicy) readCache() *AsyncResult {
	policy.logger.Println("Reading from cache")
	return AsCompletedAsyncResult(policy.Store.Get())
}
