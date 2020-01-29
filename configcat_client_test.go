package configcat

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

const (
	jsonFormat = "{ \"%s\": { \"v\": %s, \"p\": [], \"r\": [] }}"
)

type FailingCache struct {
}

// get reads the configuration from the cache.
func (cache *FailingCache) Get() (string, error) {
	return "", errors.New("fake failing cache fails to get")
}

// set writes the configuration into the cache.
func (cache *FailingCache) Set(value string) error {
	return errors.New("fake failing cache fails to set")
}

func getTestClients() (*fakeConfigProvider, *Client) {

	config := ClientConfig{Mode: ManualPoll()}
	fetcher := newFakeConfigProvider()
	client := newInternal("fakeKey",
		config,
		fetcher)

	return fetcher, client
}

func TestClient_Refresh(t *testing.T) {

	config := ClientConfig{Mode: ManualPoll()}
	fetcher := newFakeConfigProvider()
	client := newInternal("fakeKey",
		config,
		fetcher)

	fetcher.SetResponse(fetchResponse{status: Fetched, body: fmt.Sprintf(jsonFormat, "key", "\"value\"")})
	client.Refresh()
	result := client.GetValue("key", "default")

	if result != "value" {
		t.Error("Expecting non default string value")
	}

	fetcher.SetResponse(fetchResponse{status: Fetched, body: fmt.Sprintf(jsonFormat, "key", "\"value2\"")})
	client.Refresh()
	result = client.GetValue("key", "default")
	if result != "value2" {
		t.Error("Expecting non default string value")
	}
}

func TestClient_Refresh_Timeout(t *testing.T) {

	config := ClientConfig{Mode: ManualPoll(), MaxWaitTimeForSyncCalls: time.Second * 1}
	fetcher := newFakeConfigProvider()
	client := newInternal("fakeKey",
		config,
		fetcher)

	fetcher.SetResponse(fetchResponse{status: Fetched, body: fmt.Sprintf(jsonFormat, "key", "\"value\"")})
	client.Refresh()
	result := client.GetValue("key", "default")

	if result != "value" {
		t.Error("Expecting non default string value")
	}

	fetcher.SetResponseWithDelay(fetchResponse{status: Fetched, body: fmt.Sprintf(jsonFormat, "key", "\"value2\"")}, time.Second*10)
	client.Refresh()
	result = client.GetValue("key", "default")
	if result != "value" {
		t.Error("Expecting non default string value")
	}
}

func TestClient_Get(t *testing.T) {
	fetcher, client := getTestClients()
	fetcher.SetResponse(fetchResponse{status: Fetched, body: fmt.Sprintf(jsonFormat, "key", "3213")})
	client.Refresh()
	result := client.GetValue("key", 0)

	if result == nil || result == 0 {
		t.Error("Expecting non default value")
	}
}

func TestClient_Get_Default(t *testing.T) {
	fetcher, client := getTestClients()
	fetcher.SetResponse(fetchResponse{status: Failure, body: ""})
	result := client.GetValue("key", 0)

	if result != 0 {
		t.Error("Expecting default int value")
	}
}

func TestClient_Get_Latest(t *testing.T) {
	fetcher, client := getTestClients()
	fetcher.SetResponse(fetchResponse{status: Fetched, body: fmt.Sprintf(jsonFormat, "key", "3213")})
	client.Refresh()
	result := client.GetValue("key", 0)

	if result == nil || result == 0 {
		t.Error("Expecting non default value")
	}

	fetcher.SetResponse(fetchResponse{status: Failure, body: ""})

	result = client.GetValue("key", 0)

	if result == nil || result == 0 {
		t.Error("Expecting non default value")
	}
}

func TestClient_Get_WithTimeout(t *testing.T) {
	config := ClientConfig{Mode: ManualPoll(), MaxWaitTimeForSyncCalls: time.Second * 1}
	fetcher := newFakeConfigProvider()
	client := newInternal("fakeKey",
		config,
		fetcher)

	fetcher.SetResponseWithDelay(fetchResponse{status: Fetched, body: fmt.Sprintf(jsonFormat, "key", "3213")}, time.Second*10)
	result := client.GetValue("key", 0)

	if result != 0 {
		t.Error("Expecting default value")
	}
}

func TestClient_Get_WithFailingCache(t *testing.T) {
	config := ClientConfig{Mode: ManualPoll(), Cache: &FailingCache{}}
	fetcher := newFakeConfigProvider()
	client := newInternal("fakeKey",
		config,
		fetcher)

	fetcher.SetResponse(fetchResponse{status: Fetched, body: fmt.Sprintf(jsonFormat, "key", "3213")})
	client.Refresh()
	result := client.GetValue("key", 0)

	if result == 0 {
		t.Error("Expecting non default value")
	}
}

func TestClient_GetAllKeys(t *testing.T) {

	client := NewClient("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A")

	keys, err := client.GetAllKeys()

	if err != nil {
		t.Error(err)
	}

	if len(keys) != 16 {
		t.Error("Expecting 16 items")
	}
}
