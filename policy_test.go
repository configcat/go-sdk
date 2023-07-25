package configcat

import (
	"context"
	"fmt"
	"github.com/configcat/go-sdk/v8/internal/cacheutils"
	"net/http"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// TestFetchFailWithCacheFallback tests that cache fallback behavior
// works as expected.
func TestFetchFailWithCacheFallback(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"test":1}`})

	// First use a client to populate the cache.
	cfg := srv.config()
	cfg.PollInterval = 10 * time.Millisecond

	cache := &customCache{
		items: map[string]string{},
	}
	cfg.Cache = cache

	client := NewCustomClient(cfg)
	defer client.Close()
	client.Refresh(context.Background())
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"test":1}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := cacheutils.CacheSegmentsFromBytes(cached)
		c.Assert(string(b), qt.Equals, `{"test":1}`)
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
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"test":1}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := cacheutils.CacheSegmentsFromBytes(cached)
		c.Assert(string(b), qt.Equals, `{"test":1}`)
	}

	time.Sleep(20 * time.Millisecond)
	// Check that the same client will fall back to the old
	// value even when the cache fails subsequently.
	cache.setGetError(fmt.Errorf("cache failure"))
	time.Sleep(60 * time.Millisecond)
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"test":1}`)

	// Check that if the cache value changes, it's still consulted.
	cache.setGetError(nil)
	for key := range cache.allItems() {
		cache.Set(context.Background(), key, cacheutils.CacheSegmentsToBytes(time.Now(), "etag", []byte(`{"test":2}`)))
	}
	time.Sleep(20 * time.Millisecond)
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"test":2}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := cacheutils.CacheSegmentsFromBytes(cached)
		c.Assert(string(b), qt.Equals, `{"test":2}`)
	}

	// Check that we still get the value from the server when it recovers.
	srv.setResponse(configResponse{body: `{"test":99}`})
	time.Sleep(20 * time.Millisecond)
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"test":99}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := cacheutils.CacheSegmentsFromBytes(cached)
		c.Assert(string(b), qt.Equals, `{"test":99}`)
	}
}

func Test_Consistent_Cache(t *testing.T) {
	c := qt.New(t)
	cacheEntry := `1690219337289
6458130e-993
{"f":{"flag":{"i":"","v":true,"t":0,"r":[],"p":[]}}}`
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

	details := client.GetBoolValueDetails("flag", false, nil)
	c.Assert(details.Data.FetchTime.UnixMilli(), qt.Equals, int64(1690219337289))
	c.Assert(details.Value, qt.IsTrue)
	cached, _ := cache.Get(context.Background(), "")
	ft, etag, _, _ := cacheutils.CacheSegmentsFromBytes(cached)
	c.Assert(ft.UnixMilli(), qt.Equals, int64(1690219337289))
	c.Assert(etag, qt.Equals, "6458130e-993")
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
