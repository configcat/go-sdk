package configcat

import (
	"fmt"
	"testing"
)

func TestClient_RefreshAsync(t *testing.T) {

	config := DefaultClientConfig()
	config.PolicyFactory = func(configProvider ConfigProvider, store *ConfigStore, logger Logger) RefreshPolicy {
		return NewManualPollingPolicy(configProvider, store, logger)
	}
	fetcher := newFakeConfigProvider()
	client := newInternal("fakeKey",
		config,
		fetcher)

	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: fmt.Sprintf(jsonFormat, "key", "\"value\"")})
	client.Refresh()
	c := make(chan string, 1)
	defer close(c)
	client.GetValueAsync("key", "default", func(result interface{}) {
		c <- result.(string)
	})

	result := <-c

	if result != "value" {
		t.Error("Expecting non default string value")
	}

	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: fmt.Sprintf(jsonFormat, "key", "\"value2\"")})
	client.Refresh()
	c2 := make(chan string, 1)
	defer close(c2)
	client.RefreshAsync(func() {
		c2 <- client.GetValue("key", "default").(string)
	})
	result = <-c2
	if result != "value2" {
		t.Error("Expecting non default string value")
	}
}

func TestClient_GetAsync(t *testing.T) {
	fetcher, client := getTestClients()
	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: fmt.Sprintf(jsonFormat, "key", "3213")})
	c := make(chan interface{}, 1)
	defer close(c)
	client.GetValueAsync("key", 0, func(result interface{}) {
		c <- result
	})

	result := <-c

	if result == nil {
		t.Error("Expecting non default value")
	}
}

func TestClient_GetAsync_Default(t *testing.T) {
	fetcher, client := getTestClients()
	fetcher.SetResponse(FetchResponse{Status: Failure, Body: ""})
	c := make(chan interface{}, 1)
	defer close(c)
	client.GetValueAsync("key", 0, func(result interface{}) {
		c <- result
	})

	result := <-c

	if result != 0 {
		t.Error("Expecting default int value")
	}
}

func TestClient_GetAsync_Latest(t *testing.T) {
	fetcher, client := getTestClients()
	fetcher.SetResponse(FetchResponse{Status: Fetched, Body: fmt.Sprintf(jsonFormat, "key", "3213")})
	c := make(chan interface{}, 1)
	defer close(c)
	client.GetValueAsync("key", 0, func(result interface{}) {
		c <- result
	})

	result := <-c

	if result == nil {
		t.Error("Expecting non default value")
	}

	fetcher.SetResponse(FetchResponse{Status: Failure, Body: ""})

	c2 := make(chan interface{}, 1)
	defer close(c2)
	client.GetValueAsync("key", 0, func(result interface{}) {
		c2 <- result
	})

	result = <-c2

	if result == nil {
		t.Error("Expecting non default value")
	}
}
