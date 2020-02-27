package configcat

import (
	"testing"
)

func TestConfigFetcher_GetConfigurationJson(t *testing.T) {

	fetcher := newConfigFetcher("PKDVCLf-Hq-h-kCzMp-L7Q/PaDVCFk9EpmD6sLpGLltTA", defaultConfig())
	response := fetcher.getConfigurationAsync().get().(fetchResponse)

	if !response.isFetched() {
		t.Error("Expecting fetched")
	}

	response2 := fetcher.getConfigurationAsync().get().(fetchResponse)

	if !response2.isNotModified() {
		t.Error("Expecting not modified")
	}
}

func TestConfigFetcher_GetConfigurationJson_Fail(t *testing.T) {
	fetcher := newConfigFetcher("thisshouldnotexist", defaultConfig())
	response := fetcher.getConfigurationAsync().get().(fetchResponse)

	if !response.isFailed() {
		t.Error("Expecting failed")
	}
}
