package configcat

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

const (
	ConfigJsonName = "config_v5"

	NoRedirect     = 0
	ShouldRedirect = 1
	ForceRedirect  = 2
)

// fetchResponse represents a configuration fetch response.
type fetchResponse struct {
	status fetchStatus
	config *config
}

type configFetcher struct {
	userAgent string
	sdkKey    string
	cacheKey  string
	cache     configCache
	logger    *leveledLogger
	client    *http.Client

	urlIsCustom bool

	// baseUrl is maintained by the fetcher goroutine.
	baseUrl string

	mu            sync.Mutex
	currentConfig *config
	prevConfig    *config
	fetchDone     chan struct{}
}

// newConfigFetcher returns a
func newConfigFetcher(sdkKey string, cache configCache, config ClientConfig, logger *leveledLogger) *configFetcher {
	f := &configFetcher{
		sdkKey:    sdkKey,
		cache:     cache,
		cacheKey:  sdkKeyToCacheKey(sdkKey),
		userAgent: "ConfigCat-Go/" + config.Mode.getModeIdentifier() + "-" + version,
		logger:    logger,
		client:    &http.Client{Timeout: config.HttpTimeout, Transport: config.Transport},
	}
	if config.BaseUrl == "" {
		if config.DataGovernance == Global {
			f.baseUrl = globalBaseUrl
		} else {
			f.baseUrl = euOnlyBaseUrl
		}
	} else {
		f.urlIsCustom = true
		f.baseUrl = config.BaseUrl
	}
	return f
}

// config returns the current config. If the current config isn't
// available, it'll wait until it is. If the context expires when waiting,
// it returns the configuration from the cache, or the most recent
// if that also fails.
func (f *configFetcher) config(ctx context.Context) *config {
	// We loop here, because it's possible that even if we've
	// waited for a running fetch to be completed, it might
	// have completed, the value invalidated and another
	// fetch to have been started. In practice we'll almost
	// never go round the loop more than twice.
	for {
		f.mu.Lock()
		conf := f.currentConfig
		fetchDone := f.fetchDone
		f.mu.Unlock()

		if conf != nil {
			return conf
		}
		if fetchDone == nil {
			// No fetch in progress, so return a cached value immediately.
			return f.cachedConfig()
		}
		select {
		case <-fetchDone:
		case <-ctx.Done():
			return f.cachedConfig()
		}
	}
}

// startRefresh starts a refresh going asynchronously if there's
// not one in progress already. It does not invalidate
// the current configuration.
func (f *configFetcher) startRefresh() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f._startRefresh()
}

// refresh starts a refresh and waits for it to complete
// or the context to be canceled.
func (f *configFetcher) refresh(ctx context.Context) {
	f.mu.Lock()
	f._startRefresh()
	fetchDone := f.fetchDone
	f.mu.Unlock()
	select {
	case <-fetchDone:
	case <-ctx.Done():
	}
}

// invalidateAndStartRefresh invalidates the current config
// and starts fetching a new one.
func (f *configFetcher) invalidateAndStartRefresh() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.currentConfig = nil
	f._startRefresh()
}

// _startRefresh is the fetcher-internal version of startRefresh.
// It's called with c.mu held.
func (f *configFetcher) _startRefresh() {
	if f.fetchDone == nil {
		f.fetchDone = make(chan struct{})
		go f.fetch()
	}
}

// cachedConfig returns the configuration from the cache,
// or the most recently returned configuration if that fails.
func (f *configFetcher) cachedConfig() *config {
	conf, err := f.cache.get(f.cacheKey)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil || conf == nil {
		return f.prevConfig
	}
	// Note: don't update f.conf, because cached data
	// is by-definition potentially stale, but do update
	// prevConfig so that if the cache fails in the future,
	// we'll fall back to using this configuration.
	f.prevConfig = conf
	return conf
}

// fetch fetches the the current configuration from the HTTP server.
// Note: although this is started asynchronously, the configFetcher
// logic guarantees that there's never more than one goroutine
// at a time running fetch.
//
// When it's finished, it closes the fetchDone channel to wake
// up anyone that's waiting for the configuration, and
// sets it to nil to signify that there's no longer an outstanding
// fetch running.
func (f *configFetcher) fetch() {
	f.mu.Lock()
	prevConfig := f.prevConfig
	f.mu.Unlock()

	resp := f.fetchHTTP(prevConfig)

	f.mu.Lock()
	defer f.mu.Unlock()
	// Note: don't invalidate any existing config because
	// we can't fetch it.
	if resp.status == Fetched {
		f.currentConfig = resp.config
		f.prevConfig = resp.config
		f.cache.set(f.cacheKey, resp.config)
	}
	close(f.fetchDone)
	f.fetchDone = nil
}

// fetchHTTP does fetches the configuration while respecting redirects.
// The prevConfig argument is used to avoid network traffic when the
// configuration hasn't changed on the server. The NotModified
// fetch status is never used.
func (f *configFetcher) fetchHTTP(prevConfig *config) fetchResponse {
	attempts := 2
	for {
		resp := f.fetchHTTPWithoutRedirect(prevConfig)
		if resp.status != Fetched {
			return resp
		}
		preferences := resp.config.root.Preferences
		if preferences == nil ||
			preferences.URL == "" ||
			preferences.URL == f.baseUrl ||
			preferences.Redirect == nil {
			return resp
		}
		redirect := *preferences.Redirect

		if f.urlIsCustom && redirect != ForceRedirect {
			return resp
		}

		// Note: it's only OK to set this without acquiring the mutex because
		// of the guarantee that there's only one fetcher active at a time.
		f.baseUrl = preferences.URL
		if redirect == NoRedirect {
			return resp
		}

		if redirect == ShouldRedirect {
			f.logger.Warn("Your config.DataGovernance parameter at ConfigCatClient " +
				"initialization is not in sync with your preferences on the ConfigCat " +
				"Dashboard: https://app.configcat.com/organization/data-governance. " +
				"Only Organization Admins can access this preference.")
		}
		if attempts <= 0 {
			f.logger.Error("Redirect loop during config.json fetch. Please contact support@configcat.com.")
			return resp
		}
		attempts--
	}
}

// fetchHTTPWithoutRedirect does the actual HTTP fetch of the config.
func (f *configFetcher) fetchHTTPWithoutRedirect(prevConfig *config) fetchResponse {
	request, err := http.NewRequest("GET", f.baseUrl+"/configuration-files/"+f.sdkKey+"/"+ConfigJsonName+".json", nil)
	if err != nil {
		return fetchResponse{status: Failure}
	}
	request.Header.Add("X-ConfigCat-UserAgent", f.userAgent)

	if prevConfig != nil && prevConfig.etag != "" {
		request.Header.Add("If-None-Match", prevConfig.etag)
	}

	response, err := f.client.Do(request)
	if err != nil {
		f.logger.Errorf("Config fetch failed: %v", err)
		return fetchResponse{status: Failure}
	}
	defer response.Body.Close()

	if response.StatusCode == 304 {
		f.logger.Debug("Config fetch succeeded: not modified.")
		return fetchResponse{
			status: Fetched,
			config: prevConfig.withFetchTime(time.Now()),
		}
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			f.logger.Errorf("Config fetch failed: %v", err)
			return fetchResponse{status: Failure}
		}
		config, err := parseConfig(body, response.Header.Get("Etag"), time.Now())
		if err != nil {
			f.logger.Errorf("Config fetch returned invalid body: %v", err)
			return fetchResponse{status: Failure}
		}

		f.logger.Debug("Config fetch succeeded: new config fetched.")
		return fetchResponse{status: Fetched, config: config}
	}

	f.logger.Errorf("Double-check your SDK KEY at https://app.configcat.com/sdkkey. "+
		"Received unexpected response: %v.", response.StatusCode)
	return fetchResponse{status: Failure}
}

const CacheBase = "go_" + ConfigJsonName + "_%s"

func sdkKeyToCacheKey(sdkKey string) string {
	sha := sha1.New()
	sha.Write([]byte(sdkKey))
	hash := hex.EncodeToString(sha.Sum(nil))
	return fmt.Sprintf(CacheBase, hash)
}
