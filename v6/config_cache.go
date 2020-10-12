package configcat

// ConfigCache is a cache API used to make custom cache implementations.
type ConfigCache interface {
	// get reads the configuration from the cache.
	Get(key string) (string, error)
	// set writes the configuration into the cache.
	Set(key string, value string) error
}

type inMemoryConfigCache struct {
	store map[string]string
}

// newInMemoryConfigCache creates an in-memory cache implementation used to store the fetched configurations.
func newInMemoryConfigCache() *inMemoryConfigCache {
	return &inMemoryConfigCache{store: make(map[string]string)}
}

// get reads the configuration from the cache.
func (cache *inMemoryConfigCache) Get(key string) (string, error) {
	return cache.store[key], nil
}

// set writes the configuration into the cache.
func (cache *inMemoryConfigCache) Set(key string, value string) error {
	cache.store[key] = value
	return nil
}
