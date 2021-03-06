// ConfigCat SDK for Go (https://configcat.com)
package configcat

import (
	"context"
	"net/http"
	"time"
)

// Client is an object for handling configurations provided by ConfigCat.
type Client struct {
	refreshPolicy           refreshPolicy
	maxWaitTimeForSyncCalls time.Duration
	logger                  *leveledLogger
}

// ClientConfig describes custom configuration options for the Client.
type ClientConfig struct {
	// Logger is used to log information about configuration evaluation
	// and issues.
	Logger Logger
	// StaticLogLevel specifies whether the log level will remain the
	// same throughout the lifetime of the client.
	// If this is true and Logger implements the LoggerWithLevel
	// interface (notably, the default logger, *logrus.Logger, implements this interface),
	// the client can use a more efficient log implementation.
	StaticLogLevel bool
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
	// Default: Global. Set this parameter to be in sync with the Data Governance preference on the Dashboard:
	// https://app.configcat.com/organization/data-governance (Only Organization Admins have access)
	DataGovernance DataGovernance
}

type refreshPolicyConfig struct {
	fetcher *configFetcher
	logger  *leveledLogger
}

type RefreshMode interface {
	getModeIdentifier() string
	refreshPolicy(rconfig refreshPolicyConfig) refreshPolicy
}

type refreshPolicy interface {
	get(ctx context.Context) *config
	refresh(ctx context.Context)
	close()
}

func defaultConfig() ClientConfig {
	return ClientConfig{
		Logger:                  DefaultLogger(LogLevelWarn),
		BaseUrl:                 "",
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
	if len(sdkKey) == 0 {
		panic("sdkKey cannot be empty")
	}

	defaultConfig := defaultConfig()

	if config.Logger == nil {
		config.Logger = defaultConfig.Logger
	}

	var cache configCache
	if config.Cache != nil {
		cache = adaptCache(config.Cache)
	} else {
		cache = inMemoryConfigCache{}
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

	logger := &leveledLogger{
		level:  LogLevelDebug,
		Logger: config.Logger,
	}
	if levlog, ok := config.Logger.(LoggerWithLevel); ok && config.StaticLogLevel {
		logger.level = levlog.GetLevel()
	}

	return &Client{
		refreshPolicy: config.Mode.refreshPolicy(refreshPolicyConfig{
			fetcher: newConfigFetcher(sdkKey, cache, config, logger),
			logger:  logger,
		}),
		maxWaitTimeForSyncCalls: config.MaxWaitTimeForSyncCalls,
		logger:                  logger,
	}
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
	return client.getValue(client.getConfig(), key, defaultValue, user)
}

// GetValueAsyncForUser reads and sends a value asynchronously to a callback function as interface{} from the configuration identified by the given key.
// Optional user argument can be passed to identify the caller.
func (client *Client) GetValueAsyncForUser(key string, defaultValue interface{}, user *User, completion func(result interface{})) {
	if len(key) == 0 {
		panic("key cannot be empty")
	}
	go func() {
		result := client.getValue(client.getConfigAsync(), key, defaultValue, user)
		completion(result)
	}()
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
	return client.getVariationId(client.getConfig(), key, defaultVariationId, user)
}

// GetVariationIdAsyncForUser reads and sends a Variation Id asynchronously to a callback function as string from the configuration identified by the given key.
// Optional user argument can be passed to identify the caller.
func (client *Client) GetVariationIdAsyncForUser(key string, defaultVariationId string, user *User, completion func(result string)) {
	if len(key) == 0 {
		panic("key cannot be empty")
	}
	go func() {
		result := client.getVariationId(client.getConfigAsync(), key, defaultVariationId, user)
		completion(result)
	}()
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
	return client.getVariationIds(client.getConfig(), user)
}

// GetAllVariationIdsAsyncForUser reads and sends a Variation ID asynchronously to a callback function as []string from the configuration.
// Optional user argument can be passed to identify the caller.
func (client *Client) GetAllVariationIdsAsyncForUser(user *User, completion func(result []string, err error)) {
	go func() {
		result, err := client.getVariationIds(client.getConfigAsync(), user)
		completion(result, err)
	}()
}

// GetKeyAndValue returns the key of a setting and its value identified by the given Variation ID.
func (client *Client) GetKeyAndValue(variationId string) (string, interface{}) {
	return client.getKeyAndValueForVariation(client.getConfig(), variationId)
}

func (client *Client) getConfig() *config {
	if client.maxWaitTimeForSyncCalls == 0 {
		return client.refreshPolicy.get(context.Background())
	}
	ctx, cancel := context.WithTimeout(context.Background(), client.maxWaitTimeForSyncCalls)
	defer cancel()
	return client.refreshPolicy.get(ctx)
}

func (client *Client) getConfigAsync() *config {
	return client.refreshPolicy.get(context.Background())
}

// GetAllVariationIdsAsyncForUser reads and sends the key of a setting and its value identified by the given
// Variation ID asynchronously to a callback function as (string, interface{}) from the configuration.
func (client *Client) GetKeyAndValueAsync(variationId string, completion func(key string, value interface{})) {
	go func() {
		key, value := client.getKeyAndValueForVariation(client.getConfigAsync(), variationId)
		completion(key, value)
	}()
}

// GetAllKeys retrieves all the setting keys.
func (client *Client) GetAllKeys() ([]string, error) {
	return client.getConfig().getAllKeys(), nil
}

// GetAllKeysAsync retrieves all the setting keys asynchronously.
func (client *Client) GetAllKeysAsync(completion func(result []string, err error)) {
	go func() {
		result := client.getConfigAsync().getAllKeys()
		completion(result, nil)
	}()
}

// Refresh initiates a force refresh synchronously on the cached configuration.
func (client *Client) Refresh() {
	ctx := context.Background()
	if timeout := client.maxWaitTimeForSyncCalls; timeout > 0 {
		ctx1, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = ctx1
	}
	client.refreshPolicy.refresh(ctx)
}

// RefreshAsync initiates a force refresh asynchronously on the cached configuration.
func (client *Client) RefreshAsync(completion func()) {
	go func() {
		client.refreshPolicy.refresh(context.Background())
		completion()
	}()
}

// Close shuts down the client. After closing, it shouldn't be used.
func (client *Client) Close() {
	client.refreshPolicy.close()
}

func (client *Client) getValue(conf *config, key string, defaultValue interface{}, user *User) interface{} {
	val, _, err := conf.getValueAndVariationId(client.logger, key, user)
	if err != nil {
		client.logger.Errorf(
			"Evaluating GetValue(%s) failed. Returning defaultValue: [%v]. %v.",
			key,
			defaultValue,
			err,
		)
		val = defaultValue
	}
	return val
}

func (client *Client) getVariationId(conf *config, key string, defaultVariationId string, user *User) string {
	_, id, err := conf.getValueAndVariationId(client.logger, key, user)
	if err != nil {
		client.logger.Errorf(
			"Evaluating GetVariationId(%s) failed. Returning defaultVariationId: [%v]. %v",
			key,
			defaultVariationId,
			err,
		)
		id = defaultVariationId
	}
	return id
}

func (client *Client) getVariationIds(conf *config, user *User) ([]string, error) {
	keys := conf.getAllKeys()
	variationIds := make([]string, len(keys))
	for index, value := range keys {
		variationIds[index] = client.getVariationId(conf, value, "", user)
	}
	return variationIds, nil
}

func (client *Client) getKeyAndValueForVariation(conf *config, variationId string) (string, interface{}) {
	key, value := conf.getKeyAndValueForVariation(variationId)
	if key == "" {
		client.logger.Errorf("Evaluating GetKeyAndValue(%s) failed. Returning nil. Variation ID not found.", variationId)
		return "", nil
	}

	return key, value
}
