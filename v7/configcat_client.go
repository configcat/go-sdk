// ConfigCat SDK for Go (https://configcat.com)
package configcat

import (
	"context"
	"net/http"
	"time"
)

const DefaultPollInterval = 120 * time.Second

// Config describes configuration options for the Client.
type Config struct {
	// SDKKey holds the key for the SDK. This parameter
	// is mandatory.
	SDKKey string

	// Logger is used to log information about configuration evaluation
	// and issues. If it's nil, DefaultLogger(LogLevelWarn) will be used.
	// It assumes that the logging level will not be increased
	// during the lifetime of the client.
	Logger Logger

	// Cache is used to cache configuration values.
	// If it's nil, no caching will be done.
	Cache ConfigCache

	// BaseURL holds the URL of the ConfigCat CDN server.
	// If this is empty, an appropriate URL will be chosen
	// based on the DataGovernance parameter.
	BaseURL string

	// Transport is used as the HTTP transport for
	// requests to the CDN. If it's nil, http.DefaultTransport
	// will be used.
	Transport http.RoundTripper

	// HTTPTimeout holds the timeout for HTTP requests
	// made by the client. If it's zero, DefaultHTTPTimeout
	// will be used. If it's negative, no timeout will be
	// used.
	HTTPTimeout time.Duration

	// PollInterval holds the polling interval for checking
	// whether the configuration has changed. If it's zero,
	// DefaultPollInterval will be used.
	PollInterval time.Duration

	// DisablePolling specifies whether polling is disabled.
	// When disabled, the Refresh method should be used
	// to explicitly request the configuration; otherwise
	// the configuration will be taken from the cache.
	DisablePolling bool

	// ChangeNotify is called, if not nill, when the settings configuration
	// has changed.
	ChangeNotify func()

	// DataGovernance specifies the data governance mode.
	// Set this parameter to be in sync with the Data Governance
	// preference on the Dashboard at
	// https://app.configcat.com/organization/data-governance
	// (only Organization Admins have access).
	// The default is Global.
	DataGovernance DataGovernance
}

// ConfigCache is a cache API used to make custom cache implementations.
type ConfigCache interface {
	// Get reads the configuration from the cache.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set writes the configuration into the cache.
	Set(ctx context.Context, key string, value []byte) error
}

// DataGovernance describes the location of your feature flag and setting data within the ConfigCat CDN.
type DataGovernance int

const (
	// Global Select this if your feature flags are published to all global CDN nodes.
	Global DataGovernance = 0
	// EUOnly Select this if your feature flags are published to CDN nodes only in the EU.
	EUOnly DataGovernance = 1
)

// Client is an object for handling configurations provided by ConfigCat.
type Client struct {
	logger  *leveledLogger
	cfg     Config
	fetcher *configFetcher
}

// NewClient returns a new Client value that access the default
// configcat servers using the given SDK key.
//
// The Bool, Int, Float and String methods can be used to find out current
// feature flag values. These methods will always return immediately without
// waiting - if there is no configuration available, they'll return a default
// value.
func NewClient(sdkKey string) *Client {
	return NewCustomClient(Config{
		SDKKey: sdkKey,
	})
}

// NewCustomClient initializes a new ConfigCat Client with advanced configuration.
func NewCustomClient(cfg Config) *Client {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.Logger == nil {
		cfg.Logger = DefaultLogger(LogLevelWarn)
	}
	logger := &leveledLogger{
		level:  cfg.Logger.GetLevel(),
		Logger: cfg.Logger,
	}
	return &Client{
		cfg:     cfg,
		logger:  logger,
		fetcher: newConfigFetcher(cfg, logger),
	}
}

// Refresh refreshes the cached configuration. If the context is
// canceled while the refresh is in progress, Refresh will return but
// the underlying HTTP request will not be canceled.
func (c *Client) Refresh(ctx context.Context) error {
	return c.fetcher.refreshIfOlder(ctx, time.Now())
}

// RefreshIfOlder is like Refresh but refreshes the configuration only
// if the most recently fetched configuration is older than the given
// age.
func (c *Client) RefreshIfOlder(ctx context.Context, age time.Duration) error {
	return c.fetcher.refreshIfOlder(ctx, time.Now().Add(-age))
}

// Close shuts down the client. After closing, it shouldn't be used.
func (client *Client) Close() {
	client.fetcher.close()
}

func (client *Client) Bool(key string, defaultValue bool, user *User) bool {
	if v, ok := client.getValue(key, user).(bool); ok {
		return v
	}
	return defaultValue
}

func (client *Client) Int(key string, defaultValue int, user *User) int {
	if v, ok := client.getValue(key, user).(float64); ok {
		// TODO log error?
		return int(v)
	}
	return defaultValue
}

func (client *Client) Float(key string, defaultValue float64, user *User) float64 {
	if v, ok := client.getValue(key, user).(float64); ok {
		// TODO log error?
		return v
	}
	return defaultValue
}

func (client *Client) String(key string, defaultValue string, user *User) string {
	if v, ok := client.getValue(key, user).(string); ok {
		// TODO log error?
		return v
	}
	return defaultValue
}

// GetValue returns a value synchronously as interface{} from the configuration identified by the given key.
func (client *Client) getValue(key string, user *User) interface{} {
	value, _, _ := client.fetcher.current().getValueAndVariationID(client.logger, key, user)
	// TODO log error?
	return value
}

// VariationID returns the variation ID that will be used for the given key
// with the given optional user. If none is found, the empty string is returned.
func (client *Client) VariationID(key string, user *User) string {
	_, variationID, _ := client.fetcher.current().getValueAndVariationID(client.logger, key, user)
	return variationID
}

// VariationIDs returns all  variation IDs in the current configuration
// that apply to the given optional user.
func (client *Client) VariationIDs(user *User) []string {
	conf := client.fetcher.current()
	keys := conf.keys()
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		_, id, err := conf.getValueAndVariationID(client.logger, key, user)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// KeyValueForVariationID returns the key and value that
// are associated with the given variation ID. If the
// variation ID isn't found, it returns "", nil.
func (client *Client) KeyValueForVariationID(id string) (string, interface{}) {
	key, value := client.fetcher.current().getKeyAndValueForVariation(id)
	if key == "" {
		client.logger.Errorf("Evaluating GetKeyAndValue(%s) failed. Returning nil. Variation ID not found.")
		return "", nil
	}
	return key, value
}

// Keys returns all the known keys.
func (client *Client) Keys() []string {
	return client.fetcher.current().keys()
}
