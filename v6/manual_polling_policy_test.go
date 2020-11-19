package configcat

import (
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestManualPollingPolicy_Refresh(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	// Note: don't set a response on the server - the test will
	// fail if we get a request.

	cfg := srv.config()
	cfg.Mode = ManualPoll()
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	c.Assert(client.getConfig(), qt.IsNil)

	srv.setResponse(configResponse{body: `{"test":1}`})
	client.Refresh()
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	srv.setResponse(configResponse{body: `{"test":2}`})
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)
	client.Refresh()
	srv.setResponse(configResponse{body: `{"test":2}`})
}

func TestManualPollingPolicy_FetchFail(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)

	cfg := srv.config()
	cfg.Mode = ManualPoll()
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	c.Assert(client.getConfig(), qt.IsNil)

	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})
	client.Refresh()
	c.Assert(client.getConfig(), qt.IsNil)
}

func TestManualPollingPolicy_FetchFailWithCacheFallback(t *testing.T) {
	testPolicy_FetchFailWithCacheFallback(t, ManualPoll(),
		func(client *Client) {
			client.Refresh()
		},
		func(client *Client) {
			time.Sleep(60 * time.Millisecond)
		},
	)
}
