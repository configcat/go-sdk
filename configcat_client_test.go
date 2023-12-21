package configcat

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

var variationConfig = &ConfigJson{
	Settings: map[string]*Setting{
		"first": {
			Value:       &SettingValue{Value: false},
			VariationID: "fakeIDFirst",
		},
		"second": {
			Value:       &SettingValue{Value: true},
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

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value"))
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)

	c.Assert(result, qt.Equals, "value")

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value2"))
	client.Refresh(context.Background())
	result = client.GetStringValue("key", "default", nil)
	if result != "value2" {
		t.Error("Expecting non default string value")
	}
}

func TestClient_Refresh_Canceled(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.PollingMode = Manual
	client := NewCustomClient(cfg)
	defer client.Close()

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value"))
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value")

	srv.setResponse(configResponse{
		body:  marshalJSON(rootNodeWithKeyValue("key", "value")),
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

func TestClient_Refresh_Timeout(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.PollingMode = Manual
	cfg.HTTPTimeout = 10 * time.Millisecond
	client := NewCustomClient(cfg)
	defer client.Close()

	srv.setResponse(configResponse{
		body:  marshalJSON(rootNodeWithKeyValue("key", "value")),
		sleep: 20 * time.Millisecond,
	})
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "default")
}

func TestClient_Float(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213.0))
	client.Refresh(context.Background())
	result := client.GetFloatValue("key", 0, nil)
	c.Assert(result, qt.Equals, 3213.0)
}

func TestClient_KeyNotFound(t *testing.T) {
	c := qt.New(t)
	// By creating this flag first, we ensure that its key ID is already
	// allocated when the configuration is parsed, so we test the
	// path when precalculated slots has no entry for a key.
	Bool("k1", false)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("k2", 3213))
	client.Refresh(context.Background())
	result := client.GetIntValue("k1", 0, nil)
	c.Assert(result, qt.Equals, 0)
}

func TestClient_Get_IsOneOf_Does_Not_Use_Contains_Semantics(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(&ConfigJson{
		Settings: map[string]*Setting{
			"feature": {
				Value:       &SettingValue{Value: false},
				VariationID: "a377be39",
				TargetingRules: []*TargetingRule{{
					Conditions: []*Condition{{
						UserCondition: &UserCondition{
							Comparator:          OpOneOf,
							ComparisonAttribute: "Identifier",
							StringArrayValue:    []string{"example", "foobar"},
						},
					}},
					ServedValue: &ServedValue{
						Value:       &SettingValue{Value: true},
						VariationID: "8bcf8608",
					},
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
	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213.0))
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

	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213.0))
	client.Refresh(context.Background())
	result := client.GetFloatValue("key", 0, nil)
	c.Assert(result, qt.Equals, 3213.0)
}

func TestClient_Get_WithEmptySDKKey(t *testing.T) {
	c := qt.New(t)
	client := NewClient("")
	err := client.Refresh(context.Background())
	c.Assert(err, qt.ErrorMatches, `config fetch failed: SDK Key is invalid`)
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

func TestClient_AllValues(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	keys := client.GetAllValues(nil)
	c.Assert(keys, qt.HasLen, 16)
}

func TestClient_VariationID(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(variationConfig)
	client.Refresh(context.Background())
	result := client.GetBoolValueDetails("first", false, nil)
	c.Assert(result.Data.VariationID, qt.Equals, "fakeIDFirst")
}

func TestClient_VariationID_Default(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(variationConfig)
	client.Refresh(context.Background())
	result := client.GetBoolValueDetails("nonexisting", false, nil)
	c.Assert(result.Data.VariationID, qt.Equals, "")
}

func TestClient_GetAllVariationIDs(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(variationConfig)
	client.Refresh(context.Background())
	result := client.GetAllValueDetails(nil)
	c.Assert(result, qt.HasLen, 2)
}

func TestClient_VariationIDs_Empty(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{body: `{ "f": {} }`})
	client.Refresh(context.Background())
	result := client.GetAllValueDetails(nil)
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
	redirect := ForceRedirect
	srv1.setResponseJSON(&ConfigJson{
		Preferences: &Preferences{
			URL:      srv2.config().BaseURL,
			Redirect: &redirect,
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value"))
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
	redirect := NoDirect
	srv1.setResponseJSON(&ConfigJson{
		Preferences: &Preferences{
			URL:      srv2.config().BaseURL,
			Redirect: &redirect,
		},
		Settings: map[string]*Setting{
			"key": {
				Value: &SettingValue{Value: "value1"},
				Type:  StringSetting,
			},
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value2"))
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
	redirect := ForceRedirect
	srv1.setResponseJSON(&ConfigJson{
		Preferences: &Preferences{
			URL:      srv1.config().BaseURL,
			Redirect: &redirect,
		},
		Settings: map[string]*Setting{
			"key": {
				Value: &SettingValue{Value: "value1"},
				Type:  StringSetting,
			},
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value2"))
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
	redirect := ShouldRedirect
	srv1.setResponseJSON(&ConfigJson{
		Preferences: &Preferences{
			URL:      srv2.config().BaseURL,
			Redirect: &redirect,
		},
	})
	srv2.setResponseJSON(rootNodeWithKeyValue("key", "value2"))
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
	redirect := ShouldRedirect
	transport.enqueue(200, marshalJSON(&ConfigJson{
		Preferences: &Preferences{
			URL:      "https://fakeUrl",
			Redirect: &redirect,
		},
	}))
	transport.enqueue(200, marshalJSON(rootNodeWithKeyValue("key", "value")))
	client := NewCustomClient(Config{
		SDKKey:    randomSdkKey(),
		Logger:    newTestLogger(t),
		LogLevel:  LogLevelDebug,
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
	redirect := NoDirect
	transport.enqueue(200, marshalJSON(&ConfigJson{
		Preferences: &Preferences{
			URL:      "https://fakeUrl",
			Redirect: &redirect,
		},
		Settings: map[string]*Setting{
			"key": {
				Value: &SettingValue{Value: "value1"},
				Type:  StringSetting,
			},
		},
	}))
	client := NewCustomClient(Config{
		SDKKey:    randomSdkKey(),
		Logger:    newTestLogger(t),
		LogLevel:  LogLevelDebug,
		Transport: transport,
	})
	client.Refresh(context.Background())
	result := client.GetStringValue("key", "default", nil)
	c.Assert(result, qt.Equals, "value1")

	transport.enqueue(200, marshalJSON(rootNodeWithKeyValue("key", "value2")))
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
	redirect := ForceRedirect
	srv1.setResponseJSON(&ConfigJson{
		Preferences: &Preferences{
			URL:      srv2.config().BaseURL,
			Redirect: &redirect,
		},
	})
	srv2.setResponseJSON(&ConfigJson{
		Preferences: &Preferences{
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
	srv.setResponseJSON(rootNodeWithKeyValue("key", 99))
	client.Refresh(context.Background())
	c.Check(client.GetIntValue("key", 0, nil), qt.Equals, 99)
}

func TestClient_DefaultUser(t *testing.T) {
	type user struct {
		Cluster string `configcat:"cluster"`
	}
	c := qt.New(t)
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.PollingMode = Manual
	u := &user{
		Cluster: "somewhere",
	}
	cfg.DefaultUser = u
	client := NewCustomClient(cfg)
	t.Cleanup(client.Close)

	srv.setResponseJSON(&ConfigJson{
		Settings: map[string]*Setting{
			"foo": {
				Value: &SettingValue{Value: "default"},
				Type:  StringSetting,
				TargetingRules: []*TargetingRule{{
					Conditions: []*Condition{{
						UserCondition: &UserCondition{
							ComparisonAttribute: "cluster",
							Comparator:          OpOneOf,
							StringArrayValue:    []string{"somewhere"},
						},
					}},
					ServedValue: &ServedValue{
						Value: &SettingValue{Value: "somewhere-match"},
					},
				}},
			},
		},
	})
	client.Refresh(context.Background())
	c.Check(client.GetStringValue("foo", "", nil), qt.Equals, "somewhere-match")

	snap := client.Snapshot(nil)
	fooFlag := String("foo", "")
	c.Check(fooFlag.Get(snap), qt.Equals, "somewhere-match")

	snap = snap.WithUser(nil)
	c.Check(fooFlag.Get(snap), qt.Equals, "somewhere-match")

	snap = snap.WithUser(u)
	c.Check(fooFlag.Get(snap), qt.Equals, "somewhere-match")

	snap = snap.WithUser(&user{
		Cluster: "otherwhere",
	})
	c.Check(fooFlag.Get(snap), qt.Equals, "default")
}

func TestSnapshot_Get(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", 99))
	client.Refresh(context.Background())
	snap := client.Snapshot(nil)
	c.Check(snap.GetValue("key"), qt.Equals, 99)
	c.Check(snap.FetchTime(), qt.Not(qt.Equals), time.Time{})
	srv.setResponseJSON(rootNodeWithKeyValue("key", 101))
	time.Sleep(1 * time.Millisecond) // wait a bit to ensure fetch times don't collide
	client.Refresh(context.Background())
	c.Check(snap.GetValue("key"), qt.Equals, 99)
	c.Check(client.Snapshot(nil).GetValue("key"), qt.Equals, 101)
	c.Check(client.Snapshot(nil).FetchTime().After(snap.FetchTime()), qt.IsTrue)
}

func TestClient_GetBoolDetails(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	user := &UserData{Identifier: "a@configcat.com", Email: "a@configcat.com"}

	details := client.GetBoolValueDetails("bool30TrueAdvancedRules", true, user)
	c.Assert(details.Value, qt.IsFalse)
	c.Assert(details.Data.IsDefaultValue, qt.IsFalse)
	c.Assert(details.Data.Error, qt.IsNil)
	c.Assert(details.Data.Key, qt.Equals, "bool30TrueAdvancedRules")
	c.Assert(details.Data.User, qt.Equals, user)
	c.Assert(details.Data.VariationID, qt.Equals, "385d9803")
	c.Assert(details.Data.MatchedPercentageOption, qt.IsNil)
	c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.Comparator, qt.Equals, OpOneOf)
	c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.ComparisonAttribute, qt.Equals, "Email")
	c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.StringArrayValue, qt.DeepEquals, []string{"a@configcat.com", "b@configcat.com"})
}

func TestClient_GetStringDetails(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	user := &UserData{Identifier: "a@configcat.com", Email: "a@configcat.com"}

	details := client.GetStringValueDetails("stringContainsDogDefaultCat", "", user)
	c.Assert(details.Value, qt.Equals, "Dog")
	c.Assert(details.Data.IsDefaultValue, qt.IsFalse)
	c.Assert(details.Data.Error, qt.IsNil)
	c.Assert(details.Data.Key, qt.Equals, "stringContainsDogDefaultCat")
	c.Assert(details.Data.User, qt.Equals, user)
	c.Assert(details.Data.VariationID, qt.Equals, "d0cd8f06")
	c.Assert(details.Data.MatchedPercentageOption, qt.IsNil)
	c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.Comparator, qt.Equals, OpContains)
	c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.ComparisonAttribute, qt.Equals, "Email")
	c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.StringArrayValue, qt.DeepEquals, []string{"@configcat.com"})
}

func TestClient_GetIntDetails(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	user := &UserData{Identifier: "a@configcat.com"}

	details := client.GetIntValueDetails("integer25One25Two25Three25FourAdvancedRules", 0, user)
	c.Assert(details.Value, qt.Equals, 1)
	c.Assert(details.Data.IsDefaultValue, qt.IsFalse)
	c.Assert(details.Data.Error, qt.IsNil)
	c.Assert(details.Data.Key, qt.Equals, "integer25One25Two25Three25FourAdvancedRules")
	c.Assert(details.Data.User, qt.Equals, user)
	c.Assert(details.Data.VariationID, qt.Equals, "11634414")
	c.Assert(details.Data.MatchedPercentageOption.Percentage, qt.Equals, int64(25))
}

func TestClient_GetFloatDetails(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	user := &UserData{Identifier: "a@configcat.com", Email: "a@configcat.com"}

	details := client.GetFloatValueDetails("double25Pi25E25Gr25Zero", 0.0, user)
	c.Assert(details.Value, qt.Equals, 5.561)
	c.Assert(details.Data.IsDefaultValue, qt.IsFalse)
	c.Assert(details.Data.Error, qt.IsNil)
	c.Assert(details.Data.Key, qt.Equals, "double25Pi25E25Gr25Zero")
	c.Assert(details.Data.User, qt.Equals, user)
	c.Assert(details.Data.VariationID, qt.Equals, "3f7826de")
	c.Assert(details.Data.MatchedPercentageOption, qt.IsNil)
	c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.Comparator, qt.Equals, OpContains)
	c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.ComparisonAttribute, qt.Equals, "Email")
	c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.StringArrayValue, qt.DeepEquals, []string{"@configcat.com"})
}

func TestClient_GetAllDetails(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	user := &UserData{Identifier: "a@configcat.com", Email: "a@configcat.com"}

	keys := client.GetAllKeys()
	details := client.GetAllValueDetails(user)
	c.Assert(len(details), qt.Equals, len(keys))
}

func TestClient_GetBoolDetails_NotExist(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	details := client.GetBoolValueDetails("non-existent", false, nil)
	_, ok := details.Data.Error.(ErrKeyNotFound)
	c.Assert(ok, qt.IsTrue)
	c.Assert(details.Value, qt.IsFalse)
}

func TestClient_GetStringDetails_NotExist(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	details := client.GetStringValueDetails("non-existent", "", nil)
	_, ok := details.Data.Error.(ErrKeyNotFound)
	c.Assert(ok, qt.IsTrue)
	c.Assert(details.Value, qt.Equals, "")
}

func TestClient_GetIntDetails_NotExist(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	details := client.GetIntValueDetails("non-existent", 0, nil)
	_, ok := details.Data.Error.(ErrKeyNotFound)
	c.Assert(ok, qt.IsTrue)
	c.Assert(details.Value, qt.Equals, 0)
}

func TestClient_GetFloatDetails_NotExist(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	details := client.GetFloatValueDetails("non-existent", 0, nil)
	_, ok := details.Data.Error.(ErrKeyNotFound)
	c.Assert(ok, qt.IsTrue)
	c.Assert(details.Value, qt.Equals, float64(0))
}

func TestClient_GetDetails_Reflected_User(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	user := &struct{ attr string }{"a"}

	details := client.GetFloatValueDetails("double25Pi25E25Gr25Zero", 0.0, user)
	c.Assert(details.Data.User, qt.Equals, user)
	c.Assert(srv.requestCount, qt.Equals, 1)
}

func TestClient_Hooks_OnFlagEvaluated(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})

	user := &UserData{Identifier: "a@configcat.com", Email: "a@configcat.com"}

	called := make(chan struct{})
	cfg := srv.config()
	cfg.Hooks = &Hooks{OnFlagEvaluated: func(details *EvaluationDetails) {
		c.Assert(details.Value, qt.Equals, 5.561)
		c.Assert(details.Data.IsDefaultValue, qt.IsFalse)
		c.Assert(details.Data.Error, qt.IsNil)
		c.Assert(details.Data.Key, qt.Equals, "double25Pi25E25Gr25Zero")
		c.Assert(details.Data.User, qt.Equals, user)
		c.Assert(details.Data.VariationID, qt.Equals, "3f7826de")
		c.Assert(details.Data.MatchedPercentageOption, qt.IsNil)
		c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.Comparator, qt.Equals, OpContains)
		c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.ComparisonAttribute, qt.Equals, "Email")
		c.Assert(details.Data.MatchedTargetingRule.Conditions[0].UserCondition.StringArrayValue, qt.DeepEquals, []string{"@configcat.com"})
		called <- struct{}{}
	}}
	client := NewCustomClient(cfg)
	client.Refresh(context.Background())

	_ = client.GetFloatValue("double25Pi25E25Gr25Zero", 0.0, user)

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatalf("timed out")
	}
}

func TestClient_Ready_Signal(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})

	user := &UserData{Identifier: "a@configcat.com", Email: "a@configcat.com"}

	cfg := srv.config()
	client := NewCustomClient(cfg)

	select {
	case <-client.Ready():
	case <-time.After(time.Second):
		t.Fatalf("timed out")
	}

	val := client.GetFloatValue("double25Pi25E25Gr25Zero", 0.0, user)
	c.Assert(val, qt.Equals, 5.561)
}

func TestClient_Ready_Signal_NoWait(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})

	user := &UserData{Identifier: "a@configcat.com", Email: "a@configcat.com"}

	cfg := srv.config()
	cfg.NoWaitForRefresh = true
	client := NewCustomClient(cfg)

	select {
	case <-client.Ready():
	case <-time.After(time.Second):
		t.Fatalf("timed out")
	}

	val := client.GetFloatValue("double25Pi25E25Gr25Zero", 0.0, user)
	c.Assert(val, qt.Equals, 5.561)
}

func TestClient_Ready_Signal_Manual(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})

	user := &UserData{Identifier: "a@configcat.com", Email: "a@configcat.com"}

	cfg := srv.config()
	cfg.PollingMode = Manual
	client := NewCustomClient(cfg)

	select {
	case <-client.Ready():
	case <-time.After(time.Second):
		t.Fatalf("timed out")
	}

	val := client.GetFloatValue("double25Pi25E25Gr25Zero", 0.0, user)
	c.Assert(val, qt.Equals, 0.0)

	client.Refresh(context.Background())

	val = client.GetFloatValue("double25Pi25E25Gr25Zero", 0.0, user)
	c.Assert(val, qt.Equals, 5.561)
}

func TestClient_Ready_Signal_Lazy(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})

	user := &UserData{Identifier: "a@configcat.com", Email: "a@configcat.com"}

	cfg := srv.config()
	cfg.PollingMode = Lazy
	client := NewCustomClient(cfg)

	select {
	case <-client.Ready():
	case <-time.After(time.Second):
		t.Fatalf("timed out")
	}

	val := client.GetFloatValue("double25Pi25E25Gr25Zero", 0.0, user)
	c.Assert(val, qt.Equals, 5.561)
}

func TestClient_InitOffline(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	config := srv.config()
	config.Offline = true
	client := NewCustomClient(config)
	client.Refresh(context.Background())

	c.Assert(client.IsOffline(), qt.IsTrue)

	client.Refresh(context.Background())

	c.Assert(srv.requestCount, qt.Equals, 0)

	client.SetOnline()
	c.Assert(client.IsOffline(), qt.IsFalse)

	client.Refresh(context.Background())

	c.Assert(srv.requestCount, qt.Equals, 1)
}

func TestClient_OfflineOnlineMode(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body: contentForIntegrationTestKey("PKDVCLf-Hq-h-kCzMp-L7Q/psuH7BGHoUmdONrzzUOY7A"),
	})
	client := NewCustomClient(srv.config())
	client.Refresh(context.Background())

	c.Assert(srv.requestCount, qt.Equals, 1)
	c.Assert(client.IsOffline(), qt.IsFalse)

	client.SetOffline()
	c.Assert(client.IsOffline(), qt.IsTrue)

	client.Refresh(context.Background())

	c.Assert(srv.requestCount, qt.Equals, 1)

	client.SetOnline()
	c.Assert(client.IsOffline(), qt.IsFalse)

	client.Refresh(context.Background())

	c.Assert(srv.requestCount, qt.Equals, 2)
}

func TestCacheKey(t *testing.T) {
	c := qt.New(t)
	tests := []struct {
		key      string
		cacheKey string
	}{
		{"test1", "7f845c43ecc95e202b91e271435935e6d1391e5d"},
		{"test2", "a78b7e323ef543a272c74540387566a22415148a"},
	}

	l := newTestLogger(t)
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			f := newConfigFetcher(Config{SDKKey: test.key, PollingMode: Manual}, newLeveledLogger(l, LogLevelWarn, nil), nil).(*configFetcher)
			c.Assert(f.cacheKey, qt.Equals, test.cacheKey)
		})
	}
}

func TestSdkKeyValidation(t *testing.T) {
	tests := []struct {
		key    string
		custom bool
		valid  bool
	}{
		{"sdk-key-90123456789012", false, false},
		{"sdk-key-9012345678901/1234567890123456789012", false, false},
		{"sdk-key-90123456789012/123456789012345678901", false, false},
		{"sdk-key-90123456789012/12345678901234567890123", false, false},
		{"sdk-key-901234567890123/1234567890123456789012", false, false},
		{"sdk-key-90123456789012/1234567890123456789012", false, true},
		{"configcat-sdk-1/sdk-key-90123456789012", false, false},
		{"configcat-sdk-1/sdk-key-9012345678901/1234567890123456789012", false, false},
		{"configcat-sdk-1/sdk-key-90123456789012/123456789012345678901", false, false},
		{"configcat-sdk-1/sdk-key-90123456789012/12345678901234567890123", false, false},
		{"configcat-sdk-1/sdk-key-901234567890123/1234567890123456789012", false, false},
		{"configcat-sdk-1/sdk-key-90123456789012/1234567890123456789012", false, true},
		{"configcat-sdk-2/sdk-key-90123456789012/1234567890123456789012", false, false},
		{"configcat-proxy/", false, false},
		{"configcat-proxy/", true, false},
		{"configcat-proxy/sdk-key-90123456789012", false, false},
		{"configcat-proxy/sdk-key-90123456789012", true, true},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			cfg := Config{
				SDKKey:      test.key,
				PollingMode: Manual,
				Logger:      newTestLogger(t),
				LogLevel:    LogLevelError,
			}
			if test.custom {
				cfg.BaseURL = "https://my-configcat-proxy"
			}
			client := NewCustomClient(cfg)
			if test.valid {
				qt.Assert(t, reflect.TypeOf(client.fetcher), qt.Equals, reflect.TypeOf((*configFetcher)(nil)))
			} else {
				qt.Assert(t, reflect.TypeOf(client.fetcher), qt.Equals, reflect.TypeOf(&emptyFetcher{}))
			}

			client.Close()
		})
	}
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
	cfg.Logger = DefaultLogger()
	cfg.LogLevel = LogLevelError
	client := NewCustomClient(cfg)
	t.Cleanup(client.Close)
	return srv, client
}

func rootNodeWithKeyValue(key string, value interface{}) *ConfigJson {
	return &ConfigJson{
		Settings: map[string]*Setting{
			key: {
				Value: &SettingValue{Value: value},
			},
		},
	}
}

func rootNodeWithKeyValueType(key string, value interface{}, t SettingType) *ConfigJson {
	return &ConfigJson{
		Settings: map[string]*Setting{
			key: {
				Value: &SettingValue{Value: value},
				Type:  t,
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
