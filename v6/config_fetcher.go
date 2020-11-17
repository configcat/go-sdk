package configcat

import (
	"io/ioutil"
	"net/http"
)

const (
	ConfigJsonName = "config_v5"

	NoRedirect     = 0
	ShouldRedirect = 1
	ForceRedirect  = 2
)

type configProvider interface {
	getConfigurationAsync() *asyncResult
}

type configFetcher struct {
	sdkKey, eTag, mode, baseUrl string
	urlIsCustom                 bool
	client                      *http.Client
	logger                      Logger
}

func newConfigFetcher(sdkKey string, config ClientConfig) *configFetcher {
	fetcher := &configFetcher{sdkKey: sdkKey,
		mode:   config.Mode.getModeIdentifier(),
		logger: config.Logger,
		client: &http.Client{Timeout: config.HttpTimeout, Transport: config.Transport},
	}

	if config.BaseUrl == "" {
		fetcher.urlIsCustom = false
		if config.DataGovernance == Global {
			fetcher.baseUrl = globalBaseUrl
		} else {
			fetcher.baseUrl = euOnlyBaseUrl
		}
	} else {
		fetcher.urlIsCustom = true
		fetcher.baseUrl = config.BaseUrl
	}

	return fetcher
}

func (fetcher *configFetcher) getConfigurationAsync() *asyncResult {
	return fetcher.executeFetchAsync(2)
}

func (fetcher *configFetcher) executeFetchAsync(executionCount int) *asyncResult {
	return fetcher.sendFetchRequestAsync().compose(func(result interface{}) *asyncResult {
		fetchResponse, ok := result.(fetchResponse)
		if !ok || !fetchResponse.isFetched() {
			return asCompletedAsyncResult(result)
		}

		preferences := fetchResponse.config.root.Preferences

		if preferences == nil {
			return asCompletedAsyncResult(fetchResponse)
		}

		if preferences.URL == "" || preferences.URL == fetcher.baseUrl {
			return asCompletedAsyncResult(fetchResponse)
		}

		if preferences.Redirect == nil {
			return asCompletedAsyncResult(fetchResponse)
		}
		redirect := *preferences.Redirect

		if fetcher.urlIsCustom && redirect != ForceRedirect {
			return asCompletedAsyncResult(fetchResponse)
		}

		fetcher.baseUrl = preferences.URL
		if redirect == NoRedirect {
			return asCompletedAsyncResult(fetchResponse)
		}
		if redirect == ShouldRedirect {
			fetcher.logger.Warnln("Your config.DataGovernance parameter at ConfigCatClient " +
				"initialization is not in sync with your preferences on the ConfigCat " +
				"Dashboard: https://app.configcat.com/organization/data-governance. " +
				"Only Organization Admins can access this preference.")
		}

		if executionCount > 0 {
			return fetcher.executeFetchAsync(executionCount - 1)
		}

		fetcher.logger.Errorln("Redirect loop during config.json fetch. Please contact support@configcat.com.")
		return asCompletedAsyncResult(fetchResponse)
	})
}

func (fetcher *configFetcher) sendFetchRequestAsync() *asyncResult {
	result := newAsyncResult()

	go func() {
		request, requestError := http.NewRequest("GET", fetcher.baseUrl+"/configuration-files/"+fetcher.sdkKey+"/"+ConfigJsonName+".json", nil)
		if requestError != nil {
			result.complete(fetchResponse{status: Failure})
			return
		}

		request.Header.Add("X-ConfigCat-UserAgent", "ConfigCat-Go/"+fetcher.mode+"-"+version)

		if fetcher.eTag != "" {
			request.Header.Add("If-None-Match", fetcher.eTag)
		}

		response, responseError := fetcher.client.Do(request)
		if responseError != nil {
			fetcher.logger.Errorf("Config fetch failed: %s.", responseError.Error())
			result.complete(fetchResponse{status: Failure})
			return
		}

		defer response.Body.Close()

		if response.StatusCode == 304 {
			fetcher.logger.Debugln("Config fetch succeeded: not modified.")
			result.complete(fetchResponse{status: NotModified})
			return
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				fetcher.logger.Errorf("Config fetch failed: %v", err)
				result.complete(fetchResponse{status: Failure})
				return
			}
			config, err := parseConfig(body)
			if err != nil {
				fetcher.logger.Errorf("Config fetch returned invalid body: %v", err)
				result.complete(fetchResponse{status: Failure})
				return
			}

			fetcher.logger.Debugln("Config fetch succeeded: new config fetched.")
			fetcher.eTag = response.Header.Get("Etag")
			result.complete(fetchResponse{status: Fetched, config: config})
			return
		}

		fetcher.logger.Errorf("Double-check your SDK KEY at https://app.configcat.com/sdkkey. "+
			"Received unexpected response: %v.", response.StatusCode)
		result.complete(fetchResponse{status: Failure})
	}()

	return result
}
