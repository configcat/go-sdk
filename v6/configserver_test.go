package configcat

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type configServer struct {
	srv *httptest.Server
	key string
	t   *testing.T

	mu   sync.Mutex
	resp *configResponse
}

type configResponse struct {
	status int
	etag   string
	body   string
	sleep  time.Duration
}

func newConfigServer(t *testing.T) *configServer {
	var buf [8]byte
	rand.Read(buf[:])
	return newConfigServerWithKey(t, fmt.Sprintf("fake-%x", buf[:]))
}

func newConfigServerWithKey(t *testing.T, sdkKey string) *configServer {
	srv := &configServer{
		t: t,
	}
	srv.srv = httptest.NewServer(srv)
	t.Cleanup(srv.srv.Close)
	srv.key = sdkKey
	return srv
}

// config returns a configuration suitable for creating
// a client that talks to the p.
func (srv *configServer) config() ClientConfig {
	return ClientConfig{
		BaseUrl: srv.srv.URL,
	}
}

func (srv *configServer) sdkKey() string {
	return srv.key
}

func (srv *configServer) close() {
	srv.srv.Close()
}

func (srv *configServer) setResponse(response configResponse) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.resp = &response
}

func (srv *configServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/configuration-files/"+srv.key+"/"+ConfigJsonName+".json" {
		srv.t.Errorf("unexpected HTTP call: %s %s", req.Method, req.URL)
		http.NotFound(w, req)
		return
	}
	if req.Method != "GET" {
		srv.t.Errorf("unexpected HTTP method: %s", req.Method)
		http.Error(w, "only GET is allowed", http.StatusMethodNotAllowed)
		return
	}
	srv.mu.Lock()
	resp := srv.resp
	srv.mu.Unlock()
	if resp == nil {
		srv.t.Errorf("HTTP call with no response provided")
		http.Error(w, "unexpected call", http.StatusInternalServerError)
		return
	}
	time.Sleep(resp.sleep)
	if resp.etag != "" {
		w.Header().Set("Etag", resp.etag)
	}
	if resp.status != 0 {
		w.WriteHeader(resp.status)
	}
	w.Write([]byte(resp.body))
}

func serverForIntegrationTestKey(t *testing.T, sdkKey string) *configServer {
	srv := newConfigServer(t)
	return srv
}

var (
	readIntegrationTestKeysOnce sync.Once
	integrationTestKeyContent   map[string]json.RawMessage
)

func contentForIntegrationTestKey(key string) string {
	readIntegrationTestKeysOnce.Do(func() {
		data, err := ioutil.ReadFile("../resources/content-by-key.json")
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, &integrationTestKeyContent); err != nil {
			panic(err)
		}
	})
	content, ok := integrationTestKeyContent[key]
	if !ok {
		panic(fmt.Errorf("integration test content for key %q not found", key))
	}
	return string(content)
}
