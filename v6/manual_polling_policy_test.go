package configcat

import (
	"testing"
)

func TestManualPollingPolicy_GetConfigurationAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()
	logger := DefaultLogger(LogLevelWarn)
	fetcher.SetResponse(fetchResponse{status: Fetched, config: mustParseConfig(`{"test":1}`)})
	policy := newManualPollingPolicy(
		fetcher,
		inMemoryConfigCache{},
		logger,
		"",
	)

	policy.refreshAsync().wait()
	conf := policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":1}` {
		t.Error("Expecting test as result")
	}

	fetcher.SetResponse(fetchResponse{status: Fetched, config: mustParseConfig(`{"test":2}`)})
	policy.refreshAsync().wait()
	conf = policy.getConfigurationAsync().get().(*config)

	if conf.body() != `{"test":2}` {
		t.Error("Expecting test2 as result")
	}
}

func TestManualPollingPolicy_GetConfigurationAsync_Fail(t *testing.T) {
	fetcher := newFakeConfigProvider()
	logger := DefaultLogger(LogLevelWarn)
	fetcher.SetResponse(fetchResponse{status: Failure})
	policy := newManualPollingPolicy(
		fetcher,
		inMemoryConfigCache{},
		logger,
		"",
	)
	config := policy.getConfigurationAsync().get().(*config)

	if config != nil {
		t.Error("Expecting default")
	}
}
