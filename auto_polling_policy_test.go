package configcat

import (
	"testing"
	"time"
)

func TestAutoPollingPolicy_GetConfigurationAsync(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: "test"})

	policy := NewAutoPollingPolicy(fetcher, newConfigStore(NewInMemoryConfigCache()), time.Second*2)
	defer policy.Close()

	config := policy.GetConfigurationAsync().Get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: "test2"})
	config = policy.GetConfigurationAsync().Get().(string)

	if config != "test" {
		t.Error("Expecting test as result")
	}

	time.Sleep(time.Second * 4)
	config = policy.GetConfigurationAsync().Get().(string)

	if config != "test2" {
		t.Error("Expecting test2 as result")
	}
}

func TestAutoPollingPolicy_GetConfigurationAsync_WithListener(t *testing.T) {
	fetcher := newFakeConfigProvider()

	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: "test"})
	c := make(chan string, 1)
	defer close(c)
	policy := NewAutoPollingPolicyWithChangeListener(fetcher, newConfigStore(NewInMemoryConfigCache()), time.Second*2, func(config string, parser *ConfigParser) {
		c <- config
	})
	defer policy.Close()
	config := <-c

	if config != "test" {
		t.Error("Expecting test as result")
	}
}
