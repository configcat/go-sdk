package configcat

import (
	"fmt"
	"testing"
)

const (
	jsonFormat = "{ \"%s\": { \"Value\": %s, \"SettingType\": 0, \"RolloutPercentageItems\": [], \"RolloutRules\": [] }}"
)

func getTestClients() (*fakeConfigProvider, *Client) {
	config := DefaultClientConfig()
	config.PolicyFactory = func(configProvider ConfigProvider, store *ConfigStore) RefreshPolicy {
		return NewManualPollingPolicy(configProvider, store)
	}
	fetcher := newFakeConfigProvider()
	client := newInternal("fakeKey",
		config,
		fetcher)

	return fetcher, client
}

func TestClient_Refresh(t *testing.T) {

	config := DefaultClientConfig()
	config.PolicyFactory = func(configProvider ConfigProvider, store *ConfigStore) RefreshPolicy {
		return NewManualPollingPolicy(configProvider, store)
	}
	fetcher := newFakeConfigProvider()
	client := newInternal("fakeKey",
		config,
		fetcher)

	fetcher.SetResponse(FetchResponse{ Status:Fetched, Body: fmt.Sprintf(jsonFormat, "key", "\"value\"")})
	result := client.GetValue("key", "default")

	if result != "value" {
		t.Error("Expecting non default string value")
	}

	fetcher.SetResponse(FetchResponse{ Status:Fetched, Body: fmt.Sprintf(jsonFormat, "key", "\"value2\"")})
	client.Refresh()
	result = client.GetValue("key", "default")
	if result != "value2" {
		t.Error("Expecting non default string value")
	}
}

func TestClient_Get(t *testing.T) {
	fetcher, client := getTestClients()
	fetcher.SetResponse(FetchResponse{ Status:Fetched, Body: fmt.Sprintf(jsonFormat, "key", "3213")})
	result := client.GetValue("key", 0)

	if result == nil {
		t.Error("Expecting non default value")
	}
}

func TestClient_Get_Default(t *testing.T) {
	fetcher, client := getTestClients()
	fetcher.SetResponse(FetchResponse{ Status:Failure, Body: ""})
	result := client.GetValue("key", 0)

	if result != 0 {
		t.Error("Expecting default int value")
	}
}

func TestClient_Get_Latest(t *testing.T) {
	fetcher, client := getTestClients()
	fetcher.SetResponse(FetchResponse{ Status:Fetched, Body: fmt.Sprintf(jsonFormat, "key", "3213")})
	result := client.GetValue("key", 0)

	if result == nil {
		t.Error("Expecting non default value")
	}

	fetcher.SetResponse(FetchResponse{ Status:Failure, Body: ""})

	result = client.GetValue("key", 0)

	if result == nil {
		t.Error("Expecting non default value")
	}
}