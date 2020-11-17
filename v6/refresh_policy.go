package configcat

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sync"
)

const (
	CacheBase = "go_" + ConfigJsonName + "_%s"
)

type refreshPolicy interface {
	getConfigurationAsync() *asyncResult
	getLastCachedConfig() *config
	refreshAsync() *async
	close()
}

type configRefresher struct {
	configFetcher configProvider
	cache         configCache
	logger        Logger
	cacheKey      string
	mu            sync.RWMutex
	inMemoryValue *config
}

func newConfigRefresher(conf refreshPolicyConfig) *configRefresher {
	sha := sha1.New()
	sha.Write([]byte(conf.sdkKey))
	hash := hex.EncodeToString(sha.Sum(nil))
	return &configRefresher{
		configFetcher: conf.configFetcher,
		cache:         conf.cache,
		logger:        conf.logger,
		cacheKey:      fmt.Sprintf(CacheBase, hash),
	}
}

func (refresher *configRefresher) refreshAsync() *async {
	return refresher.configFetcher.getConfigurationAsync().accept(func(result interface{}) {
		if response := result.(fetchResponse); response.isFetched() {
			refresher.set(response.config)
		}
	})
}

func (refresher *configRefresher) getLastCachedConfig() *config {
	refresher.mu.RLock()
	defer refresher.mu.RUnlock()

	return refresher.inMemoryValue
}

// get reads the configuration.
func (refresher *configRefresher) get() *config {
	refresher.mu.RLock()
	defer refresher.mu.RUnlock()
	value, err := refresher.cache.get(refresher.cacheKey)
	if err != nil {
		refresher.logger.Errorf("Reading from the cache failed, %s", err)
		return refresher.inMemoryValue
	}

	return value
}

// set writes the configuration.
func (refresher *configRefresher) set(value *config) {
	refresher.mu.Lock()
	defer refresher.mu.Unlock()
	refresher.inMemoryValue = value
	err := refresher.cache.set(refresher.cacheKey, value)
	if err != nil {
		refresher.logger.Errorf("Saving into the cache failed, %s", err)
	}
}
