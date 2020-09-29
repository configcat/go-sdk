// ConfigCat SDK for Go (https://configcat.com)
package configcat

import (
	"net/http"
	"time"
)

// Client is an object for handling configurations provided by ConfigCat.
type Client struct {
	parser                  *configParser
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
	// Set this parameter to restrict the location of your feature flag and setting data within the ConfigCat CDN.
	// This parameter must be in sync with the preferences on: https://app.configcat.com/organization/data-governance
	// (Only Organization Admins can set this preference.)
	DataGovernance DataGovernance
}

func defaultConfig() ClientConfig {
	return ClientConfig{
		Logger:                  DefaultLogger(LogLevelWarn),
		BaseUrl:                 "",
		Cache:                   newInMemoryConfigCache(),
		MaxWaitTimeForSyncCalls: 0,
		HttpTimeout:             time.Second * 15,
		Transport:               http.DefaultTransport,
		Mode:                    AutoPoll(time.Second * 120),
		DataGovernance:          Global,
	}
}

// NewClient initializes a new ConfigCat Client with the default configuration. The sdkKey parameter is mandatory.
func NewClient(sdkKey string) *Client {
	return NewCustomClient(sdkKey, ClientConfig{})
}

// NewCustomClient initializes a new ConfigCat Client with advanced configuration. The sdkKey parameter is mandatory.
func NewCustomClient(sdkKey string, config ClientConfig) *Client {
	return newInternal(sdkKey, config, nil)
}

func newInternal(sdkKey string, config ClientConfig, fetcher configProvider) *Client {
	if len(sdkKey) == 0 {
		panic("sdkKey cannot be empty")
	}

	defaultConfig := defaultConfig()

	if config.Logger == nil {
		config.Logger = defaultConfig.Logger
	}

	if config.Cache == nil {
		config.Cache = defaultConfig.Cache
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

	parser := newParser(config.Logger)

	if fetcher == nil {
		fetcher = newConfigFetcher(sdkKey, config, parser)
	}

	return &Client{
		parser:                  parser,
		refreshPolicy:           config.Mode.accept(newRefreshPolicyFactory(fetcher, config.Cache, config.Logger, sdkKey)),
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
			return client.parseJson(client.refreshPolicy.getLastCachedConfig(), key, defaultValue, user)
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

// GetVariationId returns a Variation ID synchronously as string from the configuration identified by the given key.
func (client *Client) GetVariationId(key string, defaultVariationId string) string {
	return client.GetVariationIdForUser(key, defaultVariationId, nil)
}

// GetVariationIdAsync reads and sends a Variation ID asynchronously to a callback function as string from the configuration identified by the given key.
func (client *Client) GetVariationIdAsync(key string, defaultVariationId string, completion func(result string)) {
	client.GetVariationIdAsyncForUser(key, defaultVariationId, nil, completion)
}

// GetVariationIdForUser returns a Variation ID synchronously as string from the configuration identified by the given key.
// Optional user argument can be passed to identify the caller.
func (client *Client) GetVariationIdForUser(key string, defaultVariationId string, user *User) string {
	if len(key) == 0 {
		panic("key cannot be empty")
	}

	if client.maxWaitTimeForSyncCalls > 0 {
		json, err := client.refreshPolicy.getConfigurationAsync().getOrTimeout(client.maxWaitTimeForSyncCalls)
		if err != nil {
			client.logger.Errorf("Policy could not provide the configuration: %s", err.Error())
			return client.parseVariationId(client.refreshPolicy.getLastCachedConfig(), key, defaultVariationId, user)
		}

		return client.parseVariationId(json.(string), key, defaultVariationId, user)
	}

	json, _ := client.refreshPolicy.getConfigurationAsync().get().(string)
	return client.parseVariationId(json, key, defaultVariationId, user)
}

// GetVariationIdAsyncForUser reads and sends a Variation Id asynchronously to a callback function as string from the configuration identified by the given key.
// Optional user argument can be passed to identify the caller.
func (client *Client) GetVariationIdAsyncForUser(key string, defaultVariationId string, user *User, completion func(result string)) {
	if len(key) == 0 {
		panic("key cannot be empty")
	}

	client.refreshPolicy.getConfigurationAsync().accept(func(res interface{}) {
		completion(client.parseVariationId(res.(string), key, defaultVariationId, user))
	})
}

// GetAllVariationIds returns the Variation IDs synchronously as []string from the configuration.
func (client *Client) GetAllVariationIds() ([]string, error) {
	return client.GetAllVariationIdsForUser(nil)
}

// GetAllVariationIdsAsync reads and sends a Variation ID asynchronously to a callback function as []string from the configuration.
func (client *Client) GetAllVariationIdsAsync(completion func(result []string, err error)) {
	client.GetAllVariationIdsAsyncForUser(nil, completion)
}

// GetAllVariationIdsForUser returns the Variation IDs synchronously as []string from the configuration.
// Optional user argument can be passed to identify the caller.
func (client *Client) GetAllVariationIdsForUser(user *User) ([]string, error) {
	if client.maxWaitTimeForSyncCalls > 0 {
		json, err := client.refreshPolicy.getConfigurationAsync().getOrTimeout(client.maxWaitTimeForSyncCalls)
		if err != nil {
			client.logger.Errorf("Policy could not provide the configuration: %s", err.Error())
			return nil, err
		}

		return client.getVariationIds(json.(string), user)
	}

	json, _ := client.refreshPolicy.getConfigurationAsync().get().(string)
	return client.getVariationIds(json, user)
}

// GetAllVariationIdsAsyncForUser reads and sends a Variation ID asynchronously to a callback function as []string from the configuration.
// Optional user argument can be passed to identify the caller.
func (client *Client) GetAllVariationIdsAsyncForUser(user *User, completion func(result []string, err error)) {
	client.refreshPolicy.getConfigurationAsync().accept(func(res interface{}) {
		completion(client.getVariationIds(res.(string), user))
	})
}

// GetKeyAndValue returns the key of a setting and its value identified by the given Variation ID.
func (client *Client) GetKeyAndValue(variationId string) (string, interface{}) {
	if client.maxWaitTimeForSyncCalls > 0 {
		json, err := client.refreshPolicy.getConfigurationAsync().getOrTimeout(client.maxWaitTimeForSyncCalls)
		if err != nil {
			client.logger.Errorf("Policy could not provide the configuration: %s", err.Error())
			return "", nil
		}

		return client.getKeyAndValue(json.(string), variationId)
	}

	json, _ := client.refreshPolicy.getConfigurationAsync().get().(string)
	return client.getKeyAndValue(json, variationId)
}

// GetAllVariationIdsAsyncForUser reads and sends the key of a setting and its value identified by the given
// Variation ID asynchronously to a callback function as (string, interface{}) from the configuration.
func (client *Client) GetKeyAndValueAsync(variationId string, completion func(key string, value interface{})) {
	client.refreshPolicy.getConfigurationAsync().accept(func(res interface{}) {
		completion(client.getKeyAndValue(res.(string), variationId))
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

		return client.parser.getAllKeys(json.(string))
	}

	json, _ := client.refreshPolicy.getConfigurationAsync().get().(string)
	return client.parser.getAllKeys(json)
}

// GetAllKeysAsync retrieves all the setting keys asynchronously.
func (client *Client) GetAllKeysAsync(completion func(result []string, err error)) {
	client.refreshPolicy.getConfigurationAsync().accept(func(res interface{}) {
		completion(client.parser.getAllKeys(res.(string)))
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

// RefreshAsync initiates a force refresh asynchronously on the cached configuration.
func (client *Client) RefreshAsync(completion func()) {
	client.refreshPolicy.refreshAsync().accept(completion)
}

// Close shuts down the client, after closing, it shouldn't be used
func (client *Client) Close() {
	client.refreshPolicy.close()
}

func (client *Client) parseJson(json string, key string, defaultValue interface{}, user *User) interface{} {
	parsed, err := client.parser.parse(json, key, user)
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

func (client *Client) parseVariationId(json string, key string, defaultVariationId string, user *User) string {
	parsed, err := client.parser.parseVariationId(json, key, user)
	if err != nil {
		client.logger.Errorf(
			"Evaluating GetVariationId(%s) failed. Returning defaultVariationId: [%v]. %s.",
			key,
			defaultVariationId,
			err.Error())
		return defaultVariationId
	}

	return parsed
}

func (client *Client) getVariationIds(json string, user *User) ([]string, error) {
	keys, err := client.parser.getAllKeys(json)
	if err != nil {
		client.logger.Errorf(
			"Evaluating GetAllVariationIds() failed. Returning nil. %s.",
			err.Error())
		return nil, err
	}
	variationIds := make([]string, len(keys))
	for index, value := range keys {
		variationIds[index] = client.parseVariationId(json, value, "", user)
	}

	return variationIds, nil
}

func (client *Client) getKeyAndValue(json string, variationId string) (string, interface{}) {
	key, value, err := client.parser.parseKeyValue(json, variationId)
	if err != nil {
		client.logger.Errorf(
			"Evaluating GetKeyAndValue(%s) failed. Returning nil. %s.",
			variationId,
			err.Error())
		return "", nil
	}

	return key, value
}
