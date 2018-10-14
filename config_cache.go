package configcat

import (
	"log"
	"os"
	"sync"
)

// A cache API used to make custom cache implementations.
type ConfigCache interface {
	// Get reads the configuration from the cache.
	Get() (string, error)
	// Set writes the configuration into the cache.
	Set(value string) error
}

type inMemoryConfigCache struct {
	value string
}

// A structure which is used to maintain the cached configuration.
type ConfigStore struct {
	cache 			ConfigCache
	logger 			*log.Logger
	inMemoryValue 	string
	sync.RWMutex
}

func newConfigStore(cache ConfigCache) *ConfigStore {
	return &ConfigStore{ cache: cache, logger: log.New(os.Stderr, "[ConfigCat - Config Cache]", log.LstdFlags) }
}

// NewInMemoryConfigCache creates an in-memory cache implementation used to store the fetched configurations.
func NewInMemoryConfigCache() *inMemoryConfigCache {
	return &inMemoryConfigCache{ value: "" }
}

// Get reads the configuration from the cache.
func (cache *inMemoryConfigCache) Get() (string, error) {
	return cache.value, nil
}

// Set writes the configuration into the cache.
func (cache *inMemoryConfigCache) Set(value string) error {
	cache.value = value
	return nil
}

// Get reads the configuration.
func (store *ConfigStore) Get() string {
	store.RLock()
	defer store.RUnlock()
	value, err := store.cache.Get()
	if err != nil {
		store.logger.Printf("Reading from the cache failed, %s", err)
		return store.inMemoryValue
	}

	return value
}

// Set writes the configuration.
func (store *ConfigStore) Set(value string) {
	store.Lock()
	defer store.Unlock()
	store.inMemoryValue = value
	err := store.cache.Set(value)
	if err != nil {
		store.logger.Printf("Saving into the cache failed, %s", err)
	}
}

