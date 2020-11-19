package configcat

import (
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestClient_RefreshAsync(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", "value"))

	resultc := make(chan interface{})
	client.RefreshAsync(func() {
		resultc <- client.GetValue("key", "default")
	})
	c.Assert(waitChan(t, resultc), qt.Equals, "value")

	srv.setResponseJSON(rootNodeWithKeyValue("key", "value2"))

	resultc2 := make(chan interface{})
	client.RefreshAsync(func() {
		resultc2 <- client.GetValue("key", "default")
	})
	c.Assert(waitChan(t, resultc2), qt.Equals, "value2")
}

func TestClient_GetAsync(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213))
	client.Refresh()
	resultc := make(chan interface{}, 1)
	client.GetValueAsync("key", 0, func(result interface{}) {
		resultc <- result
	})
	c.Assert(waitChan(t, resultc), qt.Equals, 3213.0)
}

func TestClient_GetAsync_Default(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})
	resultc := make(chan interface{}, 1)
	client.GetValueAsync("key", 0, func(result interface{}) {
		resultc <- result
	})
	c.Assert(waitChan(t, resultc), qt.Equals, 0)
}

func TestClient_GetAsync_Latest(t *testing.T) {
	c := qt.New(t)
	srv, client := getTestClients(t)
	srv.setResponseJSON(rootNodeWithKeyValue("key", 3213))
	resultc := make(chan interface{}, 1)
	client.GetValueAsync("key", 0, func(result interface{}) {
		resultc <- result
	})

	c.Assert(waitChan(t, resultc), qt.Equals, 0)

	srv.setResponse(configResponse{
		status: http.StatusInternalServerError,
		body:   `something failed`,
	})

	resultc2 := make(chan interface{}, 1)
	client.GetValueAsync("key", 0, func(result interface{}) {
		resultc2 <- result
	})
	c.Assert(waitChan(t, resultc2), qt.Equals, 0)
}

func waitChan(t *testing.T, c <-chan interface{}) interface{} {
	t.Helper()
	select {
	case v := <-c:
		return v
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for result")
	}
	panic("unreachable")
}
