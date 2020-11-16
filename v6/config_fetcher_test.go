package configcat

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

const jsonTemplate = `{ "p": { "u": "%s", "r": %d }, "f": {} }`
const customCdnUrl = "https://custom-cdn.configcat.com"

func TestConfigFetcher_GetConfigurationJson(t *testing.T) {
	fetcher := newConfigFetcher("PKDVCLf-Hq-h-kCzMp-L7Q/PaDVCFk9EpmD6sLpGLltTA",
		defaultConfig())
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

func TestConfigFetcher_ShouldStayOnGivenUrl(t *testing.T) {
	// Arrange
	body := fmt.Sprintf(jsonTemplate, "https://fakeUrl", 0)
	transport := newMockHttpTransport()
	transport.enqueue(200, body)

	fetcher := createFetcher(transport, "")

	// Act
	result := fetcher.getConfigurationAsync().get().(fetchResponse).config

	// Assert
	if body != result.body() {
		t.Error("same result expected")
	}

	if len(transport.requests) != 1 {
		t.Error("1 request expected")
	}

	if !strings.Contains(globalBaseUrl, transport.requests[0].Host) {
		t.Error(globalBaseUrl + " does not contain " + transport.requests[0].Host)
	}
}

func TestConfigFetcher_ShouldStayOnSameUrlWithRedirect(t *testing.T) {
	// Arrange
	body := fmt.Sprintf(jsonTemplate, globalBaseUrl, 1)
	transport := newMockHttpTransport()
	transport.enqueue(200, body)

	fetcher := createFetcher(transport, "")

	// Act
	result := fetcher.getConfigurationAsync().get().(fetchResponse).config

	// Assert
	if body != result.body() {
		t.Error("same result expected")
	}

	if len(transport.requests) != 1 {
		t.Error("1 request expected")
	}

	if !strings.Contains(globalBaseUrl, transport.requests[0].Host) {
		t.Error(globalBaseUrl + " does not contain " + transport.requests[0].Host)
	}
}

func TestConfigFetcher_ShouldStayOnSameUrlEvenWhenForced(t *testing.T) {
	// Arrange
	body := fmt.Sprintf(jsonTemplate, globalBaseUrl, 2)
	transport := newMockHttpTransport()
	transport.enqueue(200, body)

	fetcher := createFetcher(transport, "")

	// Act
	result := fetcher.getConfigurationAsync().get().(fetchResponse).config

	// Assert
	if body != result.body() {
		t.Error("same result expected")
	}

	if len(transport.requests) != 1 {
		t.Error("1 request expected")
	}

	if !strings.Contains(globalBaseUrl, transport.requests[0].Host) {
		t.Error(globalBaseUrl + " does not contain " + transport.requests[0].Host)
	}
}

func TestConfigFetcher_ShouldRedirectToAnotherServer(t *testing.T) {
	// Arrange
	body1 := fmt.Sprintf(jsonTemplate, euOnlyBaseUrl, 1)
	body2 := fmt.Sprintf(jsonTemplate, euOnlyBaseUrl, 0)
	transport := newMockHttpTransport()
	transport.enqueue(200, body1)
	transport.enqueue(200, body2)

	fetcher := createFetcher(transport, "")

	// Act
	result := fetcher.getConfigurationAsync().get().(fetchResponse).config

	// Assert
	if body2 != result.body() {
		t.Error("same result expected")
	}

	if len(transport.requests) != 2 {
		t.Error("2 request expected")
	}

	if !strings.Contains(globalBaseUrl, transport.requests[0].Host) {
		t.Error(globalBaseUrl + " does not contain " + transport.requests[0].Host)
	}

	if !strings.Contains(euOnlyBaseUrl, transport.requests[1].Host) {
		t.Error(euOnlyBaseUrl + " does not contain " + transport.requests[1].Host)
	}
}

func TestConfigFetcher_ShouldRedirectToAnotherServerWhenForced(t *testing.T) {
	// Arrange
	body1 := fmt.Sprintf(jsonTemplate, euOnlyBaseUrl, 2)
	body2 := fmt.Sprintf(jsonTemplate, euOnlyBaseUrl, 0)
	transport := newMockHttpTransport()
	transport.enqueue(200, body1)
	transport.enqueue(200, body2)

	fetcher := createFetcher(transport, "")

	// Act
	result := fetcher.getConfigurationAsync().get().(fetchResponse).config

	// Assert
	if body2 != result.body() {
		t.Error("same result expected")
	}

	if len(transport.requests) != 2 {
		t.Error("2 request expected")
	}

	if !strings.Contains(globalBaseUrl, transport.requests[0].Host) {
		t.Error(globalBaseUrl + " does not contain " + transport.requests[0].Host)
	}

	if !strings.Contains(euOnlyBaseUrl, transport.requests[1].Host) {
		t.Error(euOnlyBaseUrl + " does not contain " + transport.requests[1].Host)
	}
}

func TestConfigFetcher_ShouldBreakRedirectLoop(t *testing.T) {
	// Arrange
	body1 := fmt.Sprintf(jsonTemplate, euOnlyBaseUrl, 1)
	body2 := fmt.Sprintf(jsonTemplate, globalBaseUrl, 1)
	transport := newMockHttpTransport()
	transport.enqueue(200, body1)
	transport.enqueue(200, body2)
	transport.enqueue(200, body1)

	fetcher := createFetcher(transport, "")

	// Act
	result := fetcher.getConfigurationAsync().get().(fetchResponse).config

	// Assert
	if body1 != result.body() {
		t.Error("same result expected")
	}

	if len(transport.requests) != 3 {
		t.Error("3 request expected")
	}

	if !strings.Contains(globalBaseUrl, transport.requests[0].Host) {
		t.Error(globalBaseUrl + " does not contain " + transport.requests[0].Host)
	}

	if !strings.Contains(euOnlyBaseUrl, transport.requests[1].Host) {
		t.Error(euOnlyBaseUrl + " does not contain " + transport.requests[1].Host)
	}

	if !strings.Contains(globalBaseUrl, transport.requests[2].Host) {
		t.Error(globalBaseUrl + " does not contain " + transport.requests[2].Host)
	}
}

func TestConfigFetcher_ShouldRespectCustomUrlWhenNotForced(t *testing.T) {
	// Arrange
	body := fmt.Sprintf(jsonTemplate, globalBaseUrl, 1)
	transport := newMockHttpTransport()
	transport.enqueue(200, body)

	fetcher := createFetcher(transport, customCdnUrl)

	// Act
	result := fetcher.getConfigurationAsync().get().(fetchResponse).config

	// Assert
	if body != result.body() {
		t.Error("same result expected")
	}

	if len(transport.requests) != 1 {
		t.Error("1 request expected")
	}

	if !strings.Contains(customCdnUrl, transport.requests[0].Host) {
		t.Error(customCdnUrl + " does not contain " + transport.requests[0].Host)
	}
}

func TestConfigFetcher_ShouldNotRespectCustomUrlWhenForced(t *testing.T) {
	// Arrange
	body1 := fmt.Sprintf(jsonTemplate, globalBaseUrl, 2)
	body2 := fmt.Sprintf(jsonTemplate, globalBaseUrl, 0)
	transport := newMockHttpTransport()
	transport.enqueue(200, body1)
	transport.enqueue(200, body2)

	fetcher := createFetcher(transport, customCdnUrl)

	// Act
	result := fetcher.getConfigurationAsync().get().(fetchResponse).config

	// Assert
	if body2 != result.body() {
		t.Error("same result expected")
	}

	if len(transport.requests) != 2 {
		t.Error("1 request expected")
	}

	if !strings.Contains(customCdnUrl, transport.requests[0].Host) {
		t.Error(customCdnUrl + " does not contain " + transport.requests[0].Host)
	}

	if !strings.Contains(globalBaseUrl, transport.requests[1].Host) {
		t.Error(globalBaseUrl + " does not contain " + transport.requests[1].Host)
	}
}

func createFetcher(transport http.RoundTripper, url string) *configFetcher {
	config := defaultConfig()
	config.BaseUrl = url
	config.Transport = transport
	return newConfigFetcher("fakeKey", config)
}

type mockHttpTransport struct {
	requests  []*http.Request
	responses []*http.Response
}

func newMockHttpTransport() *mockHttpTransport {
	return &mockHttpTransport{}
}

func (m *mockHttpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)

	nextResponseInQueue := m.responses[0]
	m.responses = m.responses[1:]
	return nextResponseInQueue, nil
}

func (m *mockHttpTransport) enqueue(statusCode int, body string) {
	m.responses = append(m.responses, &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(bytes.NewBufferString(body)),
	})
}
