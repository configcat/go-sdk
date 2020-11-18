package configcat

import (
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestLazyLoadingPolicy_NoAsync(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"test":1}`})

	cfg := srv.config()
	cfg.Mode = LazyLoad(50*time.Millisecond, false)
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	srv.setResponse(configResponse{
		body:  `{"test":2}`,
		sleep: 40 * time.Millisecond,
	})
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	time.Sleep(100 * time.Millisecond)
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":2}`)
}

func TestLazyLoadingPolicy_FetchFail(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})

	cfg := srv.config()
	cfg.Mode = LazyLoad(50*time.Millisecond, false)
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	c.Assert(client.getConfig(), qt.IsNil)
}

func TestLazyLoadingPolicy_Async(t *testing.T) {
	c := qt.New(t)
	srv := newConfigServer(t)
	srv.setResponse(configResponse{body: `{"test":1}`})

	cfg := srv.config()
	cfg.Mode = LazyLoad(50*time.Millisecond, true)
	client := NewCustomClient(srv.sdkKey(), cfg)
	defer client.Close()

	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	srv.setResponse(configResponse{
		body:  `{"test":2}`,
		sleep: 40 * time.Millisecond,
	})
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	time.Sleep(100 * time.Millisecond)
	// The config is fetched lazily and takes at least 40ms, so
	// we'll still see the old value.
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":1}`)

	time.Sleep(50 * time.Millisecond)
	c.Assert(client.getConfig().body(), qt.Equals, `{"test":2}`)
}
