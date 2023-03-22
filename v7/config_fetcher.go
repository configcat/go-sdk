package configcat

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/configcat/go-sdk/v7/internal/wireconfig"
)

const configJSONName = "config_v5"

const (
	globalBaseURL = "https://cdn-global.configcat.com"
	euOnlyBaseURL = "https://cdn-eu.configcat.com"
)

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
func newConfigFetcher(cfg Config, logger *leveledLogger, defaultUser User) *configFetcher {
	f := &configFetcher{
		sdkKey:       cfg.SDKKey,
		cache:        cfg.Cache,
		cacheKey:     sdkKeyToCacheKey(cfg.SDKKey),
		overrides:    cfg.FlagOverrides,
		changeNotify: cfg.ChangeNotify,
		hooks:        cfg.Hooks,
		logger:       logger,
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
		f.refreshIfOlder(f.ctx, time.Time{}, false)
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
		f.logger.Infof(5200, "Switched to OFFLINE mode.")
	} else {
		atomic.StoreUint32(&f.offline, 0)
		f.logger.Infof(5200, "Switched to ONLINE mode.")
	}
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
		if err := f.refreshIfOlder(f.ctx, time.Now().Add(-pollInterval/2), true); err != nil {
			f.logger.Errorf(0, "cannot refresh configcat configuration: %v", err)
		}
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
		logError := !wait
		go f.fetcher(prevConfig, logError)
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
func (f *configFetcher) fetcher(prevConfig *config, logError bool) {
	defer f.wg.Done()
	config, newURL, err := f.fetchConfig(f.ctx, f.baseURL, prevConfig, logError)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil {
		err = fmt.Errorf("config fetch failed: %v", err)
	} else if config != nil && !config.equal(prevConfig) {
		f.baseURL = newURL
		f.config.Store(config)
		if f.cache != nil {
			if err := f.cache.Set(f.ctx, f.cacheKey, config.jsonBody); err != nil {
				f.logger.Errorf(2201, "Error occurred while writing the cache: %v", err)
			}
		}
		contentEquals := config.equalContent(prevConfig)
		if f.changeNotify != nil && !contentEquals {
			go f.changeNotify()
		}
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

func (f *configFetcher) fetchConfig(ctx context.Context, baseURL string, prevConfig *config, logError bool) (_ *config, _newURL string, _err error) {
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
			var message = "the SDK is in offline mode and no cache is configured"
			if logError {
				f.logger.Errorf(0, message)
			}
			return nil, "", fmt.Errorf(message)
		}
		cfg := f.readCache(ctx, prevConfig)
		if cfg == nil {
			var message = "the SDK is in offline mode and wasn't able to read a valid configuration from the cache"
			if logError {
				f.logger.Errorf(0, message)
			}
			return nil, "", fmt.Errorf(message)
		}
		return cfg, baseURL, nil
	}

	// We are online, use HTTP
	cfg, newBaseURL, err := f.fetchHTTP(ctx, baseURL, prevConfig, logError)
	if err == nil {
		return cfg, newBaseURL, nil
	}
	f.logger.Infof(0, "falling back to cache after config fetch error: %v", err)
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
	configText, cacheErr := f.cache.Get(ctx, f.cacheKey)
	if cacheErr != nil {
		f.logger.Errorf(2200, "Error occurred while reading the cache: %v", cacheErr)
		return nil
	}
	if len(configText) == 0 {
		f.logger.Debugf("empty config text in cache")
		return nil
	}
	cfg, parseErr := parseConfig(configText, "", time.Time{}, f.logger, f.defaultUser, f.overrides, f.hooks)
	if parseErr != nil {
		f.logger.Errorf(2200, "Error occurred while reading the cache. Cache contained invalid config: %v", parseErr)
		return nil
	}
	if prevConfig == nil || !cfg.fetchTime.Before(prevConfig.fetchTime) {
		f.logger.Debugf("returning cached config %v", cfg.body())
		return cfg
	}
	// The cached config is older than the one we already had.
	return nil
}

// fetchHTTP fetches the configuration while respecting redirects.
// The prevConfig argument is used to avoid network traffic when the
// configuration hasn't changed on the server. The NotModified
// fetch status is never used.
//
// It returns the newly fetched configuration and the new base URL
// (empty if it hasn't changed).
func (f *configFetcher) fetchHTTP(ctx context.Context, baseURL string, prevConfig *config, logError bool) (newConfig *config, newBaseURL string, err error) {
	f.logger.Infof(0, "fetching from %v", baseURL)
	for i := 0; i < 3; i++ {
		config, err := f.fetchHTTPWithoutRedirect(ctx, baseURL, prevConfig, logError)
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
		if redirect == wireconfig.ForceRedirect {
			f.logger.Infof(0, "forced redirect to %v (count %d)", preferences.URL, i+1)
			baseURL = preferences.URL
			continue
		}
		if f.urlIsCustom {
			if redirect == wireconfig.Nodirect {
				// The config is available, but we won't respect the redirection
				// request for a custom URL.
				f.logger.Infof(0, "config fetched but refusing to redirect from custom URL without forced redirection")
				return config, baseURL, nil
			}
			// With shouldRedirect, there is no configuration available
			// other than the redirection information itself, so error.
			var message = "refusing to redirect from custom URL without forced redirection"
			if logError {
				f.logger.Errorf(0, message)
			}
			return nil, "", fmt.Errorf(message)
		}
		if preferences.URL == "" {
			var message = "refusing to redirect to empty URL"
			if logError {
				f.logger.Errorf(0, message)
			}
			return nil, "", fmt.Errorf(message)
		}
		baseURL = preferences.URL

		f.logger.Warnf(3002,
			"The `config.DataGovernance` parameter specified at the client initialization is not in sync with the preferences on the ConfigCat Dashboard. " +
			"Read more: https://configcat.com/docs/advanced/data-governance/",
		)
		if redirect == wireconfig.Nodirect {
			// We've already got the configuration data, we'll just fetch
			// from the redirected URL next time.
			f.logger.Infof(0, "redirection on next fetch to %v", baseURL)
			return config, baseURL, nil
		}
		if redirect != wireconfig.ShouldRedirect {
			var message = "unknown redirection kind %d in response"
			if logError {
				f.logger.Errorf(0, message, redirect)
			}
			return nil, "", fmt.Errorf(message, redirect)
		}
		f.logger.Infof(0, "redirecting to %v", baseURL)
	}
	var message = "Redirection loop encountered while trying to fetch config JSON. Please contact us at https://configcat.com/support/"
	if logError {
		f.logger.Errorf(1104, message)
	}
	return nil, "", fmt.Errorf(message)
}

// fetchHTTPWithoutRedirect does the actual HTTP fetch of the config.
func (f *configFetcher) fetchHTTPWithoutRedirect(ctx context.Context, baseURL string, prevConfig *config, logError bool) (*config, error) {
	if f.sdkKey == "" {
		var message = "empty SDK key in configcat configuration"
		if logError {
			f.logger.Errorf(0, message)
		}
		return nil, fmt.Errorf(message)
	}
	request, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/configuration-files/"+f.sdkKey+"/"+configJSONName+".json", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-ConfigCat-UserAgent", "ConfigCat-Go/"+f.pollingIdentifier+"-"+version)

	if prevConfig != nil && prevConfig.etag != "" {
		request.Header.Add("If-None-Match", prevConfig.etag)
	}
	request = request.WithContext(f.ctx)
	response, err := f.client.Do(request)
	if err != nil {
		var message = "Unexpected error occurred while trying to fetch config JSON: %v"
		if logError {
			f.logger.Errorf(1103, message, err)
		}
		return nil, fmt.Errorf(message, err)
	}
	defer response.Body.Close()

	if response.StatusCode == 304 {
		f.logger.Debugf("Config fetch succeeded: not modified.")
		return prevConfig.withFetchTime(time.Now()), nil
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			var message = "Unexpected error occurred while trying to fetch config JSON. Read failed: %v"
			if logError {
				f.logger.Errorf(1103, message, err)
			}
			return nil, fmt.Errorf(message, err)
		}
		config, err := parseConfig(body, response.Header.Get("Etag"), time.Now(), f.logger, f.defaultUser, f.overrides, f.hooks)
		if err != nil {
			var message = "Fetching config JSON was successful but the HTTP response content was invalid: %v"
			if logError {
				f.logger.Errorf(1105, message, err)
			}
			return nil, fmt.Errorf(message, err)
		}
		f.logger.Debugf("Config fetch succeeded: new config fetched.")
		return config, nil
	}
	if response.StatusCode == http.StatusNotFound {
		var message = "Your SDK Key seems to be wrong. You can find the valid SDK Key at https://app.configcat.com/sdkkey"
		if logError {
			f.logger.Errorf(1100, message)
		}
		return nil, fmt.Errorf(message)
	}
	var message = "Unexpected HTTP response was received while trying to fetch config JSON: %v"
	if logError {
		f.logger.Errorf(1101, message, response.Status)
	}
	return nil, fmt.Errorf(message, response.Status)
}

func sdkKeyToCacheKey(sdkKey string) string {
	return fmt.Sprintf("go_"+configJSONName+"_%x", sha1.Sum([]byte(sdkKey)))
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
