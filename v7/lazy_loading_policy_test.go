package configcat

import (
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
)

func TestLazyLoadingPolicy_NoAsync(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", "value1", stringEntry))

	cfg := srv.config()
	cfg.RefreshMode = Lazy
	cfg.MaxAge = 50 * time.Millisecond
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.String("key", "", nil), qt.Equals, "value1")

	srv.setResponse(configResponse{
		body:  marshalJSON(rootNodeWithKeyValue("key", "value2", stringEntry)),
		sleep: 40 * time.Millisecond,
	})
	c.Assert(client.String("key", "", nil), qt.Equals, "value1")

	time.Sleep(100 * time.Millisecond)
	c.Assert(client.String("key", "", nil), qt.Equals, "value2")
}

func TestLazyLoadingPolicy_FetchFail(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})

	cfg := srv.config()
	cfg.RefreshMode = Lazy
	cfg.MaxAge = 50 * time.Millisecond
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.fetcher.current(), qt.IsNil)
}

func TestLazyLoadingPolicy_Async(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", "value1", stringEntry))

	cfg := srv.config()
	cfg.RefreshMode = Lazy
	cfg.MaxAge = 50 * time.Millisecond
	cfg.NoWaitForRefresh = true
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(client.String("key", "", nil), qt.Equals, "")
	// Wait for the response to arrive.
	time.Sleep(10 * time.Millisecond)
	c.Assert(client.String("key", "", nil), qt.Equals, "value1")

	srv.setResponse(configResponse{
		body:  marshalJSON(rootNodeWithKeyValue("key", "value2", stringEntry)),
		sleep: 40 * time.Millisecond,
	})
	c.Assert(client.String("key", "", nil), qt.Equals, "value1")

	time.Sleep(100 * time.Millisecond)
	// The config is fetched lazily and takes at least 40ms, so
	// we'll still see the old value.
	c.Assert(client.String("key", "", nil), qt.Equals, "value1")

	time.Sleep(50 * time.Millisecond)
	c.Assert(client.String("key", "", nil), qt.Equals, "value2")
}

func TestLazyLoadingPolicy_NotModified(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		body:  `{"test":1}`,
		sleep: time.Millisecond,
	})

	cfg := srv.config()
	cfg.RefreshMode = Lazy
	cfg.MaxAge = 10 * time.Millisecond
	client := NewCustomClient(cfg)
	defer client.Close()

	c.Assert(string(client.Snapshot(nil).config.jsonBody), qt.Equals, `{"test":1}`)
	time.Sleep(20 * time.Millisecond)

	c.Assert(string(client.Snapshot(nil).config.jsonBody), qt.Equals, `{"test":1}`)

	c.Assert(srv.allResponses(), deepEquals, []configResponse{{
		status: http.StatusOK,
		body:   `{"test":1}`,
		sleep:  time.Millisecond,
	}, {
		status: http.StatusNotModified,
		sleep:  time.Millisecond,
	}})
}

var deepEquals = qt.CmpEquals(cmp.AllowUnexported(configResponse{}))
