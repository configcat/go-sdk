package configcat

import (
	"testing"
	"time"
)

func TestLazyLoadingPolicy_GetConfigurationAsync_DoNotUseAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: "test"})

	policy := NewLazyLoadingPolicy(fetcher, newConfigStore(NewInMemoryConfigCache()), time.Second*2, false)
	config := policy.GetConfigurationAsync().Get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: "test2"})
	config = policy.GetConfigurationAsync().Get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 2)
	config = policy.GetConfigurationAsync().Get().(string)

	if config != "test2" {
		t.Error("Expecting test2 as result")
	}
}

func TestLazyLoadingPolicy_GetConfigurationAsync_Fail(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(FetchResponse{Status: Failure, Body: ""})

	policy := NewLazyLoadingPolicy(fetcher, newConfigStore(NewInMemoryConfigCache()), time.Second*2, false)
	config := policy.GetConfigurationAsync().Get().(string)

	if config != "" {
		t.Error("Expecting default")
	}
}

func TestLazyLoadingPolicy_GetConfigurationAsync_UseAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: "test"})

	policy := NewLazyLoadingPolicy(fetcher, newConfigStore(NewInMemoryConfigCache()), time.Second*2, true)
	config := policy.GetConfigurationAsync().Get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 2)

	fetcher.SetResponseWithDelay(FetchResponse{Status: Fetched, Body: "test2"}, time.Second*1)
	config = policy.GetConfigurationAsync().Get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 2)
	config = policy.GetConfigurationAsync().Get().(string)

	if config != "test2" {
		t.Error("Expecting test2 as result")
	}
}
