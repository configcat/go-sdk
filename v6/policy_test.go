package configcat

import (
	"fmt"
	"net/http"
	"sync"
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

	cache := &customCache{
		items: map[string]string{},
	}
	cfg.Cache = cache

	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()
	refresh(client)
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	// Check that the cache has been populated.
	c.Assert(cache.allItems(), qt.HasLen, 1)

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
	cache.setGetError(fmt.Errorf("cache failure"))
	time.Sleep(60 * time.Millisecond)
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	// Check that if the cache value changes, it's still consulted.
	cache.setGetError(nil)
	for key := range cache.allItems() {
		cache.Set(key, `{"test":2}`)
	}
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":2}`)
}

type customCache struct {
	mu       sync.Mutex
	getError error
	items    map[string]string
}

func (c *customCache) Get(key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.getError != nil {
		return "", c.getError
	}
	return c.items[key], nil
}

func (c *customCache) Set(key, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
	return nil
}

func (c *customCache) allItems() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	items := make(map[string]string)
	for key, val := range c.items {
		items[key] = val
	}
	return items
}

func (c *customCache) setGetError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getError = err
}
