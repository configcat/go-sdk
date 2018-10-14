package configcat

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// Describes a configuration provider which used to collect the actual configuration.
type ConfigProvider interface {
	// GetConfigurationAsync collects the actual configuration.
	GetConfigurationAsync() *AsyncResult
}

// A structure used to fetch the actual configuration over HTTP.
type ConfigFetcher struct {
	apiKey, eTag, mode 	string
	client 				*http.Client
	logger 				*log.Logger
}

func newConfigFetcher(apiKey string, config ClientConfig) *ConfigFetcher {
	return &ConfigFetcher{ apiKey:apiKey,
		logger:log.New(os.Stderr, "[ConfigCat - Config Fetcher]", log.LstdFlags),
		client: &http.Client{ Timeout: config.HttpTimeout }}
}

// GetConfigurationAsync collects the actual configuration over HTTP.
func (fetcher *ConfigFetcher) GetConfigurationAsync() *AsyncResult {
	result := NewAsyncResult()

	go func() {
		request, requestError := http.NewRequest("GET", "https://cdn.configcat.com/configuration-files/" + fetcher.apiKey + "/config_v2.json", nil)
		if requestError != nil {
			result.Complete(FetchResponse{ Status:Failure })
		}

		request.Header.Add("X-ConfigCat-UserAgent", "ConfigCat-Go/" + fetcher.mode + "-" + version)

		if fetcher.eTag !=  "" {
			request.Header.Add("If-None-Match", fetcher.eTag)
		}

		response, responseError := fetcher.client.Do(request)
		if responseError != nil {
			fetcher.logger.Printf("Config fetch failed: %s", responseError.Error())
			result.Complete(FetchResponse{Status:Failure, Body:""})
		}

		defer response.Body.Close()

		if response.StatusCode == 304 {
			fetcher.logger.Print("Config fetch succeeded: not modified")
			result.Complete(FetchResponse{ Status:NotModified })
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			body, bodyError := ioutil.ReadAll(response.Body)
			if bodyError == nil {
				fetcher.logger.Print("Config fetch succeeded: new config fetched")
				fetcher.eTag = response.Header.Get("Etag")
				result.Complete(FetchResponse{Status: Fetched, Body: string(body) })
			}
		}

		result.Complete(FetchResponse{ Status:Failure })
	}()

	return result
}
