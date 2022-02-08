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
	sdkKey       string
	cacheKey     string
	cache        ConfigCache
	logger       *leveledLogger
	client       *http.Client
	urlIsCustom  bool
	changeNotify func()

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
func newConfigFetcher(cfg Config, logger *leveledLogger) *configFetcher {
	f := &configFetcher{
		sdkKey:       cfg.SDKKey,
		cache:        cfg.Cache,
		cacheKey:     sdkKeyToCacheKey(cfg.SDKKey),
		changeNotify: cfg.ChangeNotify,
		logger:       logger,
		client: &http.Client{
			Timeout:   cfg.HTTPTimeout,
			Transport: cfg.Transport,
		},
		doneInitialGet: make(chan struct{}),
	}
	f.ctx, f.ctxCancel = context.WithCancel(context.Background())
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
			f.logger.Errorf("cannot refresh configcat configuration: %v", err)
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
	config, newURL, err := f.fetchConfig(f.ctx, f.baseURL, prevConfig)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil {
		err = fmt.Errorf("config fetch failed: %v", err)
		if logError {
			f.logger.Errorf("%v", err)
		}
	} else if config != nil && !config.equal(prevConfig) {
		f.baseURL = newURL
		f.config.Store(config)
		if f.cache != nil {
			if err := f.cache.Set(f.ctx, f.cacheKey, config.jsonBody); err != nil {
				f.logger.Errorf("failed to save configuration to cache: %v", err)
			}
		}
		if f.changeNotify != nil && !config.equalContent(prevConfig) {
			go f.changeNotify()
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
	cfg, newBaseURL, err := f.fetchHTTP(ctx, baseURL, prevConfig)
	if err == nil {
		return cfg, newBaseURL, nil
	}
	if f.cache == nil {
		return nil, "", err
	}
	f.logger.Infof("falling back to cache after config fetch error: %v", err)
	// Fall back to the cache
	configText, cacheErr := f.cache.Get(ctx, f.cacheKey)
	if cacheErr != nil {
		f.logger.Errorf("cache get failed: %v", cacheErr)
		return nil, "", err
	}
	if len(configText) == 0 {
		f.logger.Debugf("empty config text in cache")
		return nil, "", err
	}
	cfg, cacheErr = parseConfig(configText, "", time.Time{})
	if cacheErr != nil {
		f.logger.Errorf("cache contained invalid config: %v", err)
		return nil, "", err
	}
	if prevConfig == nil || !cfg.fetchTime.Before(prevConfig.fetchTime) {
		f.logger.Debugf("returning cached config %v", cfg.body())
		return cfg, baseURL, nil
	}
	// The cached config is older than the one we already had.
	return nil, "", err
}

// fetchHTTP fetches the configuration while respecting redirects.
// The prevConfig argument is used to avoid network traffic when the
// configuration hasn't changed on the server. The NotModified
// fetch status is never used.
//
// It returns the newly fetched configuration and the new base URL
// (empty if it hasn't changed).
func (f *configFetcher) fetchHTTP(ctx context.Context, baseURL string, prevConfig *config) (newConfig *config, newBaseURL string, err error) {
	f.logger.Infof("fetching from %v", baseURL)
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
		if redirect == wireconfig.ForceRedirect {
			f.logger.Infof("forced redirect to %v (count %d)", preferences.URL, i+1)
			baseURL = preferences.URL
			continue
		}
		if f.urlIsCustom {
			if redirect == wireconfig.Nodirect {
				// The config is available, but we won't respect the redirection
				// request for a custom URL.
				f.logger.Infof("config fetched but refusing to redirect from custom URL without forced redirection")
				return config, baseURL, nil
			}
			// With shouldRedirect, there is no configuration available
			// other than the redirection information itself, so error.
			return nil, "", fmt.Errorf("refusing to redirect from custom URL without forced redirection")
		}
		if preferences.URL == "" {
			return nil, "", fmt.Errorf("refusing to redirect to empty URL")
		}
		baseURL = preferences.URL

		f.logger.Warnf(
			"Your config.DataGovernance parameter at ConfigCatClient " +
				"initialization is not in sync with your preferences on the ConfigCat " +
				"Dashboard: https://app.configcat.com/organization/data-governance. " +
				"Only Organization Admins can access this preference.",
		)
		if redirect == wireconfig.Nodirect {
			// We've already got the configuration data, we'll just fetch
			// from the redirected URL next time.
			f.logger.Infof("redirection on next fetch to %v", baseURL)
			return config, baseURL, nil
		}
		if redirect != wireconfig.ShouldRedirect {
			return nil, "", fmt.Errorf("unknown redirection kind %d in response", redirect)
		}
		f.logger.Infof("redirecting to %v", baseURL)
	}
	return nil, "", fmt.Errorf("redirect loop during config.json fetch. Please contact support@configcat.com")
}

// fetchHTTPWithoutRedirect does the actual HTTP fetch of the config.
func (f *configFetcher) fetchHTTPWithoutRedirect(ctx context.Context, baseURL string, prevConfig *config) (*config, error) {
	if f.sdkKey == "" {
		return nil, fmt.Errorf("empty SDK key in configcat configuration!")
	}
	request, err := http.NewRequest("GET", baseURL+"/configuration-files/"+f.sdkKey+"/"+configJSONName+".json", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-ConfigCat-UserAgent", "ConfigCat-Go/"+version)

	if prevConfig != nil && prevConfig.etag != "" {
		request.Header.Add("If-None-Match", prevConfig.etag)
	}
	request = request.WithContext(f.ctx)
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("config fetch request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode == 304 {
		f.logger.Debugf("Config fetch succeeded: not modified.")
		return prevConfig.withFetchTime(time.Now()), nil
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("config fetch read failed: %v", err)
		}
		config, err := parseConfig(body, response.Header.Get("Etag"), time.Now())
		if err != nil {
			return nil, fmt.Errorf("config fetch returned invalid body: %v", err)
		}
		f.logger.Debugf("Config fetch succeeded: new config fetched.")
		return config, nil
	}
	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf(
			"configuration not found. " +
				"Double-check your SDK KEY at https://app.configcat.com/sdkkey",
		)
	}
	return nil, fmt.Errorf("received unexpected response %v", response.Status)
}

func sdkKeyToCacheKey(sdkKey string) string {
	return fmt.Sprintf("go_"+configJSONName+"_%x", sha1.Sum([]byte(sdkKey)))
}
