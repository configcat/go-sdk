package configcat

import (
	"context"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestManualPollingPolicy_Refresh(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	// Note: don't set a response on the server - the test will
	// fail if we get a request.

	cfg := srv.config()
	cfg.RefreshMode = Manual
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.fetcher.current(), qt.IsNil)

	srv.setResponse(configResponse{body: `{"test":1}`})
	client.Refresh(context.Background())
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"test":1}`)

	srv.setResponse(configResponse{body: `{"test":2}`})
	c.Assert(client.fetcher.current().body(), qt.Equals, `{"test":1}`)
	client.Refresh(context.Background())
	srv.setResponse(configResponse{body: `{"test":2}`})
}

func TestManualPollingPolicy_FetchFail(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)

	cfg := srv.config()
	cfg.RefreshMode = Manual
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.fetcher.current(), qt.IsNil)

	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})
	client.Refresh(context.Background())
	c.Assert(client.fetcher.current(), qt.IsNil)
}
