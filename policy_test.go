package configcat

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/configcat/go-sdk/v9/configcatcache"

	qt "github.com/frankban/quicktest"
)

// TestFetchFailWithCacheFallback tests that cache fallback behavior
// works as expected.
func TestFetchFailWithCacheFallback(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"f":{}}`})

	// First use a client to populate the cache.
	cfg := srv.config()
	cfg.PollInterval = 10 * time.Millisecond

	cache := &customCache{
		items: map[string]string{},
	}
	cfg.Cache = cache

	client := NewCustomClient(cfg)
	defer client.Close()
	_ = client.Refresh(context.Background())
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"f":{}}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := configcatcache.CacheSegmentsFromBytes(cached)
		c.Assert(string(b), qt.Equals, `{"f":{}}`)
	}

	// Check that the cache has been populated.
	c.Assert(cache.allItems(), qt.HasLen, 1)

	c.Logf("cache populated")
	// Prevent the first client from changing the cache again.
	client.Close()

	// Check that a new client can fetch the response from
	// the cache.
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})
	client = NewCustomClient(cfg)
	client.Refresh(context.Background())
	defer client.Close()
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"f":{}}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := configcatcache.CacheSegmentsFromBytes(cached)
		c.Assert(string(b), qt.Equals, `{"f":{}}`)
	}

	time.Sleep(20 * time.Millisecond)
	// Check that the same client will fall back to the old
	// value even when the cache fails subsequently.
	cache.setGetError(fmt.Errorf("cache failure"))
	time.Sleep(60 * time.Millisecond)
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"f":{}}`)

	// Check that if the cache value changes, it's still consulted.
	cache.setGetError(nil)
	for key := range cache.allItems() {
		cache.Set(context.Background(), key, configcatcache.CacheSegmentsToBytes(time.Now(), "etag", []byte(`{"p":{},"f":{}}`)))
	}
	time.Sleep(20 * time.Millisecond)
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"p":{},"f":{}}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := configcatcache.CacheSegmentsFromBytes(cached)
		c.Assert(string(b), qt.Equals, `{"p":{},"f":{}}`)
	}

	// Check that we still get the value from the server when it recovers.
	srv.setResponse(configResponse{body: `{"p":{},"f":{},"s":[]}`})
	time.Sleep(20 * time.Millisecond)
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"p":{},"f":{},"s":[]}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := configcatcache.CacheSegmentsFromBytes(cached)
		c.Assert(string(b), qt.Equals, `{"p":{},"f":{},"s":[]}`)
	}
}

func Test_Consistent_Cache(t *testing.T) {
	c := qt.New(t)
	cacheEntry := `1686756435844
test-etag
{"p":{"u":"https://cdn-global.configcat.com","r":0,"s":"FUkC6RADjzF0vXrDSfJn7BcEBag9afw1Y6jkqjMP9BA="},"f":{"testKey":{"t":1,"v":{"s":"testValue"}}}}`
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})
	cfg := srv.config()
	cfg.PollInterval = 10 * time.Millisecond

	cache := &simpleCache{entry: []byte(cacheEntry)}
	cfg.Cache = cache

	client := NewCustomClient(cfg)
	defer client.Close()

	details := client.GetStringValueDetails("testKey", "", nil)
	c.Assert(details.Data.FetchTime.UnixMilli(), qt.Equals, int64(1686756435844))
	c.Assert(details.Value, qt.Equals, "testValue")
	cached, _ := cache.Get(context.Background(), "")
	ft, etag, _, _ := configcatcache.CacheSegmentsFromBytes(cached)
	c.Assert(ft.UnixMilli(), qt.Equals, int64(1686756435844))
	c.Assert(etag, qt.Equals, "test-etag")

	tn, _ := time.Parse(time.RFC3339Nano, "2023-06-14T15:27:15.8440000Z")
	serialized := configcatcache.CacheSegmentsToBytes(tn, "test-etag", []byte(`{"p":{"u":"https://cdn-global.configcat.com","r":0,"s":"FUkC6RADjzF0vXrDSfJn7BcEBag9afw1Y6jkqjMP9BA="},"f":{"testKey":{"t":1,"v":{"s":"testValue"}}}}`))
	c.Assert(string(serialized), qt.Equals, cacheEntry)
}

func Test_Ensure_Fetch_Fail_Logger(t *testing.T) {
	c := qt.New(t)
	cacheEntry := `1686756435844
test-etag
{"p":{"u":"https://cdn-global.configcat.com","r":0,"s":"FUkC6RADjzF0vXrDSfJn7BcEBag9afw1Y6jkqjMP9BA="},"f":{"testKey":{"t":1,"v":{"s":"testValue"}}}}`
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})
	logger := newTestLogger(t).(*testLogger)
	cfg := srv.config()
	cfg.PollInterval = 10 * time.Millisecond
	cfg.Logger = logger

	cache := &simpleCache{entry: []byte(cacheEntry)}
	cfg.Cache = cache

	client := NewCustomClient(cfg)
	defer client.Close()

	val := client.GetStringValue("testKey", "", nil)
	c.Assert(val, qt.Equals, "testValue")
	c.Assert(logger.Logs(), qt.Contains, "ERROR: [1101] config fetch failed: unexpected HTTP response was received while trying to fetch config JSON: 500 Internal Server Error")
}

func Test_Ensure_Empty_Response_Is_Not_Cached(t *testing.T) {
	c := qt.New(t)
	cacheEntry := `1686756435844
test-etag
{"p":{"u":"https://cdn-global.configcat.com","r":0,"s":"FUkC6RADjzF0vXrDSfJn7BcEBag9afw1Y6jkqjMP9BA="},"f":{"testKey":{"t":1,"v":{"s":"testValue"}}}}`
	srv := newConfigServer(t)
	cfg := srv.config()
	cfg.PollingMode = Manual

	cache := &simpleCache{entry: []byte(cacheEntry)}
	cfg.Cache = cache

	client := NewCustomClient(cfg)
	defer client.Close()

	tests := []struct {
		b string
	}{
		{b: "null"},
		{b: "{}"},
		{b: ""},
	}

	for _, test := range tests {
		t.Run(test.b, func(t *testing.T) {
			srv.setResponse(configResponse{
				status: http.StatusOK,
				body:   test.b,
			})

			_ = client.Refresh(t.Context())
			r, _ := cache.Get(t.Context(), "")
			c.Assert(string(r), qt.Equals, cacheEntry)
		})
	}
}

type simpleCache struct {
	mu    sync.Mutex
	entry []byte
}

func (s *simpleCache) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entry, nil
}

func (s *simpleCache) Set(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entry = value
	return nil
}

type customCache struct {
	mu       sync.Mutex
	getError error
	items    map[string]string
}

func (c *customCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.getError != nil {
		return nil, c.getError
	}
	return []byte(c.items[key]), nil
}

func (c *customCache) Set(ctx context.Context, key string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = string(value)
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
