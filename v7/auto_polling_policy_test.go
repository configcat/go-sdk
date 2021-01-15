package configcat

import (
	"context"
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestAutoPollingPolicy_PollChange(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"test":1}`})

	cfg := srv.config()
	cfg.PollInterval = 10 * time.Millisecond
	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	c.Assert(err, qt.Equals, nil)

	c.Assert(string(client.Snapshot(nil).config.jsonBody), qt.Equals, `{"test":1}`)

	srv.setResponse(configResponse{body: `{"test":2}`})
	c.Assert(string(client.Snapshot(nil).config.jsonBody), qt.Equals, `{"test":1}`)

	time.Sleep(40 * time.Millisecond)
	c.Assert(string(client.Snapshot(nil).config.jsonBody), qt.Equals, `{"test":2}`)
}

func TestAutoPollingPolicy_FetchFail(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something wrong`,
	})
	cfg := srv.config()
	cfg.PollInterval = 2 * time.Second
	client := NewCustomClient(cfg)
	defer client.Close()
	err := client.Refresh(context.Background())
	c.Assert(err, qt.ErrorMatches, `config fetch failed: received unexpected response 500 Internal Server Error`)

	conf := client.Snapshot(nil).config
	c.Assert(conf, qt.IsNil)
}

func TestAutoPollingPolicy_DoubleClose(t *testing.T) {
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"test":1}`})
	cfg := srv.config()
	cfg.PollInterval = time.Millisecond
	client := NewCustomClient(cfg)
	client.Close()
	client.Close()
}

func TestAutoPollingPolicy_WithNotify(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"test":1}`})
	cfg := srv.config()
	notifyc := make(chan struct{})
	cfg.PollInterval = time.Millisecond
	cfg.ChangeNotify = func() { notifyc <- struct{}{} }
	client := NewCustomClient(cfg)
	defer client.Close()
	select {
	case <-notifyc:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for notification")
	}
	c.Assert(string(client.Snapshot(nil).config.jsonBody), qt.Equals, `{"test":1}`)

	// Change the content and we should see another notification.
	srv.setResponse(configResponse{body: `{"test":2}`})
	select {
	case <-notifyc:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for notification")
	}
	c.Assert(string(client.Snapshot(nil).config.jsonBody), qt.Equals, `{"test":2}`)

	// Check that we don't see any more notifications.
	select {
	case <-notifyc:
		t.Fatalf("unexpected notification received")
	case <-time.After(20 * time.Millisecond):
	}
}
