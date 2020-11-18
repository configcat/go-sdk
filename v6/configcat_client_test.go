package configcat

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

const (
	jsonFormat          = `{ "f": { "%s": { "v": %s, "p": [], "r": [] }}}`
	variationJsonFormat = `{ "f": { "first": { "v": false, "p": [], "r": [], "i":"fakeIdFirst" }, "second": { "v": true, "p": [], "r": [], "i":"fakeIdSecond" }}}`
)

func TestClient_Refresh(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.Mode = ManualPoll()
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	srv.setResponse(configResponse{body: fmt.Sprintf(jsonFormat, "key", `"value"`)})
	client.Refresh()
	result := client.GetValue("key", "default")

	c.Assert(result, qt.Equals, "value")

	srv.setResponse(configResponse{body: fmt.Sprintf(jsonFormat, "key", `"value2"`)})
	client.Refresh()
	result = client.GetValue("key", "default")
	if result != "value2" {
		t.Error("Expecting non default string value")
	}
}

func TestClient_Refresh_Timeout(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.Mode = ManualPoll()
	cfg.MaxWaitTimeForSyncCalls = 10 * time.Millisecond
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	srv.setResponse(configResponse{body: fmt.Sprintf(jsonFormat, "key", `"value"`)})
	client.Refresh()
	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value")

	srv.setResponse(configResponse{
		body:  fmt.Sprintf(jsonFormat, "key", `"value"`),
		sleep: time.Second,
	})
	t0 := time.Now()
	client.Refresh()
	if d := time.Since(t0); d < 10*time.Millisecond || d > 50*time.Millisecond {
		t.Errorf("refresh returned too quickly; got %v want >10ms, <50ms", d)
	}
	result = client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value")
}

func TestClient_Get(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: fmt.Sprintf(jsonFormat, "key", "3213")})
	client.Refresh()
	result := client.GetValue("key", 0)

	c.Assert(result, qt.Equals, 3213.0)
}

func TestClient_Get_Default(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})
	result := client.GetValue("key", 0)
	c.Assert(result, qt.Equals, 0)
}

func TestClient_Get_Latest(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: fmt.Sprintf(jsonFormat, "key", "3213")})
	client.Refresh()

	result := client.GetValue("key", 0)
	c.Assert(result, qt.Equals, 3213.0)

	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})

	result = client.GetValue("key", 0)
	c.Assert(result, qt.Equals, 3213.0)
}

func TestClient_Get_WithTimeout(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.MaxWaitTimeForSyncCalls = 10 * time.Millisecond
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	srv.setResponse(configResponse{
		body:  fmt.Sprintf(jsonFormat, "key", "3213"),
		sleep: time.Second,
	})
	t0 := time.Now()
	result := client.GetValue("key", 0)
	c.Assert(result, qt.Equals, 0)
	if d := time.Since(t0); d < 10*time.Millisecond || d > 50*time.Millisecond {
		t.Errorf("refresh returned too quickly; got %v want >10ms, <50ms", d)
	}
}

func TestClient_Get_WithFailingCacheSet(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.Cache = failingCache{}
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	srv.setResponse(configResponse{body: fmt.Sprintf(jsonFormat, "key", "3213")})
	client.Refresh()
	result := client.GetValue("key", 0)
	c.Assert(result, qt.Equals, 3213.0)
}

func TestClient_GetAllKeys(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.sdkKey(), srv.config())

	keys, err := client.GetAllKeys()
	c.Assert(err, qt.IsNil)
	c.Assert(keys, qt.HasLen, 16)
}

func TestClient_GetVariationId(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: fmt.Sprintf(variationJsonFormat)})
	client.Refresh()
	result := client.GetVariationId("first", "")
	c.Assert(result, qt.Equals, "fakeIdFirst")
}

func TestClient_GetVariationId_Default(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: fmt.Sprintf(variationJsonFormat)})
	client.Refresh()
	result := client.GetVariationId("nonexisting", "")
	c.Assert(result, qt.Equals, "")
}

func TestClient_GetAllVariationIds(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: fmt.Sprintf(variationJsonFormat)})
	client.Refresh()
	result, err := client.GetAllVariationIds()
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.HasLen, 2)
}

func TestClient_GetAllVariationIds_Empty(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: `{ "f": {} }`})
	client.Refresh()
	result, err := client.GetAllVariationIds()
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.HasLen, 0)
}

func TestClient_GetKeyAndValue(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: fmt.Sprintf(variationJsonFormat)})
	client.Refresh()
	key, value := client.GetKeyAndValue("fakeIdSecond")
	c.Assert(key, qt.Equals, "second")
	c.Assert(value, qt.Equals, true)
}

func TestClient_GetKeyAndValue_Empty(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: fmt.Sprintf(variationJsonFormat)})
	client.Refresh()
	key, value := client.GetKeyAndValue("nonexisting")
	c.Assert(key, qt.Equals, "")
	c.Assert(value, qt.Equals, nil)
}

type failingCache struct{}

// get reads the configuration from the cache.
func (cache failingCache) Get(key string) (string, error) {
	return "", errors.New("fake failing cache fails to get")
}

// set writes the configuration into the cache.
func (cache failingCache) Set(key string, value string) error {
	return errors.New("fake failing cache fails to set")
}

type KeyCheckerCache struct {
	key string
}

func (cache *KeyCheckerCache) Get(key string) (string, error) {
	return "", nil
}

func (cache *KeyCheckerCache) Set(key string, value string) error {
	cache.key = key
	return nil
}

func getTestClients(t *testing.T) (*configServer, *Client) {
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.Mode = ManualPoll()
	client := NewCustomClient(srv.sdkKey(), cfg)
	t.Cleanup(client.Close)
	return srv, client
}
