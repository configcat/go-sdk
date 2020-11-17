package configcat

import (
	"testing"
	"time"
)

func TestLazyLoadingPolicy_GetConfigurationAsync_DoNotUseAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Fetched, config: mustParseConfig(`{"test":1}`)})
	logger := DefaultLogger(LogLevelWarn)
	policy := newLazyLoadingPolicy(
		lazyLoadConfig{
			cacheInterval:   time.Second * 2,
			useAsyncRefresh: false,
		},
		refreshPolicyConfig{
			configFetcher: fetcher,
			cache:         inMemoryConfigCache{},
			logger:        logger,
			sdkKey:        "",
		},
	)
	conf := policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":1}` {
		t.Error("Expecting test as result")
	}

	fetcher.SetResponse(fetchResponse{status: Fetched, config: mustParseConfig(`{"test":2}`)})
	conf = policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":1}` {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 2)
	conf = policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":2}` {
		t.Error("Expecting test2 as result")
	}
}

func TestLazyLoadingPolicy_GetConfigurationAsync_Fail(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Failure})
	logger := DefaultLogger(LogLevelWarn)
	policy := newLazyLoadingPolicy(
		lazyLoadConfig{
			cacheInterval:   time.Second * 2,
			useAsyncRefresh: false,
		},
		refreshPolicyConfig{
			configFetcher: fetcher,
			cache:         inMemoryConfigCache{},
			logger:        logger,
			sdkKey:        "",
		},
	)
	config := policy.getConfigurationAsync().get().(*config)

	if config != nil {
		t.Error("Expecting default")
	}
}

func TestLazyLoadingPolicy_GetConfigurationAsync_UseAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Fetched, config: mustParseConfig(`{"test":1}`)})
	logger := DefaultLogger(LogLevelWarn)
	policy := newLazyLoadingPolicy(
		lazyLoadConfig{
			cacheInterval:   time.Second * 2,
			useAsyncRefresh: true,
		},
		refreshPolicyConfig{
			configFetcher: fetcher,
			cache:         inMemoryConfigCache{},
			logger:        logger,
			sdkKey:        "",
		},
	)
	conf := policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":1}` {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 2)

	fetcher.SetResponseWithDelay(fetchResponse{status: Fetched, config: mustParseConfig(`{"test":2}`)}, time.Second*1)
	conf = policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":1}` {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 2)
	conf = policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":2}` {
		t.Errorf("Expecting test2 as result, got %s", conf.body())
	}
}
