package configcat

import (
	"io/ioutil"
	"net/http"
)

const ConfigJsonName = "config_v5"

type configProvider interface {
	getConfigurationAsync() *asyncResult
}

type configFetcher struct {
	sdkKey, eTag, mode, baseUrl string
	urlIsCustom                 bool
	parser                      *configParser
	client                      *http.Client
	logger                      Logger
}

func newConfigFetcher(sdkKey string, config ClientConfig, parser *configParser) *configFetcher {
	fetcher := &configFetcher{sdkKey: sdkKey,
		mode:   config.Mode.getModeIdentifier(),
		parser: parser,
		logger: config.Logger,
		client: &http.Client{Timeout: config.HttpTimeout, Transport: config.Transport}}

	if len(config.BaseUrl) == 0 {
		fetcher.urlIsCustom = false
		fetcher.baseUrl = func() string {
			if config.DataGovernance == Global {
				return globalBaseUrl
			} else {
				return euOnlyBaseUrl
			}
		}()
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

		rootNode, err := fetcher.parser.deserialize(fetchResponse.body)
		if err != nil {
			return asCompletedAsyncResult(fetchResponse)
		}

		preferences, ok := rootNode[preferences].(map[string]interface{})
		if !ok {
			return asCompletedAsyncResult(fetchResponse)
		}

		newUrl, ok := preferences[preferencesUrl].(string)
		if !ok || len(newUrl) == 0 || newUrl == fetcher.baseUrl {
			return asCompletedAsyncResult(fetchResponse)
		}

		redirect, ok := preferences[preferencesRedirect].(float64)
		if !ok {
			return asCompletedAsyncResult(fetchResponse)
		}

		if fetcher.urlIsCustom && redirect != 2 {
			return asCompletedAsyncResult(fetchResponse)
		}

		fetcher.baseUrl = newUrl
		if redirect == 0 {
			return asCompletedAsyncResult(fetchResponse)
		} else {
			if redirect == 1 {
				fetcher.logger.Warnln("Your config.DataGovernance parameter at ConfigCatClient " +
					"initialization is not in sync with your preferences on the ConfigCat " +
					"Dashboard: https://app.configcat.com/organization/data-governance. " +
					"Only Organization Admins can access this preference.")
			}

			if executionCount > 0 {
				return fetcher.executeFetchAsync(executionCount - 1)
			}
		}

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

		fetcher.logger.Errorf("Double-check your SDK KEY at https://app.configcat.com/sdkkey. "+
			"Received unexpected response: %v.", response.StatusCode)
		result.complete(fetchResponse{status: Failure})
	}()

	return result
}
