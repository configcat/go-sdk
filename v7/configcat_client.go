// Package configcat contains the Go SDK of ConfigCat (https://configcat.com)
package configcat

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const DefaultPollInterval = 60 * time.Second

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

	// PollingMode specifies how the configuration is refreshed.
	// The zero value (default) is AutoPoll.
	PollingMode PollingMode

	// NoWaitForRefresh specifies that a Client get method (GetBoolValue,
	// GetIntValue, GetFloatValue, GetStringValue) should never wait for a
	// configuration refresh to complete before returning.
	//
	// By default, when this is false, if PollingMode is AutoPoll,
	// the first request may block, and if PollingMode is Lazy, any
	// request may block.
	NoWaitForRefresh bool

	// PollInterval specifies how old a configuration can
	// be before it's considered stale. If this is less
	// than 1, DefaultPollInterval is used.
	//
	// This parameter is ignored when PollingMode is Manual.
	PollInterval time.Duration

	// ChangeNotify is called, if not nil, when the settings configuration
	// has changed.
	ChangeNotify func()

	// DataGovernance specifies the data governance mode.
	// Set this parameter to be in sync with the Data Governance
	// preference on the Dashboard at
	// https://app.configcat.com/organization/data-governance
	// (only Organization Admins have access).
	// The default is Global.
	DataGovernance DataGovernance

	// DefaultUser holds the default user information to associate
	// with the Flagger, used whenever a nil User is passed.
	// This usually won't contain user-specific
	// information, but it may be useful when feature flags are dependent
	// on attributes of the current machine or similar. It's somewhat
	// more efficient to use DefaultUser=u than to call flagger.Snapshot(u)
	// on every feature flag evaluation.
	DefaultUser User

	// FlagOverrides holds the feature flag and setting overrides.
	FlagOverrides *FlagOverrides
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
	logger         *leveledLogger
	cfg            Config
	fetcher        *configFetcher
	needGetCheck   bool
	firstFetchWait sync.Once
	defaultUser    User
}

// PollingMode specifies a strategy for refreshing the configuration.
type PollingMode int

const (
	// AutoPoll causes the client to refresh the configuration
	// automatically at least as often as the Config.PollInterval
	// parameter.
	AutoPoll PollingMode = iota

	// Manual will only refresh the configuration when Refresh
	// is called explicitly, falling back to the cache for the initial
	// value or if the refresh fails.
	Manual

	// Lazy will refresh the configuration whenever a value
	// is retrieved and the configuration is older than
	// Config.PollInterval.
	Lazy
)

// NewClient returns a new Client value that access the default
// configcat servers using the given SDK key.
//
// The GetBoolValue, GetIntValue, GetFloatValue and GetStringValue methods can be used to find out current
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
	if cfg.PollInterval < 1 {
		cfg.PollInterval = DefaultPollInterval
	}
	logger := newLeveledLogger(cfg.Logger)
	if cfg.FlagOverrides != nil {
		cfg.FlagOverrides.loadEntries(logger)
	}
	fetcher := newConfigFetcher(cfg, logger, cfg.DefaultUser)
	return &Client{
		cfg:          cfg,
		logger:       logger,
		fetcher:      fetcher,
		needGetCheck: cfg.PollingMode == Lazy || cfg.PollingMode == AutoPoll && !cfg.NoWaitForRefresh,
		defaultUser:  cfg.DefaultUser,
	}
}

// Refresh refreshes the cached configuration. If the context is
// canceled while the refresh is in progress, Refresh will return but
// the underlying HTTP request will not be canceled.
func (client *Client) Refresh(ctx context.Context) error {
	// Note: add a tiny bit to the current time so that we refresh
	// even if the current time hasn't changed since the last
	// time we refreshed.
	if client.fetcher != nil {
		return client.fetcher.refreshIfOlder(ctx, time.Now().Add(1), true)
	}
	return nil
}

// RefreshIfOlder is like Refresh but refreshes the configuration only
// if the most recently fetched configuration is older than the given
// age.
func (client *Client) RefreshIfOlder(ctx context.Context, age time.Duration) error {
	if client.fetcher != nil {
		return client.fetcher.refreshIfOlder(ctx, time.Now().Add(-age), true)
	}
	return nil
}

// Close shuts down the client. After closing, it shouldn't be used.
func (client *Client) Close() {
	if client.fetcher != nil {
		client.fetcher.close()
	}
}

// GetBoolValue returns the value of a boolean-typed feature flag, or defaultValue if no
// value can be found. If user is non-nil, it will be used to
// choose the value (see the User documentation for details).
// If user is nil and Config.DefaultUser was non-nil, that will be used instead.
//
// In Lazy refresh mode, this can block indefinitely while the configuration
// is fetched. Use RefreshIfOlder explicitly if explicit control of timeouts
// is needed.
func (client *Client) GetBoolValue(key string, defaultValue bool, user User) bool {
	return Bool(key, defaultValue).Get(client.Snapshot(user))
}

// GetIntValue is like GetBoolValue except for int-typed (whole number) feature flags.
func (client *Client) GetIntValue(key string, defaultValue int, user User) int {
	return Int(key, defaultValue).Get(client.Snapshot(user))
}

// GetFloatValue is like GetBoolValue except for float-typed (decimal number) feature flags.
func (client *Client) GetFloatValue(key string, defaultValue float64, user User) float64 {
	return Float(key, defaultValue).Get(client.Snapshot(user))
}

// GetStringValue is like GetBoolValue except for string-typed (text) feature flags.
func (client *Client) GetStringValue(key string, defaultValue string, user User) string {
	return String(key, defaultValue).Get(client.Snapshot(user))
}

// GetVariationID returns the variation ID (analytics) that will be used for the given key
// with respect to the given user, or the default value if none is found.
func (client *Client) GetVariationID(key string, defaultVariationId string, user User) string {
	if result := client.Snapshot(user).GetVariationID(key); result != "" {
		return result
	}
	return defaultVariationId
}

// GetVariationIDs returns all variation IDs (analytics) in the current configuration
// that apply to the given user, or Config.DefaultUser if user is nil.
func (client *Client) GetVariationIDs(user User) []string {
	return client.Snapshot(user).GetVariationIDs()
}

// GetKeyValueForVariationID returns the key and value that
// are associated with the given variation ID. If the
// variation ID isn't found, it returns "", nil.
func (client *Client) GetKeyValueForVariationID(id string) (string, interface{}) {
	return client.Snapshot(nil).GetKeyValueForVariationID(id)
}

// GetAllKeys returns all the known keys.
func (client *Client) GetAllKeys() []string {
	return client.Snapshot(nil).GetAllKeys()
}

// GetAllValues returns all keys and values in a key-value map.
func (client *Client) GetAllValues(user User) map[string]interface{} {
	return client.Snapshot(user).GetAllValues()
}

// Snapshot returns an immutable snapshot of the most recent feature
// flags retrieved by the client, associated with the given user, or
// Config.DefaultUser if user is nil.
func (client *Client) Snapshot(user User) *Snapshot {
	if client.needGetCheck {
		switch client.cfg.PollingMode {
		case Lazy:
			if err := client.fetcher.refreshIfOlder(client.fetcher.ctx, time.Now().Add(-client.cfg.PollInterval), !client.cfg.NoWaitForRefresh); err != nil {
				client.logger.Errorf("lazy refresh failed: %v", err)
			}
		case AutoPoll:
			client.firstFetchWait.Do(func() {
				// Note: we don't have to select on client.fetcher.ctx.Done here
				// because if that's closed, the first fetch will be unblocked and
				// will close this channel.
				<-client.fetcher.doneInitialGet
			})
		}
	}
	return newSnapshot(client.fetcher.current(), user, client.logger)
}
