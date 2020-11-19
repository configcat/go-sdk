package configcat

import (
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
	cfg.Mode = AutoPoll(10 * time.Millisecond)
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	srv.setResponse(configResponse{body: `{"test":2}`})
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	time.Sleep(40 * time.Millisecond)
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":2}`)
}

func TestAutoPollingPolicy_FetchFail(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something wrong`,
	})
	cfg := srv.config()
	cfg.Mode = AutoPoll(2 * time.Second)
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	conf := client.getConfig()
	c.Assert(conf, qt.IsNil)
}

func TestAutoPollingPolicy_FetchFailWithCacheFallback(t *testing.T) {
	testPolicy_FetchFailWithCacheFallback(t, AutoPoll(10*time.Millisecond),
		func(client *Client) {},
		func(client *Client) {
			time.Sleep(20 * time.Millisecond)
		},
	)
}

func TestAutoPollingPolicy_DoubleClose(t *testing.T) {
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"test":1}`})
	cfg := srv.config()
	cfg.Mode = AutoPoll(time.Millisecond)
	client := NewCustomClient(srv.sdkKey(), cfg)
	client.Close()
	client.Close()
}

func TestAutoPollingPolicy_WithNotify(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"test":1}`})
	cfg := srv.config()
	notifyc := make(chan struct{})
	cfg.Mode = AutoPollWithChangeListener(
		time.Millisecond,
		func() { notifyc <- struct{}{} },
	)
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()
	select {
	case <-notifyc:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for notification")
	}
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	// Change the content and we should see another notification.
	srv.setResponse(configResponse{body: `{"test":2}`})
	select {
	case <-notifyc:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for notification")
	}
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":2}`)

	// Check that we don't see any more notifications.
	select {
	case <-notifyc:
		t.Fatalf("unexpected notification received")
	case <-time.After(20 * time.Millisecond):
	}
}
