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
	inMemoryValue *config
	cacheKey      string
	sync.RWMutex
}

func newConfigRefresher(configFetcher configProvider, cache configCache, logger Logger, sdkKey string) configRefresher {
	sha := sha1.New()
	sha.Write([]byte(sdkKey))
	hash := hex.EncodeToString(sha.Sum(nil))
	cacheKey := fmt.Sprintf(CacheBase, hash)
	return configRefresher{configFetcher: configFetcher, cache: cache, logger: logger, cacheKey: cacheKey}
}

func (refresher *configRefresher) refreshAsync() *async {
	return refresher.configFetcher.getConfigurationAsync().accept(func(result interface{}) {
		if response := result.(fetchResponse); response.isFetched() {
			refresher.set(response.config)
		}
	})
}

func (refresher *configRefresher) getLastCachedConfig() *config {
	return refresher.inMemoryValue
}

// get reads the configuration.
func (refresher *configRefresher) get() *config {
	refresher.RLock()
	defer refresher.RUnlock()
	value, err := refresher.cache.get(refresher.cacheKey)
	if err != nil {
		refresher.logger.Errorf("Reading from the cache failed, %s", err)
		return refresher.inMemoryValue
	}

	return value
}

// set writes the configuration.
func (refresher *configRefresher) set(value *config) {
	refresher.Lock()
	defer refresher.Unlock()
	refresher.inMemoryValue = value
	err := refresher.cache.set(refresher.cacheKey, value)
	if err != nil {
		refresher.logger.Errorf("Saving into the cache failed, %s", err)
	}
}
