package configcat

import (
	"sync"
)

// ConfigCache is a cache API used to make custom cache implementations.
type ConfigCache interface {
	// get reads the configuration from the cache.
	Get() (string, error)
	// set writes the configuration into the cache.
	Set(value string) error
}

type inMemoryConfigCache struct {
	value string
}

// configStore is used to maintain the cached configuration.
type configStore struct {
	cache         ConfigCache
	logger        Logger
	inMemoryValue string
	sync.RWMutex
}

func newConfigStore(log Logger, cache ConfigCache) *configStore {
	return &configStore{cache: cache, logger: log}
}

// newInMemoryConfigCache creates an in-memory cache implementation used to store the fetched configurations.
func newInMemoryConfigCache() *inMemoryConfigCache {
	return &inMemoryConfigCache{value: ""}
}

// get reads the configuration from the cache.
func (cache *inMemoryConfigCache) Get() (string, error) {
	return cache.value, nil
}

// set writes the configuration into the cache.
func (cache *inMemoryConfigCache) Set(value string) error {
	cache.value = value
	return nil
}

// get reads the configuration.
func (store *configStore) get() string {
	store.RLock()
	defer store.RUnlock()
	value, err := store.cache.Get()
	if err != nil {
		store.logger.Errorf("Reading from the cache failed, %s", err)
		return store.inMemoryValue
	}

	return value
}

// set writes the configuration.
func (store *configStore) set(value string) {
	store.Lock()
	defer store.Unlock()
	store.inMemoryValue = value
	err := store.cache.Set(value)
	if err != nil {
		store.logger.Errorf("Saving into the cache failed, %s", err)
	}
}
