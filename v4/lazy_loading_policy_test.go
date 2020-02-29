package configcat

import (
	"testing"
	"time"
)

func TestLazyLoadingPolicy_GetConfigurationAsync_DoNotUseAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Fetched, body: "test"})
	logger := DefaultLogger(LogLevelError)
	policy := newLazyLoadingPolicy(
		fetcher,
		newConfigStore(logger, newInMemoryConfigCache()),
		logger,
		lazyLoadConfig{time.Second * 2, false})
	config := policy.getConfigurationAsync().get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	fetcher.SetResponse(fetchResponse{status: Fetched, body: "test2"})
	config = policy.getConfigurationAsync().get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 2)
	config = policy.getConfigurationAsync().get().(string)

	if config != "test2" {
		t.Error("Expecting test2 as result")
	}
}

func TestLazyLoadingPolicy_GetConfigurationAsync_Fail(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Failure, body: ""})
	logger := DefaultLogger(LogLevelError)
	policy := newLazyLoadingPolicy(
		fetcher,
		newConfigStore(logger, newInMemoryConfigCache()),
		logger,
		lazyLoadConfig{time.Second * 2, false})
	config := policy.getConfigurationAsync().get().(string)

	if config != "" {
		t.Error("Expecting default")
	}
}

func TestLazyLoadingPolicy_GetConfigurationAsync_UseAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(fetchResponse{status: Fetched, body: "test"})
	logger := DefaultLogger(LogLevelError)
	policy := newLazyLoadingPolicy(
		fetcher,
		newConfigStore(logger, newInMemoryConfigCache()),
		logger,
		lazyLoadConfig{time.Second * 2, true})
	config := policy.getConfigurationAsync().get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 2)

	fetcher.SetResponseWithDelay(fetchResponse{status: Fetched, body: "test2"}, time.Second*1)
	config = policy.getConfigurationAsync().get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 2)
	config = policy.getConfigurationAsync().get().(string)

	if config != "test2" {
		t.Error("Expecting test2 as result")
	}
}
