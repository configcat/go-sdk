package configcat

import (
	"log"
	"os"
	"sync/atomic"
	"time"
)

// Describes a RefreshPolicy which uses an expiring cache to maintain the internally stored configuration.
type ExpiringCachePolicy struct {
	ConfigRefresher
	cacheInterval 		time.Duration
	isFetching 			uint32
	initialized			uint32
	useAsyncRefresh		bool
	lastRefreshTime		time.Time
	logger 				*log.Logger
	fetching			*AsyncResult
	init				*Async
}

// NewExpiringCachePolicy initializes a new ExpiringCachePolicy.
func NewExpiringCachePolicy(
	fetcher ConfigProvider,
	store *ConfigStore,
	cacheInterval time.Duration,
	useAsyncRefresh bool) *ExpiringCachePolicy {
	return &ExpiringCachePolicy{ ConfigRefresher: ConfigRefresher{ Fetcher:fetcher, Store:store },
		cacheInterval: cacheInterval,
		isFetching: no,
		initialized: no,
		useAsyncRefresh: useAsyncRefresh,
		lastRefreshTime: time.Time{},
		init: NewAsync(),
		logger: log.New(os.Stderr, "[ConfigCat - Expiring Cache Policy]", log.LstdFlags)}
}

// GetConfigurationAsync reads the current configuration value.
func (policy *ExpiringCachePolicy) GetConfigurationAsync() *AsyncResult {
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
		} else {
			if atomic.CompareAndSwapUint32(&policy.isFetching, no, yes) {
				policy.fetching = policy.fetch()
			}
			return policy.init.Apply(func() interface{} {
				return policy.Store.Get()
			})
		}
	}

	return policy.readCache()
}

// Close shuts down the policy.
func (policy *ExpiringCachePolicy) Close() {
}

func (policy *ExpiringCachePolicy) fetch() *AsyncResult {
	return policy.Fetcher.GetConfigurationAsync().ApplyThen(func(result interface{}) interface{} {
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

		if fetched && atomic.CompareAndSwapUint32(&policy.initialized, no, yes) {
			policy.init.Complete()
		}

		if fetched {
			return response.Body
		}

		return cached
	})
}

func (policy *ExpiringCachePolicy) readCache() *AsyncResult {
	policy.logger.Println("Reading from cache")
	return AsCompletedAsyncResult(policy.Store.Get())
}