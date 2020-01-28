// Package configcat contains the Golang SDK for ConfigCat (https://configcat.com)
package configcat

import (
	"net/http"
	"time"
)

// Client is an object for handling configurations provided by ConfigCat.
type Client struct {
	store                   *configStore
	parser                  *ConfigParser
	refreshPolicy           refreshPolicy
	maxWaitTimeForSyncCalls time.Duration
	logger                  Logger
}

// ClientConfig describes custom configuration options for the Client.
type ClientConfig struct {
	// Base logger used to create new loggers
	Logger Logger
	// The custom cache implementation used to store the configuration.
	Cache ConfigCache
	// The maximum time how long at most the synchronous calls (e.g. client.get(...)) should block the caller.
	// If it's 0 then the caller will be blocked in case of sync calls, until the operation succeeds or fails.
	MaxWaitTimeForSyncCalls time.Duration
	// The maximum wait time for a http response.
	HttpTimeout time.Duration
	// The base ConfigCat CDN url.
	BaseUrl string
	// The custom http transport object.
	Transport http.RoundTripper
	// The refresh mode of the cached configuration.
	Mode RefreshMode
}

func defaultConfig() ClientConfig {
	return ClientConfig{
		Logger:                  DefaultLogger(),
		BaseUrl:                 "https://cdn.configcat.com",
		Cache:                   newInMemoryConfigCache(),
		MaxWaitTimeForSyncCalls: 0,
		HttpTimeout:             time.Second * 15,
		Transport:               http.DefaultTransport,
		Mode:					 AutoPoll(time.Second * 120),
	}
}

// NewClient initializes a new ConfigCat Client with the default configuration. The api key parameter is mandatory.
func NewClient(apiKey string) *Client {
	return NewCustomClient(apiKey, ClientConfig{})
}

// NewCustomClient initializes a new ConfigCat Client with advanced configuration. The api key parameter is mandatory.
func NewCustomClient(apiKey string, config ClientConfig) *Client {
	return newInternal(apiKey, config, nil)
}

func newInternal(apiKey string, config ClientConfig, fetcher configProvider) *Client {
	if len(apiKey) == 0 {
		panic("apiKey cannot be empty")
	}

	defaultConfig := defaultConfig()

	if config.Logger == nil {
		config.Logger = defaultConfig.Logger
	}

	if config.Cache == nil {
		config.Cache = defaultConfig.Cache
	}

	if len(config.BaseUrl) == 0 {
		config.BaseUrl = defaultConfig.BaseUrl
	}

	if config.MaxWaitTimeForSyncCalls < 0 {
		config.MaxWaitTimeForSyncCalls = defaultConfig.MaxWaitTimeForSyncCalls
	}

	if config.HttpTimeout <= 0 {
		config.HttpTimeout = defaultConfig.HttpTimeout
	}

	if config.Transport == nil {
		config.Transport = defaultConfig.Transport
	}

	if config.Mode == nil {
		config.Mode = defaultConfig.Mode
	}

	if fetcher == nil {
		fetcher = newConfigFetcher(apiKey, config)
	}

	store := newConfigStore(config.Logger, config.Cache)

	return &Client{store:        store,
		parser:                  newParser(config.Logger),
		refreshPolicy:           createRefreshPolicyByMode(config, fetcher, store),
		maxWaitTimeForSyncCalls: config.MaxWaitTimeForSyncCalls,
		logger:                  config.Logger}
}

// GetValue returns a value synchronously as interface{} from the configuration identified by the given key.
func (client *Client) GetValue(key string, defaultValue interface{}) interface{} {
	return client.GetValueForUser(key, defaultValue, nil)
}

// GetValueAsync reads and sends a value asynchronously to a callback function as interface{} from the configuration identified by the given key.
func (client *Client) GetValueAsync(key string, defaultValue interface{}, completion func(result interface{})) {
	client.GetValueAsyncForUser(key, defaultValue, nil, completion)
}

// GetValueForUser returns a value synchronously as interface{} from the configuration identified by the given key.
// Optional user argument can be passed to identify the caller.
func (client *Client) GetValueForUser(key string, defaultValue interface{}, user *User) interface{} {
	if len(key) == 0 {
		panic("key cannot be empty")
	}

	if client.maxWaitTimeForSyncCalls > 0 {
		json, err := client.refreshPolicy.getConfigurationAsync().getOrTimeout(client.maxWaitTimeForSyncCalls)
		if err != nil {
			client.logger.Errorf("Policy could not provide the configuration: %s", err.Error())
			return client.parseJson(client.store.get(), key, defaultValue, user)
		}

		return client.parseJson(json.(string), key, defaultValue, user)
	}

	json, _ := client.refreshPolicy.getConfigurationAsync().get().(string)
	return client.parseJson(json, key, defaultValue, user)
}

// GetValueAsyncForUser reads and sends a value asynchronously to a callback function as interface{} from the configuration identified by the given key.
// Optional user argument can be passed to identify the caller.
func (client *Client) GetValueAsyncForUser(key string, defaultValue interface{}, user *User, completion func(result interface{})) {
	if len(key) == 0 {
		panic("key cannot be empty")
	}

	client.refreshPolicy.getConfigurationAsync().accept(func(res interface{}) {
		completion(client.parseJson(res.(string), key, defaultValue, user))
	})
}

// GetAllKeys retrieves all the setting keys.
func (client *Client) GetAllKeys() ([]string, error) {
	if client.maxWaitTimeForSyncCalls > 0 {
		json, err := client.refreshPolicy.getConfigurationAsync().getOrTimeout(client.maxWaitTimeForSyncCalls)
		if err != nil {
			client.logger.Errorf("Policy could not provide the configuration: %s", err.Error())
			return nil, err
		}

		return client.parser.GetAllKeys(json.(string))
	}

	json, _ := client.refreshPolicy.getConfigurationAsync().get().(string)
	return client.parser.GetAllKeys(json)
}

// GetAllKeysAsync retrieves all the setting keys asynchronously.
func (client *Client) GetAllKeysAsync(completion func(result []string, err error)) {
	client.refreshPolicy.getConfigurationAsync().accept(func(res interface{}) {
		completion(client.parser.GetAllKeys(res.(string)))
	})
}

// Refresh initiates a force refresh synchronously on the cached configuration.
func (client *Client) Refresh() {
	if client.maxWaitTimeForSyncCalls > 0 {
		client.refreshPolicy.refreshAsync().waitOrTimeout(client.maxWaitTimeForSyncCalls)
	} else {
		client.refreshPolicy.refreshAsync().wait()
	}
}

// refreshAsync initiates a force refresh asynchronously on the cached configuration.
func (client *Client) RefreshAsync(completion func()) {
	client.refreshPolicy.refreshAsync().accept(completion)
}

// close shuts down the client, after closing, it shouldn't be used
func (client *Client) Close() {
	client.refreshPolicy.close()
}

func (client *Client) parseJson(json string, key string, defaultValue interface{}, user *User) interface{} {
	parsed, err := client.parser.ParseWithUser(json, key, user)
	if err != nil {
		client.logger.Errorf(
			"Evaluating GetValue(%s) failed. Returning defaultValue: [%v]. %s.",
			key,
			defaultValue,
			err.Error())
		return defaultValue
	}

	return parsed
}

func createRefreshPolicyByMode(config ClientConfig, fetcher configProvider, store *configStore) refreshPolicy {
	autoPoll, ok := config.Mode.(autoPollConfig)
	if ok {
		return newAutoPollingPolicy(fetcher, store, config.Logger, autoPoll)
	}

	lazyLoad, ok := config.Mode.(lazyLoadConfig)
	if ok {
		return newLazyLoadingPolicy(fetcher, store, config.Logger, lazyLoad)
	}

	_, ok = config.Mode.(manualPollConfig)
	if ok {
		return newManualPollingPolicy(fetcher, store, config.Logger)
	}

	panic("Invalid refresh mode, please choose from AutoPoll(), LazyLoad() or ManualPoll().")
}
