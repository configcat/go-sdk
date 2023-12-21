package configcat

import (
	"context"
	"errors"
	"fmt"
	"github.com/configcat/go-sdk/v9/configcatcache"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	globalBaseURL = "https://cdn-global.configcat.com"
	euOnlyBaseURL = "https://cdn-eu.configcat.com"
)

type fetcherError struct {
	Err     error
	EventId int
}

func (f *fetcherError) Error() string {
	return f.Err.Error()
}

type fetcher interface {
	refreshIfOlder(ctx context.Context, before time.Time, wait bool) error
	close()
	current() *config
	isOffline() bool
	setMode(offline bool)
	context() context.Context
	doneInitGet() chan struct{}
}

type configFetcher struct {
	sdkKey            string
	cacheKey          string
	cache             ConfigCache
	logger            *leveledLogger
	client            *http.Client
	urlIsCustom       bool
	changeNotify      func()
	defaultUser       User
	pollingIdentifier string
	overrides         *FlagOverrides
	hooks             *Hooks
	offline           uint32
	timeout           time.Duration

	ctx       context.Context
	ctxCancel func()

	// baseURL is maintained by the fetcher goroutine.
	baseURL string

	// wg counts the number of outstanding goroutines
	// so that we can wait for them to finish when closed.
	wg sync.WaitGroup

	// doneInitialGet is closed when the very first
	// config get has completed, regardless of whether
	// it succeeded.
	doneInitialGet chan struct{}
	doneGetOnce    sync.Once

	mu        sync.Mutex
	config    atomic.Value // holds *config or nil.
	fetchDone chan error
}

// newConfigFetcher returns a
func newConfigFetcher(cfg Config, logger *leveledLogger, defaultUser User) fetcher {
	f := &configFetcher{
		sdkKey:    cfg.SDKKey,
		cache:     cfg.Cache,
		cacheKey:  configcatcache.ProduceCacheKey(cfg.SDKKey, configcatcache.ConfigJSONName, configcatcache.ConfigJSONCacheVersion),
		overrides: cfg.FlagOverrides,
		hooks:     cfg.Hooks,
		logger:    logger,
		timeout:   cfg.HTTPTimeout,
		client: &http.Client{
			Timeout:   cfg.HTTPTimeout,
			Transport: cfg.Transport,
		},
		doneInitialGet:    make(chan struct{}),
		defaultUser:       defaultUser,
		pollingIdentifier: pollingModeToIdentifier(cfg.PollingMode),
	}
	f.ctx, f.ctxCancel = context.WithCancel(context.Background())
	if cfg.Offline {
		f.offline = 1
	}
	if cfg.BaseURL == "" {
		if cfg.DataGovernance == Global {
			f.baseURL = globalBaseURL
		} else {
			f.baseURL = euOnlyBaseURL
		}
	} else {
		f.urlIsCustom = true
		f.baseURL = cfg.BaseURL
	}
	if cfg.PollingMode == AutoPoll {
		// Start a fetcher goroutine immediately
		// to avoid a potential double fetch
		// when someone calls Refresh immediately
		// after creating the client.
		_ = f.refreshIfOlder(f.ctx, time.Time{}, false)
		f.wg.Add(1)
		go f.runPoller(cfg.PollInterval)
	}
	return f
}

func (f *configFetcher) isOffline() bool {
	return atomic.LoadUint32(&f.offline) == 1
}

func (f *configFetcher) setMode(offline bool) {
	if offline {
		atomic.StoreUint32(&f.offline, 1)
		f.logger.Infof(5200, "switched to OFFLINE mode")
	} else {
		atomic.StoreUint32(&f.offline, 0)
		f.logger.Infof(5200, "switched to ONLINE mode")
	}
}

func (f *configFetcher) context() context.Context {
	return f.ctx
}

func (f *configFetcher) doneInitGet() chan struct{} {
	return f.doneInitialGet
}

func (f *configFetcher) close() {
	f.ctxCancel()
	f.wg.Wait()
}

func (f *configFetcher) runPoller(pollInterval time.Duration) {
	defer f.wg.Done()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-f.ctx.Done():
			return
		}
		_ = f.refreshIfOlder(f.ctx, time.Now().Add(-pollInterval/2), true)
	}
}

// current returns the current configuration.
func (f *configFetcher) current() *config {
	cfg, _ := f.config.Load().(*config)
	return cfg
}

// refreshIfOlder refreshes the configuration if it was retrieved
// before the given time or if there is no current configuration. If the context is
// canceled while the refresh is in progress, Refresh will return but
// the underlying HTTP request will not be stopped.
//
// If wait is false, refreshIfOlder returns immediately without waiting
// for the refresh to complete.
func (f *configFetcher) refreshIfOlder(ctx context.Context, before time.Time, wait bool) error {
	f.mu.Lock()
	prevConfig := f.current()
	if prevConfig != nil && !prevConfig.fetchTime.Before(before) {
		f.mu.Unlock()
		return nil
	}
	fetchDone := f.fetchDone
	if fetchDone == nil {
		fetchDone = make(chan error, 1)
		f.fetchDone = fetchDone
		f.wg.Add(1)
		go f.fetcher(prevConfig)
	}
	f.mu.Unlock()
	if !wait {
		return nil
	}
	select {
	case err := <-fetchDone:
		// Put the error back in the channel so that other
		// concurrent refresh calls can have access to it.
		fetchDone <- err
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// fetcher fetches the latest available configuration, updates f.config and possibly
// f.baseURL, and sends the result on fetchDone.
//
// Note: although this is started asynchronously, the configFetcher
// logic guarantees that there's never more than one goroutine
// at a time running f.fetcher.
func (f *configFetcher) fetcher(prevConfig *config) {
	defer f.wg.Done()
	config, newURL, err := f.fetchConfig(f.ctx, f.baseURL, prevConfig)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil {
		var fErr *fetcherError
		if errors.As(err, &fErr) {
			f.logger.Errorf(fErr.EventId, "config fetch failed: %v", fErr.Err)
		} else {
			f.logger.Errorf(0, "config fetch failed: %v", err)
		}
		err = fmt.Errorf("config fetch failed: %v", err)
	} else if config != nil && !config.equal(prevConfig) {
		f.baseURL = newURL
		f.config.Store(config)
		if err := f.saveToCache(f.ctx, config.fetchTime, config.etag, config.jsonBody); err != nil {
			f.logger.Errorf(2201, "error occurred while writing the cache: %v", err)
		}
		contentEquals := config.equalContent(prevConfig)
		if f.hooks != nil && f.hooks.OnConfigChanged != nil && !contentEquals {
			go f.hooks.OnConfigChanged()
		}
	}
	// Unblock any Client.getValue call that's waiting for the first configuration to be retrieved.
	f.doneGetOnce.Do(func() {
		close(f.doneInitialGet)
	})
	f.fetchDone <- err
	f.fetchDone = nil
}

func (f *configFetcher) fetchConfig(ctx context.Context, baseURL string, prevConfig *config) (_ *config, _newURL string, _err error) {
	if f.overrides != nil && f.overrides.Behavior == LocalOnly {
		// TODO could potentially refresh f.overrides if it's come from a file.
		cfg, err := parseConfig(nil, "", time.Now(), f.logger, f.defaultUser, f.overrides, f.hooks)
		if err != nil {
			return nil, "", err
		}
		return cfg, "", nil
	}

	// If we are in offline mode skip HTTP completely and fall back to cache every time.
	if f.isOffline() {
		if f.cache == nil {
			return nil, "", &fetcherError{EventId: 0, Err: fmt.Errorf("the SDK is in offline mode and no cache is configured")}
		}
		cfg := f.readCache(ctx, prevConfig)
		if cfg == nil {
			return nil, "", &fetcherError{EventId: 0, Err: fmt.Errorf("the SDK is in offline mode and wasn't able to read a valid configuration from the cache")}
		}
		return cfg, baseURL, nil
	}

	// We are online, use HTTP
	cfg, newBaseURL, err := f.fetchHTTP(ctx, baseURL, prevConfig)
	if err == nil {
		return cfg, newBaseURL, nil
	}
	cfg = f.readCache(ctx, prevConfig)
	if cfg == nil {
		return nil, "", err
	}
	return cfg, baseURL, nil
}

func (f *configFetcher) readCache(ctx context.Context, prevConfig *config) (_ *config) {
	if f.cache == nil {
		return nil
	}
	// Fall back to the cache
	fetchTime, eTag, configBytes, cacheErr := f.parseFromCache(ctx)
	if cacheErr != nil {
		f.logger.Errorf(2200, "error occurred while reading the cache: %v", cacheErr)
		return nil
	}
	cfg, parseErr := parseConfig(configBytes, eTag, fetchTime, f.logger, f.defaultUser, f.overrides, f.hooks)
	if parseErr != nil {
		f.logger.Errorf(2200, "error occurred while reading the cache; cache contained invalid config: %v", parseErr)
		return nil
	}
	if prevConfig == nil || !cfg.fetchTime.Before(prevConfig.fetchTime) {
		f.logger.Debugf("returning cached config %v", cfg.body())
		return cfg
	}
	// The cached config is older than the one we already had.
	return nil
}

func (f *configFetcher) parseFromCache(ctx context.Context) (fetchTime time.Time, eTag string, config []byte, err error) {
	cacheText, cacheErr := f.cache.Get(ctx, f.cacheKey)
	if cacheErr != nil {
		return time.Time{}, "", nil, cacheErr
	}

	return configcatcache.CacheSegmentsFromBytes(cacheText)
}

func (f *configFetcher) saveToCache(ctx context.Context, fetchTime time.Time, eTag string, config []byte) (err error) {
	if f.cache == nil {
		return nil
	}

	toCache := configcatcache.CacheSegmentsToBytes(fetchTime, eTag, config)
	return f.cache.Set(ctx, f.cacheKey, toCache)
}

// fetchHTTP fetches the configuration while respecting redirects.
// The prevConfig argument is used to avoid network traffic when the
// configuration hasn't changed on the server. The NotModified
// fetch status is never used.
//
// It returns the newly fetched configuration and the new base URL
// (empty if it hasn't changed).
func (f *configFetcher) fetchHTTP(ctx context.Context, baseURL string, prevConfig *config) (newConfig *config, newBaseURL string, err error) {
	f.logger.Debugf("fetching from %v", baseURL)
	for i := 0; i < 3; i++ {
		config, err := f.fetchHTTPWithoutRedirect(ctx, baseURL, prevConfig)
		if err != nil {
			return nil, "", err
		}
		preferences := config.root.Preferences
		if preferences == nil ||
			preferences.Redirect == nil ||
			preferences.URL == "" ||
			preferences.URL == baseURL {
			return config, baseURL, nil
		}
		redirect := *preferences.Redirect
		if redirect == ForceRedirect {
			f.logger.Debugf("forced redirect to %v (count %d)", preferences.URL, i+1)
			baseURL = preferences.URL
			continue
		}
		if f.urlIsCustom {
			if redirect == NoDirect {
				// The config is available, but we won't respect the redirection
				// request for a custom URL.
				f.logger.Debugf("config fetched but refusing to redirect from custom URL without forced redirection")
				return config, baseURL, nil
			}
			// With shouldRedirect, there is no configuration available
			// other than the redirection information itself, so error.
			return nil, "", &fetcherError{EventId: 0, Err: fmt.Errorf("refusing to redirect from custom URL without forced redirection")}
		}
		if preferences.URL == "" {
			return nil, "", &fetcherError{EventId: 0, Err: fmt.Errorf("refusing to redirect to empty URL")}
		}
		baseURL = preferences.URL

		f.logger.Warnf(3002,
			"the `config.DataGovernance` parameter specified at the client initialization is not in sync with the preferences on the ConfigCat Dashboard; "+
				"read more: https://configcat.com/docs/advanced/data-governance/",
		)
		if redirect == NoDirect {
			// We've already got the configuration data, we'll just fetch
			// from the redirected URL next time.
			f.logger.Debugf("redirection on next fetch to %v", baseURL)
			return config, baseURL, nil
		}
		if redirect != ShouldRedirect {
			return nil, "", &fetcherError{EventId: 0, Err: fmt.Errorf("unknown redirection kind %d in response", redirect)}
		}
		f.logger.Debugf("redirecting to %v", baseURL)
	}
	return nil, "", &fetcherError{EventId: 1104, Err: fmt.Errorf("redirection loop encountered while trying to fetch config JSON; please contact us at https://configcat.com/support/")}
}

// fetchHTTPWithoutRedirect does the actual HTTP fetch of the config.
func (f *configFetcher) fetchHTTPWithoutRedirect(ctx context.Context, baseURL string, prevConfig *config) (*config, error) {
	if f.sdkKey == "" {
		return nil, &fetcherError{EventId: 0, Err: fmt.Errorf("empty SDK key in configcat configuration")}
	}
	request, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/configuration-files/"+f.sdkKey+"/"+configcatcache.ConfigJSONName, nil)
	if err != nil {
		return nil, &fetcherError{EventId: 0, Err: err}
	}
	request.Header.Set("X-ConfigCat-UserAgent", "ConfigCat-Go/"+f.pollingIdentifier+"-"+version)

	if prevConfig != nil && prevConfig.etag != "" {
		request.Header.Add("If-None-Match", prevConfig.etag)
	}
	request = request.WithContext(f.ctx)
	response, err := f.client.Do(request)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, &fetcherError{EventId: 1102, Err: fmt.Errorf("request timed out while trying to fetch config JSON. (timeout value: %dms) %v", f.timeout.Milliseconds(), err)}
		} else {
			return nil, &fetcherError{EventId: 1103, Err: fmt.Errorf("unexpected error occurred while trying to fetch config JSON: %v", err)}
		}
	}
	defer response.Body.Close()

	if response.StatusCode == 304 {
		f.logger.Debugf("config fetch succeeded: not modified")
		return prevConfig.withFetchTime(time.Now()), nil
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, &fetcherError{EventId: 1103, Err: fmt.Errorf("unexpected error occurred while trying to fetch config JSON; read failed: %v", err)}
		}
		config, err := parseConfig(body, response.Header.Get("Etag"), time.Now(), f.logger, f.defaultUser, f.overrides, f.hooks)
		if err != nil {
			return nil, &fetcherError{EventId: 1105, Err: fmt.Errorf("fetching config JSON was successful but the HTTP response content was invalid: %v", err)}
		}
		f.logger.Debugf("config fetch succeeded: new config fetched")
		return config, nil
	}
	if response.StatusCode == http.StatusNotFound {
		return nil, &fetcherError{EventId: 1100, Err: fmt.Errorf("your SDK Key seems to be wrong; you can find the valid SDK Key at https://app.configcat.com/sdkkey")}
	}
	return nil, &fetcherError{EventId: 1101, Err: fmt.Errorf("unexpected HTTP response was received while trying to fetch config JSON: %v", response.Status)}
}

func pollingModeToIdentifier(pollingMode PollingMode) string {
	switch pollingMode {
	case AutoPoll:
		return "a"
	case Lazy:
		return "l"
	case Manual:
		return "m"
	default:
		return "-"
	}
}

type emptyFetcher struct {
	doneInitialGet chan struct{}
}

func newEmptyFetcher() fetcher {
	f := &emptyFetcher{doneInitialGet: make(chan struct{})}
	close(f.doneInitialGet)
	return f
}

func (e *emptyFetcher) refreshIfOlder(_ context.Context, _ time.Time, _ bool) error {
	return errors.New("config fetch failed: SDK Key is invalid")
}

func (e *emptyFetcher) close() {
	// no action
}

func (e *emptyFetcher) current() *config {
	return nil
}

func (e *emptyFetcher) isOffline() bool {
	return true
}

func (e *emptyFetcher) setMode(_ bool) {
	// no action
}

func (e *emptyFetcher) context() context.Context {
	return context.TODO()
}

func (e *emptyFetcher) doneInitGet() chan struct{} {
	return e.doneInitialGet
}
