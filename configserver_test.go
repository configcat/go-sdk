package configcat

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/configcat/go-sdk/v8/configcatcache"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

type configServer struct {
	srv *httptest.Server
	key string
	t   testing.TB

	mu           sync.Mutex
	resp         *configResponse
	responses    []configResponse
	requestCount int
}

type configResponse struct {
	status int
	body   string
	sleep  time.Duration
}

func newConfigServer(t testing.TB) *configServer {
	var buf [8]byte
	rand.Read(buf[:])
	return newConfigServerWithKey(t, fmt.Sprintf("testing-%x", buf[:]))
}

func newConfigServerWithKey(t testing.TB, sdkKey string) *configServer {
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
func (srv *configServer) config() Config {
	return Config{
		SDKKey:   srv.key,
		BaseURL:  srv.srv.URL,
		Logger:   newTestLogger(srv.t),
		LogLevel: LogLevelDebug,
	}
}

// setResponse sets the response that will be returned from the server.
func (srv *configServer) setResponse(response configResponse) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.resp = &response
}

func (srv *configServer) setResponseJSON(x interface{}) {
	srv.setResponse(configResponse{
		body: marshalJSON(x),
	})
}

// allResponses returns all the responses that have been served over
// the lifetime of the server, excluding those that will have
// caused the test to fail.
func (srv *configServer) allResponses() []configResponse {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return append([]configResponse(nil), srv.responses...)
}

func (srv *configServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/configuration-files/"+srv.key+"/"+configcatcache.ConfigJSONName {
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
	srv.requestCount++
	resp0 := srv.resp
	defer srv.mu.Unlock()
	if resp0 == nil {
		srv.t.Errorf("HTTP call with no response provided")
		http.Error(w, "unexpected call", http.StatusInternalServerError)
		return
	}
	resp := *resp0
	time.Sleep(resp.sleep)
	if resp.status == 0 {
		w.Header().Set("Etag", etagOf(resp.body))
		if req.Header.Get("If-None-Match") == etagOf(resp.body) {
			resp.status = http.StatusNotModified
			resp.body = ""
		} else {
			resp.status = http.StatusOK
		}
	}
	w.WriteHeader(resp.status)
	w.Write([]byte(resp.body))
	// Record the response so that it's possible to check what went on behind the scenes later.
	srv.responses = append(srv.responses, resp)
}

var (
	readIntegrationTestKeysOnce sync.Once
	integrationTestKeyContent   map[string]json.RawMessage
)

func contentForIntegrationTestKey(key string) string {
	readIntegrationTestKeysOnce.Do(func() {
		data, err := os.ReadFile("resources/content-by-key.json")
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

func etagOf(content string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(content)))
}

func marshalJSON(x interface{}) string {
	data, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// testLogger implements the Logger interface by logging to the test.T
// instance.
type testLogger struct {
	sync.RWMutex

	t    testing.TB
	logs []string
}

func newTestLogger(t testing.TB) Logger {
	return &testLogger{
		t: t,
	}
}

func (log *testLogger) Debugf(format string, args ...interface{}) {
	log.Lock()
	defer log.Unlock()
	s := fmt.Sprintf("DEBUG: %s", fmt.Sprintf(format, args...))
	log.logs = append(log.logs, s)
	log.t.Log(s)
}

func (log *testLogger) Infof(format string, args ...interface{}) {
	log.Lock()
	defer log.Unlock()
	s := fmt.Sprintf("INFO: %s", fmt.Sprintf(format, args...))
	log.logs = append(log.logs, s)
	log.t.Log(s)
}

func (log *testLogger) Warnf(format string, args ...interface{}) {
	log.Lock()
	defer log.Unlock()
	s := fmt.Sprintf("WARN: %s", fmt.Sprintf(format, args...))
	log.logs = append(log.logs, s)
	log.t.Log(s)
}

func (log *testLogger) Errorf(format string, args ...interface{}) {
	log.Lock()
	defer log.Unlock()
	s := fmt.Sprintf("ERROR: %s", fmt.Sprintf(format, args...))
	log.logs = append(log.logs, s)
	log.t.Log(s)
}

func (log *testLogger) Logs() []string {
	log.RLock()
	defer log.RUnlock()
	return log.logs
}

func (log *testLogger) Clear() {
	log.Lock()
	defer log.Unlock()
	log.logs = nil
}
