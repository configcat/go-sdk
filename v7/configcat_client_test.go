package configcat

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/configcat/go-sdk/v7/internal/wireconfig"
	qt "github.com/frankban/quicktest"
)

var variationConfig = &wireconfig.RootNode{
	Entries: map[string]*wireconfig.Entry{
		"first": {
			Value:       false,
			VariationID: "fakeIDFirst",
		},
		"second": {
			Value:       true,
			VariationID: "fakeIDSecond",
		},
	},
}

func TestClient_Refresh(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.PollingMode = Manual
	client := NewCustomClient(cfg)
	defer client.Close()

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value", wireconfig.StringEntry))
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)

	c.Assert(result, qt.Equals, "value")

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value2", wireconfig.StringEntry))
	client.Refresh(context.Background())
	result = client.GetStringValue("key", "default", nil)
	if result != "value2" {
		t.Error("Expecting non default string value")
	}
}

func TestClient_Refresh_Timeout(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.PollingMode = Manual
	client := NewCustomClient(cfg)
	defer client.Close()

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value", wireconfig.StringEntry))
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value")

	srv.setResponse(configResponse{
		body:  marshalJSON(rootNodeWithKeyValue("key", "value", wireconfig.StringEntry)),
		sleep: time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	t0 := time.Now()
	client.Refresh(ctx)
	if d := time.Since(t0); d < 10*time.Millisecond || d > 50*time.Millisecond {
		t.Errorf("refresh returned too quickly; got %v want >10ms, <50ms", d)
	}
	result = client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value")
}

func TestClient_Float(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213, wireconfig.FloatEntry))
	client.Refresh(context.Background())
	result := client.GetFloatValue("key", 0, nil)
	c.Assert(result, qt.Equals, 3213.0)
}

func TestClient_Get_IsOneOf_Does_Not_Use_Contains_Semantics(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(&wireconfig.RootNode{
		Entries: map[string]*wireconfig.Entry{
			"feature": {
				Value:       false,
				VariationID: "a377be39",
				RolloutRules: []*wireconfig.RolloutRule{{
					Comparator:          wireconfig.OpOneOf,
					ComparisonAttribute: "Identifier",
					ComparisonValue:     "example,foobar",
					Value:               true,
					VariationID:         "8bcf8608",
				}},
			},
		},
	})
	client.Refresh(context.Background())

	matchingUser := &UserData{Identifier: "mple"}
	result := client.GetBoolValue("feature", false, matchingUser)
	c.Assert(result, qt.IsFalse)

	matchingUser = &UserData{Identifier: "foobar"}
	result = client.GetBoolValue("feature", false, matchingUser)
	c.Assert(result, qt.IsTrue)

	matchingUser = &UserData{Identifier: "nonexisting"}
	result = client.GetBoolValue("feature", false, matchingUser)
	c.Assert(result, qt.IsFalse)
}

func TestClient_Get_Default(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})
	result := client.GetFloatValue("key", 0, nil)
	c.Assert(result, qt.Equals, 0.0)
}

func TestClient_Get_Latest(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213, wireconfig.FloatEntry))
	client.Refresh(context.Background())

	result := client.GetFloatValue("key", 0, nil)
	c.Assert(result, qt.Equals, 3213.0)

	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})

	result = client.GetFloatValue("key", 0, nil)
	c.Assert(result, qt.Equals, 3213.0)
}

func TestClient_Get_WithFailingCacheSet(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.Cache = failingCache{}
	client := NewCustomClient(cfg)
	defer client.Close()

	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213, wireconfig.FloatEntry))
	client.Refresh(context.Background())
	result := client.GetFloatValue("key", 0, nil)
	c.Assert(result, qt.Equals, 3213.0)
}

func TestClient_Get_WithEmptySDKKey(t *testing.T) {
	c := qt.New(t)
	client := NewClient("")
	err := client.Refresh(context.Background())
	c.Assert(err, qt.ErrorMatches, `config fetch failed: empty SDK key in configcat configuration!`)
}

func TestClient_Get_WithEmptyKey(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(variationConfig)
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())
	c.Assert(client.GetBoolValue("", false, nil), qt.Equals, false)
}

func TestClient_Keys(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	keys := client.GetAllKeys()
	c.Assert(keys, qt.HasLen, 16)
}

func TestClient_VariationID(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(variationConfig)
	client.Refresh(context.Background())
	result := client.GetVariationID("first", "", nil)
	c.Assert(result, qt.Equals, "fakeIDFirst")
}

func TestClient_VariationID_Default(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(variationConfig)
	client.Refresh(context.Background())
	result := client.GetVariationID("nonexisting", "def", nil)
	c.Assert(result, qt.Equals, "def")
}

func TestClient_GetAllVariationIDs(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(variationConfig)
	client.Refresh(context.Background())
	result := client.GetVariationIDs(nil)
	c.Assert(result, qt.HasLen, 2)
}

func TestClient_VariationIDs_Empty(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: `{ "f": {} }`})
	client.Refresh(context.Background())
	result := client.GetVariationIDs(nil)
	c.Assert(result, qt.HasLen, 0)
}

func TestClient_GetKeyAndValue(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(variationConfig)
	client.Refresh(context.Background())
	key, value := client.GetKeyValueForVariationID("fakeIDSecond")
	c.Assert(key, qt.Equals, "second")
	c.Assert(value, qt.Equals, true)
}

func TestClient_GetKeyAndValue_Empty(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(variationConfig)
	client.Refresh(context.Background())
	key, value := client.GetKeyValueForVariationID("nonexisting")
	c.Assert(key, qt.Equals, "")
	c.Assert(value, qt.Equals, nil)
}

func TestClient_GetWithRedirectSuccess(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := wireconfig.ForceRedirect
	srv1.setResponseJSON(&wireconfig.RootNode{
		Preferences: &wireconfig.Preferences{
			URL:      srv2.config().BaseURL,
			Redirect: &redirect,
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value", wireconfig.StringEntry))
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value")
	c.Assert(srv1.allResponses(), qt.HasLen, 1)
	c.Assert(srv2.allResponses(), qt.HasLen, 1)

	// Another request should go direct to the second server.
	client.Refresh(context.Background())
	c.Assert(srv1.allResponses(), qt.HasLen, 1)
	c.Assert(srv2.allResponses(), qt.HasLen, 2)
}

func TestClient_GetWithDifferentURLAndNoRedirect(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := wireconfig.Nodirect
	srv1.setResponseJSON(&wireconfig.RootNode{
		Preferences: &wireconfig.Preferences{
			URL:      srv2.config().BaseURL,
			Redirect: &redirect,
		},
		Entries: map[string]*wireconfig.Entry{
			"key": &wireconfig.Entry{
				Value: "value1",
			},
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value2", wireconfig.StringEntry))
	client.Refresh(context.Background())

	// Check that the value still comes from the same server and
	// that no requests were made to the second server.
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value1")

	c.Assert(srv2.allResponses(), qt.HasLen, 0)
}

func TestClient_GetWithRedirectToSameURL(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := wireconfig.ForceRedirect
	srv1.setResponseJSON(&wireconfig.RootNode{
		Preferences: &wireconfig.Preferences{
			URL:      srv1.config().BaseURL,
			Redirect: &redirect,
		},
		Entries: map[string]*wireconfig.Entry{
			"key": &wireconfig.Entry{
				Value: "value1",
			},
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value2", wireconfig.StringEntry))
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value1")

	// Check that it hasn't made another request to the same server.
	c.Assert(srv1.allResponses(), qt.HasLen, 1)
}

func TestClient_GetWithCustomURLAndShouldRedirect(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := wireconfig.ShouldRedirect
	srv1.setResponseJSON(&wireconfig.RootNode{
		Preferences: &wireconfig.Preferences{
			URL:      srv2.config().BaseURL,
			Redirect: &redirect,
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value2", wireconfig.StringEntry))
	err := client.Refresh(context.Background())
	c.Assert(err, qt.ErrorMatches, "config fetch failed: refusing to redirect from custom URL without forced redirection")

	// Check that it still behaves as if there was no configuration
	// fetched.
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "default")

	c.Assert(srv2.allResponses(), qt.HasLen, 0)
}

func TestClient_GetWithStandardURLAndShouldRedirect(t *testing.T) {
	c := qt.New(t)
	// Use a mock transport so that we can serve the request even though it's
	// going to a non localhost address.
	transport := newMockHTTPTransport()
	redirect := wireconfig.ShouldRedirect
	transport.enqueue(200, marshalJSON(&wireconfig.RootNode{
		Preferences: &wireconfig.Preferences{
			URL:      "https://fakeUrl",
			Redirect: &redirect,
		},
	}))
	transport.enqueue(200, marshalJSON(rootNodeWithKeyValue("key", "value", wireconfig.StringEntry)))
	client := NewCustomClient(Config{
		SDKKey:    "fakeKey",
		Logger:    newTestLogger(t, LogLevelDebug),
		Transport: transport,
	})
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value")
	c.Assert(transport.requests, qt.HasLen, 2)
	c.Assert(transport.requests[0].URL.Host, qt.Equals, strings.TrimPrefix(globalBaseURL, "https://"))
	c.Assert(transport.requests[1].URL.Host, qt.Equals, "fakeUrl")
}

func TestClient_GetWithStandardURLAndNoRedirect(t *testing.T) {
	c := qt.New(t)
	// Use a mock transport so that we can serve the request even though it's
	// going to a non localhost address.
	transport := newMockHTTPTransport()
	redirect := wireconfig.Nodirect
	transport.enqueue(200, marshalJSON(&wireconfig.RootNode{
		Preferences: &wireconfig.Preferences{
			URL:      "https://fakeUrl",
			Redirect: &redirect,
		},
		Entries: map[string]*wireconfig.Entry{
			"key": &wireconfig.Entry{
				Value: "value1",
			},
		},
	}))
	client := NewCustomClient(Config{
		SDKKey:    "fakeKey",
		Logger:    newTestLogger(t, LogLevelDebug),
		Transport: transport,
	})
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value1")

	transport.enqueue(200, marshalJSON(rootNodeWithKeyValue("key", "value2", wireconfig.StringEntry)))
	// The next request should go to the redirected server.
	client.Refresh(context.Background())

	result = client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value2")

	c.Assert(transport.requests, qt.HasLen, 2)
	c.Assert(transport.requests[0].URL.Host, qt.Equals, strings.TrimPrefix(globalBaseURL, "https://"))
	c.Assert(transport.requests[1].URL.Host, qt.Equals, "fakeUrl")
}

func TestClient_GetWithRedirectLoop(t *testing.T) {
	c := qt.New(t)
	srv1, client := getTestClients(t)
	srv2, _ := getTestClients(t)
	srv2.key = srv1.key
	redirect := wireconfig.ForceRedirect
	srv1.setResponseJSON(&wireconfig.RootNode{
		Preferences: &wireconfig.Preferences{
			URL:      srv2.config().BaseURL,
			Redirect: &redirect,
		},
	})
	srv2.setResponseJSON(&wireconfig.RootNode{
		Preferences: &wireconfig.Preferences{
			URL:      srv1.config().BaseURL,
			Redirect: &redirect,
		},
	})
	client.Refresh(context.Background())

	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "default")
	c.Assert(srv1.allResponses(), qt.HasLen, 2)
	c.Assert(srv2.allResponses(), qt.HasLen, 1)
}

func TestClient_GetWithInvalidConfig(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: "invalid-json"})
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "default")
}

func TestClient_GetInt(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", 99, wireconfig.IntEntry))
	client.Refresh(context.Background())
	c.Check(client.GetIntValue("key", 0, nil), qt.Equals, 99)
}

func TestSnapshot_Get(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", 99, wireconfig.IntEntry))
	client.Refresh(context.Background())
	c.Check(client.Snapshot(nil).GetValue("key"), qt.Equals, 99)
}

type failingCache struct{}

// get reads the configuration from the cache.
func (cache failingCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, errors.New("fake failing cache fails to get")
}

// set writes the configuration into the cache.
func (cache failingCache) Set(ctx context.Context, key string, value []byte) error {
	return errors.New("fake failing cache fails to set")
}

func getTestClients(t *testing.T) (*configServer, *Client) {
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.PollingMode = Manual
	client := NewCustomClient(cfg)
	t.Cleanup(client.Close)
	return srv, client
}

func rootNodeWithKeyValue(key string, value interface{}, typ wireconfig.EntryType) *wireconfig.RootNode {
	return &wireconfig.RootNode{
		Entries: map[string]*wireconfig.Entry{
			key: &wireconfig.Entry{
				Value: value,
				Type:  typ,
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
