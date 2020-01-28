package configcat

import (
	"io/ioutil"
	"net/http"
)

// configProvider describes a configuration provider which used to collect the actual configuration.
type configProvider interface {
	// getConfigurationAsync collects the actual configuration.
	getConfigurationAsync() *asyncResult
}

// configFetcher used to fetch the actual configuration over HTTP.
type configFetcher struct {
	apiKey, eTag, mode, baseUrl string
	client                      *http.Client
	logger                      Logger
}

func newConfigFetcher(apiKey string, config ClientConfig) *configFetcher {
	return &configFetcher{apiKey: apiKey,
		mode: config.Mode.getModeIdentifier(),
		baseUrl: config.BaseUrl,
		logger:  config.Logger,
		client:  &http.Client{Timeout: config.HttpTimeout, Transport: config.Transport}}
}

// getConfigurationAsync collects the actual configuration over HTTP.
func (fetcher *configFetcher) getConfigurationAsync() *asyncResult {
	result := newAsyncResult()

	go func() {
		request, requestError := http.NewRequest("GET", fetcher.baseUrl+"/configuration-files/"+fetcher.apiKey+"/config_v3.json", nil)
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
			result.complete(fetchResponse{status: Failure, body: ""})
			return
		}

		defer response.Body.Close()

		if response.StatusCode == 304 {
			fetcher.logger.Debugln("Config fetch succeeded: not modified.")
			result.complete(fetchResponse{status: NotModified})
			return
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			body, bodyError := ioutil.ReadAll(response.Body)
			if bodyError != nil {
				fetcher.logger.Errorf("Config fetch failed: %s.", bodyError.Error())
				result.complete(fetchResponse{status: Failure})
				return
			}

			fetcher.logger.Debugln("Config fetch succeeded: new config fetched.")
			fetcher.eTag = response.Header.Get("Etag")
			result.complete(fetchResponse{status: Fetched, body: string(body)})
			return
		}

		fetcher.logger.Errorf("Double-check your API KEY at https://app.configcat.com/apikey. "+
			"Received unexpected response: %v.", response.StatusCode)
		result.complete(fetchResponse{status: Failure})
	}()

	return result
}
