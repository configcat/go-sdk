package configcat

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

const (
	variationJsonFormat = `{ "f": { "first": { "v": false, "p": [], "r": [], "i":"fakeIdFirst" }, "second": { "v": true, "p": [], "r": [], "i":"fakeIdSecond" }}}`
)

func TestClient_Refresh(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.Mode = ManualPoll()
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value"))
	client.Refresh()
	result := client.GetValue("key", "default")

	c.Assert(result, qt.Equals, "value")

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value2"))
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

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value"))
	client.Refresh()
	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value")

	srv.setResponse(configResponse{
		body:  marshalJSON(rootNodeWithKeyValue("key", "value")),
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
	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213))
	client.Refresh()
	result := client.GetValue("key", 0)

	c.Assert(result, qt.Equals, 3213.0)
}

func TestClient_Get_IsOneOf_Uses_Contains_Semantics(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(&rootNode{
		Entries: map[string]*entry{
			"feature": {
				Value:       false,
				VariationID: "a377be39",
				RolloutRules: []*rolloutRule{{
					Comparator:          opOneOf,
					ComparisonAttribute: "Identifier",
					ComparisonValue:     "example,foobar",
					Value:               true,
					VariationID:         "8bcf8608",
				}},
			},
		},
	})
	client.Refresh()

	matchingUser := NewUser("mple")
	result := client.GetValueForUser("feature", 0, matchingUser)
	c.Assert(result, qt.IsTrue)

	matchingUser = NewUser("foobar")
	result = client.GetValueForUser("feature", 0, matchingUser)
	c.Assert(result, qt.IsTrue)

	matchingUser = NewUser("nonexisting")
	result = client.GetValueForUser("feature", 0, matchingUser)
	c.Assert(result, qt.IsFalse)
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
	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213))
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
		body:  marshalJSON(rootNodeWithKeyValue("key", 3213)),
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

	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213))
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

func TestClient_GetWithRedirectSuccess(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := ForceRedirect
	srv1.setResponseJSON(&rootNode{
		Preferences: &preferences{
			URL:      srv2.config().BaseUrl,
			Redirect: &redirect,
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value"))
	client.Refresh()
	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value")
	c.Assert(srv1.allResponses(), qt.HasLen, 1)
	c.Assert(srv2.allResponses(), qt.HasLen, 1)

	// Another request should go direct to the second server.
	client.Refresh()
	c.Assert(srv1.allResponses(), qt.HasLen, 1)
	c.Assert(srv2.allResponses(), qt.HasLen, 2)
}

func TestClient_GetWithDifferentURLAndNoRedirect(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := NoRedirect
	srv1.setResponseJSON(&rootNode{
		Preferences: &preferences{
			URL:      srv2.config().BaseUrl,
			Redirect: &redirect,
		},
		Entries: map[string]*entry{
			"key": &entry{
				Value: "value1",
			},
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value2"))
	client.Refresh()

	// Check that the value still comes from the same server and
	// that no requests were made to the second server.
	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value1")

	c.Assert(srv2.allResponses(), qt.HasLen, 0)
}

func TestClient_GetWithRedirectToSameURL(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := ForceRedirect
	srv1.setResponseJSON(&rootNode{
		Preferences: &preferences{
			URL:      srv1.config().BaseUrl,
			Redirect: &redirect,
		},
		Entries: map[string]*entry{
			"key": &entry{
				Value: "value1",
			},
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value2"))
	client.Refresh()
	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value1")

	// Check that it hasn't made another request to the same server.
	c.Assert(srv1.allResponses(), qt.HasLen, 1)
}

func TestClient_GetWithCustomURLAndShouldRedirect(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := ShouldRedirect
	srv1.setResponseJSON(&rootNode{
		Preferences: &preferences{
			URL:      srv2.config().BaseUrl,
			Redirect: &redirect,
		},
		Entries: map[string]*entry{
			"key": &entry{
				Value: "value1",
			},
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value2"))
	client.Refresh()

	// Check that the value still comes from the same server and
	// that no requests were made to the second server.
	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value1")

	c.Assert(srv2.allResponses(), qt.HasLen, 0)
}

func TestClient_GetWithStandardURLAndShouldRedirect(t *testing.T) {
	c := qt.New(t)
	// Use a mock transport so that we can serve the request even though it's
	// going to a non localhost address.
	transport := newMockHTTPTransport()
	redirect := ShouldRedirect
	transport.enqueue(200, marshalJSON(&rootNode{
		Preferences: &preferences{
			URL:      "https://fakeUrl",
			Redirect: &redirect,
		},
	}))
	transport.enqueue(200, marshalJSON(rootNodeWithKeyValue("key", "value")))
	client := NewCustomClient("fakeKey", ClientConfig{
		Logger:    newTestLogger(t, LogLevelDebug),
		Transport: transport,
	})
	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value")
	c.Assert(transport.requests, qt.HasLen, 2)
	c.Assert(transport.requests[0].URL.Host, qt.Equals, strings.TrimPrefix(globalBaseUrl, "https://"))
	c.Assert(transport.requests[1].URL.Host, qt.Equals, "fakeUrl")
}

func TestClient_GetWithStandardURLAndNoRedirect(t *testing.T) {
	c := qt.New(t)
	// Use a mock transport so that we can serve the request even though it's
	// going to a non localhost address.
	transport := newMockHTTPTransport()
	redirect := NoRedirect
	transport.enqueue(200, marshalJSON(&rootNode{
		Preferences: &preferences{
			URL:      "https://fakeUrl",
			Redirect: &redirect,
		},
		Entries: map[string]*entry{
			"key": &entry{
				Value: "value1",
			},
		},
	}))
	client := NewCustomClient("fakeKey", ClientConfig{
		Logger:    newTestLogger(t, LogLevelDebug),
		Transport: transport,
	})
	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value1")

	transport.enqueue(200, marshalJSON(rootNodeWithKeyValue("key", "value2")))
	// The next request should go to the redirected server.
	client.Refresh()

	result = client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "value2")

	c.Assert(transport.requests, qt.HasLen, 2)
	c.Assert(transport.requests[0].URL.Host, qt.Equals, strings.TrimPrefix(globalBaseUrl, "https://"))
	c.Assert(transport.requests[1].URL.Host, qt.Equals, "fakeUrl")
}

func TestClient_GetWithRedirectLoop(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := ForceRedirect
	srv1.setResponseJSON(&rootNode{
		Preferences: &preferences{
			URL:      srv2.config().BaseUrl,
			Redirect: &redirect,
		},
	})
	srv2.setResponseJSON(&rootNode{
		Preferences: &preferences{
			URL:      srv1.config().BaseUrl,
			Redirect: &redirect,
		},
	})
	client.Refresh()

	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "default")
	c.Assert(srv1.allResponses(), qt.HasLen, 2)
	c.Assert(srv2.allResponses(), qt.HasLen, 1)
}

func TestClient_GetWithInvalidConfig(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: "invalid-json"})
	client.Refresh()
	result := client.GetValue("key", "default")
	c.Assert(result, qt.Equals, "default")
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

func getTestClients(t *testing.T) (*configServer, *Client) {
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.Mode = ManualPoll()
	client := NewCustomClient(srv.sdkKey(), cfg)
	t.Cleanup(client.Close)
	return srv, client
}

func rootNodeWithKeyValue(key string, value interface{}) *rootNode {
	return &rootNode{
		Entries: map[string]*entry{
			key: &entry{
				Value: value,
			},
		},
	}
}

type mockHTTPTransport struct {
	requests  []*http.Request
	responses []*http.Response
}

func newMockHTTPTransport() *mockHTTPTransport {
	return &mockHTTPTransport{}
}

func (m *mockHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)

	nextResponseInQueue := m.responses[0]
	m.responses = m.responses[1:]
	return nextResponseInQueue, nil
}

func (m *mockHTTPTransport) enqueue(statusCode int, body string) {
	m.responses = append(m.responses, &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(strings.NewReader(body)),
	})
}
