package configcat

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// testPolicy_FetchFailWithCacheFallback tests that cache fallback behaviour
// works as expected for the given refresh mode.
// The refresh function will be called immediately after
// creating a client; the wait function will be called to wait
// for the config to expire.
func testPolicy_FetchFailWithCacheFallback(t *testing.T, mode RefreshMode, refresh, wait func(*Client)) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"test":1}`})

	// First use a client to populate the cache.
	cfg := srv.config()
	cfg.Mode = mode

	var cacheGetError error
	cacheItems := make(map[string]string)
	cfg.Cache = callbackCache{
		get: func(key string) (string, error) {
			if cacheGetError != nil {
				return "", cacheGetError
			}
			return cacheItems[key], nil
		},
		set: func(key, value string) error {
			cacheItems[key] = value
			return nil
		},
	}

	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()
	refresh(client)
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	// Check that the cache has been populated.
	c.Assert(cacheItems, qt.HasLen, 1)

	// Check that a new client can fetch the response from
	// the cache.
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})
	client = NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()
	refresh(client)
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	wait(client)
	// Check that the same client will fall back to the old
	// value even when the cache fails subsequently.
	cacheGetError = fmt.Errorf("cache failure")
	time.Sleep(60 * time.Millisecond)
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	// Check that if the cache value changes, it's still consulted.
	cacheGetError = nil
	for key := range cacheItems {
		cacheItems[key] = `{"test":2}`
	}
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":2}`)
}
