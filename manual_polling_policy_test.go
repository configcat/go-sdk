package configcat

import (
	"testing"
)

func TestManualPollingPolicy_GetConfigurationAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()
	logger := DefaultLogger()
	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: "test"})
	policy := NewManualPollingPolicy(
		fetcher,
		newConfigStore(logger, NewInMemoryConfigCache()),
		logger,
	)
	config := policy.GetConfigurationAsync().Get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: "test2"})
	config = policy.GetConfigurationAsync().Get().(string)

	if config != "test2" {
		t.Error("Expecting test2 as result")
	}
}

func TestManualPollingPolicy_GetConfigurationAsync_Fail(t *testing.T) {
	fetcher := newFakeConfigProvider()
	logger := DefaultLogger()
	fetcher.SetResponse(FetchResponse{Status: Failure, Body: ""})
	policy := NewManualPollingPolicy(
		fetcher,
		newConfigStore(logger, NewInMemoryConfigCache()),
		logger,
	)
	config := policy.GetConfigurationAsync().Get().(string)

	if config != "" {
		t.Error("Expecting default")
	}
}
