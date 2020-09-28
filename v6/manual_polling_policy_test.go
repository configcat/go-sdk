package configcat

import (
	"testing"
)

func TestManualPollingPolicy_GetConfigurationAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()
	logger := DefaultLogger(LogLevelWarn)
	fetcher.SetResponse(fetchResponse{status: Fetched, body: "test"})
	policy := newManualPollingPolicy(
		fetcher,
		newInMemoryConfigCache(),
		logger,
	)

	policy.refreshAsync().wait()
	config := policy.getConfigurationAsync().get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	fetcher.SetResponse(fetchResponse{status: Fetched, body: "test2"})
	policy.refreshAsync().wait()
	config = policy.getConfigurationAsync().get().(string)

	if config != "test2" {
		t.Error("Expecting test2 as result")
	}
}

func TestManualPollingPolicy_GetConfigurationAsync_Fail(t *testing.T) {
	fetcher := newFakeConfigProvider()
	logger := DefaultLogger(LogLevelWarn)
	fetcher.SetResponse(fetchResponse{status: Failure, body: ""})
	policy := newManualPollingPolicy(
		fetcher,
		newInMemoryConfigCache(),
		logger,
	)
	config := policy.getConfigurationAsync().get().(string)

	if config != "" {
		t.Error("Expecting default")
	}
}
