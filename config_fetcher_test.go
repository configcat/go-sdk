package configcat

import (
	"testing"
)

func TestConfigFetcher_GetConfigurationJson(t *testing.T) {
	fetcher := newConfigFetcher("PKDVCLf-Hq-h-kCzMp-L7Q/PaDVCFk9EpmD6sLpGLltTA", DefaultClientConfig())
	response := fetcher.GetConfigurationAsync().Get().(FetchResponse)

	if !response.IsFetched() {
		t.Error("Expecting fetched")
	}

	response2 := fetcher.GetConfigurationAsync().Get().(FetchResponse)

	if !response2.IsNotModified() {
		t.Error("Expecting not modified")
	}
}

func TestConfigFetcher_GetConfigurationJson_Fail(t *testing.T) {
	fetcher := newConfigFetcher("thisshouldnotexist", DefaultClientConfig())
	response := fetcher.GetConfigurationAsync().Get().(FetchResponse)

	if !response.IsFailed() {
		t.Error("Expecting failed")
	}
}
