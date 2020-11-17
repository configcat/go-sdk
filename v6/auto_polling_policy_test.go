package configcat

import (
	"testing"
	"time"
)

func TestAutoPollingPolicy_GetConfigurationAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Fetched, config: mustParseConfig(`{"test":1}`)})
	logger := DefaultLogger(LogLevelWarn)
	policy := newAutoPollingPolicy(
		autoPollConfig{
			autoPollInterval: time.Second * 2,
		},
		refreshPolicyConfig{
			configFetcher: fetcher,
			cache:         inMemoryConfigCache{},
			logger:        logger,
			sdkKey:        "",
		},
	)
	defer policy.close()

	conf := policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":1}` {
		t.Errorf("Expecting test as result, got %s", conf.body())
	}

	fetcher.SetResponse(fetchResponse{status: Fetched, config: mustParseConfig(`{"test":2}`)})
	conf = policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":1}` {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 4)
	conf = policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":2}` {
		t.Error("Expecting test2 as result")
	}
}

func TestAutoPollingPolicy_GetConfigurationAsync_Fail(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Failure})
	logger := DefaultLogger(LogLevelWarn)
	policy := newAutoPollingPolicy(
		autoPollConfig{
			autoPollInterval: time.Second * 2,
		},
		refreshPolicyConfig{
			configFetcher: fetcher,
			cache:         inMemoryConfigCache{},
			logger:        logger,
			sdkKey:        "",
		},
	)
	defer policy.close()

	config := policy.getConfigurationAsync().get().(*config)

	if config.body() != "" {
		t.Error("Expecting default")
	}
}

func TestAutoPollingPolicy_GetConfigurationAsync_WithListener(t *testing.T) {
	fetcher := newFakeConfigProvider()
	logger := DefaultLogger(LogLevelWarn)
	fetcher.SetResponse(fetchResponse{status: Fetched, config: mustParseConfig(`{"test":1}`)})
	c := make(chan bool, 1)
	defer close(c)
	policy := newAutoPollingPolicy(
		AutoPollWithChangeListener(
			time.Second*2,
			func() { c <- true },
		).(autoPollConfig),
		refreshPolicyConfig{
			configFetcher: fetcher,
			cache:         inMemoryConfigCache{},
			logger:        logger,
			sdkKey:        "",
		},
	)
	defer policy.close()
	called := <-c

	if !called {
		t.Error("Expecting test as result")
	}
}

func mustParseConfig(s string) *config {
	conf, err := parseConfig([]byte(s))
	if err != nil {
		panic(err)
	}
	return conf
}
