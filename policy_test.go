package configcat

import (
	"context"
	"fmt"
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
		_, _, b, _ := GetCacheSegments(cached)
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
		_, _, b, _ := GetCacheSegments(cached)
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
		cache.Set(context.Background(), key, CacheSegmentsToByte(time.Now(), "etag", []byte(`{"test":2}`)))
	}
	time.Sleep(20 * time.Millisecond)
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"test":2}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := GetCacheSegments(cached)
		c.Assert(string(b), qt.Equals, `{"test":2}`)
	}

	// Check that we still get the value from the server when it recovers.
	srv.setResponse(configResponse{body: `{"test":99}`})
	time.Sleep(20 * time.Millisecond)
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"test":99}`)
	for key := range cache.allItems() {
		cached, _ := cache.Get(context.Background(), key)
		_, _, b, _ := GetCacheSegments(cached)
		c.Assert(string(b), qt.Equals, `{"test":99}`)
	}
}

func Test_Consistent_Cache(t *testing.T) {
	c := qt.New(t)
	cacheEntry := `1690219337289
6458130e-993
{"p":{"u":"https://cdn-global.configcat.com","r":0},"f":{"isOneOf":{"v":"Default","t":1,"i":"c4ec4d53","p":[],"r":[{"o":0,"a":"Custom1","t":4,"c":"1.0.0, 2","v":"Is one of (1.0.0, 2)","i":"1e934047"},{"o":1,"a":"Custom1","t":4,"c":"1.0.0","v":"Is one of (1.0.0)","i":"44342254"},{"o":2,"a":"Custom1","t":4,"c":"   , 2.0.1, 2.0.2,    ","v":"Is one of (   , 2.0.1, 2.0.2,    )","i":"90e3ef46"},{"o":3,"a":"Custom1","t":4,"c":"3......","v":"Is one of (3......)","i":"59523971"},{"o":4,"a":"Custom1","t":4,"c":"3....","v":"Is one of (3...)","i":"2de217a1"},{"o":5,"a":"Custom1","t":4,"c":"3..0","v":"Is one of (3..0)","i":"bf943c79"},{"o":6,"a":"Custom1","t":4,"c":"3.0","v":"Is one of (3.0)","i":"3a6a8077"},{"o":7,"a":"Custom1","t":4,"c":"3.0.","v":"Is one of (3.0.)","i":"44f25fed"},{"o":8,"a":"Custom1","t":4,"c":"3.0.0","v":"Is one of (3.0.0)","i":"e77f5306"}]},"isOneOfWithPercentage":{"v":"Default","t":1,"i":"a94ff896","p":[{"o":0,"v":"20%","p":20,"i":"e25dba31"},{"o":1,"v":"80%","p":80,"i":"8c70c181"}],"r":[{"o":0,"a":"Custom1","t":4,"c":"1.0.0","v":"is one of (1.0.0)","i":"0ac4afc1"}]},"isNotOneOf":{"v":"Default","t":1,"i":"f79b763d","p":[],"r":[{"o":0,"a":"Custom1","t":5,"c":"1.0.0, 1.0.1, 2.0.0   , 2.0.1, 2.0.2,    ","v":"Is not one of (1.0.0, 1.0.1, 2.0.0   , 2.0.1, 2.0.2,    )","i":"a8d5f278"},{"o":1,"a":"Custom1","t":5,"c":"1.0.0, 3.0.1","v":"Is not one of (1.0.0, 3.0.1)","i":"54ac757f"}]},"isNotOneOfWithPercentage":{"v":"Default","t":1,"i":"b9614bad","p":[{"o":0,"v":"20%","p":20,"i":"68f652f0"},{"o":1,"v":"80%","p":80,"i":"b8d926e0"}],"r":[{"o":0,"a":"Custom1","t":5,"c":"1.0.0, 1.0.1, 2.0.0   , 2.0.1, 2.0.2,    ","v":"Is not one of (1.0.0, 1.0.1, 2.0.0   , 2.0.1, 2.0.2,    )","i":"9bf9e66f"},{"o":1,"a":"Custom1","t":5,"c":"1.0.0, 3.0.1","v":"Is not one of (1.0.0, 3.0.1)","i":"bfc1a544"}]},"lessThanWithPercentage":{"v":"Default","t":1,"i":"0081c525","p":[{"o":0,"v":"20%","p":20,"i":"3b1fde2a"},{"o":1,"v":"80%","p":80,"i":"42e92759"}],"r":[{"o":0,"a":"Custom1","t":6,"c":" 1.0.0 ","v":"< 1.0.0","i":"0c27d053"}]},"relations":{"v":"Default","t":1,"i":"c6155773","p":[],"r":[{"o":0,"a":"Custom1","t":6,"c":"1.0.0,","v":"<1.0.0,","i":"21b31b61"},{"o":1,"a":"Custom1","t":6,"c":"1.0.0","v":"< 1.0.0","i":"db3ddb7d"},{"o":2,"a":"Custom1","t":7,"c":"1.0.0","v":"<=1.0.0","i":"aa2c7493"},{"o":3,"a":"Custom1","t":8,"c":"2.0.0","v":">2.0.0","i":"5e47a1ea"},{"o":4,"a":"Custom1","t":9,"c":"2.0.0","v":">=2.0.0","i":"99482756"}]}}}`
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

	details := client.GetStringValueDetails("isOneOf", "", nil)
	c.Assert(details.Data.FetchTime.UnixMilli(), qt.Equals, int64(1690219337289))
	c.Assert(details.Value, qt.Equals, "Default")
	cached, _ := cache.Get(context.Background(), "")
	ft, etag, _, _ := GetCacheSegments(cached)
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
