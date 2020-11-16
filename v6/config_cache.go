package configcat

import "fmt"

// ConfigCache is a cache API used to make custom cache implementations.
type ConfigCache interface {
	// get reads the configuration from the cache.
	Get(key string) (string, error)
	// set writes the configuration into the cache.
	Set(key string, value string) error
}

type configCache interface {
	// get reads the configuration from the cache.
	get(key string) (*config, error)
	// set writes the configuration into the cache.
	set(key string, value *config) error
}

type inMemoryConfigCache map[string]*config

// get reads the configuration from the cache.
func (cache inMemoryConfigCache) get(key string) (*config, error) {
	return cache[key], nil
}

// set writes the configuration into the cache.
func (cache inMemoryConfigCache) set(key string, value *config) error {
	cache[key] = value
	return nil
}

func adaptCache(c ConfigCache) configCache {
	return &cacheAdaptor{c}
}

type cacheAdaptor struct {
	c ConfigCache
}

func (c *cacheAdaptor) get(key string) (*config, error) {
	val, err := c.c.Get(key)
	if err != nil {
		return nil, err
	}
	conf, err := parseConfig([]byte(val))
	if err != nil {
		return nil, fmt.Errorf("cannot parse config from cache: %v", err)
	}
	return conf, nil
}

func (c *cacheAdaptor) set(key string, value *config) error {
	return c.c.Set(key, value.body())
}
