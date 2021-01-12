// ConfigCat SDK for Go (https://configcat.com)
package configcat

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const DefaultMaxAge = 120 * time.Second

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

	// RefreshMode specifies how the configuration is refreshed.
	// The zero value (default) is AutoPoll.
	RefreshMode RefreshMode

	// NoWaitForRefresh specifies that a Client get method (Bool,
	// Int, Float, String) should never wait for a configuration refresh
	// to complete before returning.
	//
	// By default, when this is false, if RefreshMode is AutoPoll,
	// the first request may block, and if RefreshMode is Lazy, any
	// request may block.
	NoWaitForRefresh bool

	// MaxAge specifies how old a configuration can
	// be before it's considered stale. If this is
	// zero, DefaultMaxAge is used.
	//
	// This parameter is ignored when RefreshMode is Manual.
	MaxAge time.Duration

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
	logger         *leveledLogger
	cfg            Config
	fetcher        *configFetcher
	needGetCheck   bool
	firstFetchWait sync.Once
}

// RefreshMode specifies a strategy for refreshing the configuration.
type RefreshMode int

const (
	// AutoPoll causes the client to refresh the configuration
	// automatically at least as often as the Config.MaxAge
	// parameter.
	AutoPoll RefreshMode = iota

	// Manual will only refresh the configuration when Refresh
	// is called explicitly, falling back to the cache for the initial
	// value or if the refresh fails.
	Manual

	// Lazy will refresh the configuration whenever a value
	// is retrieved and the configuration is older than
	// Config.MaxAge.
	Lazy
)

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
	if cfg.MaxAge == 0 {
		cfg.MaxAge = DefaultMaxAge
	}
	if cfg.Logger == nil {
		cfg.Logger = DefaultLogger(LogLevelWarn)
	}
	logger := &leveledLogger{
		level:  cfg.Logger.GetLevel(),
		Logger: cfg.Logger,
	}
	return &Client{
		cfg:          cfg,
		logger:       logger,
		fetcher:      newConfigFetcher(cfg, logger),
		needGetCheck: cfg.RefreshMode == Lazy || cfg.RefreshMode == AutoPoll && !cfg.NoWaitForRefresh,
	}
}

// Refresh refreshes the cached configuration. If the context is
// canceled while the refresh is in progress, Refresh will return but
// the underlying HTTP request will not be canceled.
func (client *Client) Refresh(ctx context.Context) error {
	return client.fetcher.refreshIfOlder(ctx, time.Now(), true)
}

// RefreshIfOlder is like Refresh but refreshes the configuration only
// if the most recently fetched configuration is older than the given
// age.
func (client *Client) RefreshIfOlder(ctx context.Context, age time.Duration) error {
	return client.fetcher.refreshIfOlder(ctx, time.Now().Add(-age), true)
}

// Close shuts down the client. After closing, it shouldn't be used.
func (client *Client) Close() {
	client.fetcher.close()
}

// Bool returns the value of a boolean-typed feature flag, or defaultValue if no
// value can be found. If user is non-nil, it will be used to
// choose the value (see the User documentation for details).
//
// In Lazy refresh mode, this can block indefinitely while the configuration
// is fetched. Use RefreshIfOlder explicitly if explicit control of timeouts
// is needed.
func (client *Client) Bool(key string, defaultValue bool, user User) bool {
	return Bool(key, defaultValue).Get(client.Snapshot(user))
}

// Int is like Bool except for int-typed (whole number) feature flags.
func (client *Client) Int(key string, defaultValue int, user User) int {
	return Int(key, defaultValue).Get(client.Snapshot(user))
}

// Int is like Bool except for float-typed (decimal number) feature flags.
func (client *Client) Float(key string, defaultValue float64, user User) float64 {
	return Float(key, defaultValue).Get(client.Snapshot(user))
}

// Int is like Bool except for string-typed (text) feature flags.
func (client *Client) String(key string, defaultValue string, user User) string {
	return String(key, defaultValue).Get(client.Snapshot(user))
}

// Get returns a feature flag value regardless of type. If there is no
// value found, it returns nil; otherwise the returned value
// has one of the dynamic types bool, int, float64, or string.
func (client *Client) Get(key string, user User) interface{} {
	return client.Snapshot(user).Get(key)
}

func (client *Client) Snapshot(user User) *Snapshot {
	if client.needGetCheck {
		switch client.cfg.RefreshMode {
		case Lazy:
			if err := client.fetcher.refreshIfOlder(client.fetcher.ctx, time.Now().Add(-client.cfg.MaxAge), !client.cfg.NoWaitForRefresh); err != nil {
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
