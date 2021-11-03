package configcat

import (
	"crypto/md5"
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
	t   testing.TB

	mu        sync.Mutex
	resp      *configResponse
	responses []configResponse
}

type configResponse struct {
	status int
	body   string
	sleep  time.Duration
}

func newConfigServer(t testing.TB) *configServer {
	var buf [8]byte
	rand.Read(buf[:])
	return newConfigServerWithKey(t, fmt.Sprintf("fake-%x", buf[:]))
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
		SDKKey:  srv.key,
		BaseURL: srv.srv.URL,
		Logger:  newTestLogger(srv.t, LogLevelDebug),
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
	if req.URL.Path != "/configuration-files/"+srv.key+"/"+configJSONName+".json" {
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
	t     testing.TB
	level LogLevel
}

func newTestLogger(t testing.TB, level LogLevel) Logger {
	return &testLogger{
		t:     t,
		level: level,
	}
}

func (log testLogger) GetLevel() LogLevel {
	return log.level
}

func (log testLogger) Debugf(format string, args ...interface{}) {
	log.logf("DEBUG", format, args...)
}

func (log testLogger) Infof(format string, args ...interface{}) {
	log.logf("INFO", format, args...)
}

func (log testLogger) Warnf(format string, args ...interface{}) {
	log.logf("WARN", format, args...)
}

func (log testLogger) Errorf(format string, args ...interface{}) {
	log.logf("ERROR", format, args...)
}

func (log testLogger) Debug(args ...interface{}) {
	log.log("DEBUG", args...)
}

func (log testLogger) Info(args ...interface{}) {
	log.log("INFO", args...)
}

func (log testLogger) Warn(args ...interface{}) {
	log.log("WARN", args...)
}

func (log testLogger) Error(args ...interface{}) {
	log.log("ERROR", args...)
}

func (log testLogger) Debugln(args ...interface{}) {
	log.logln("DEBUG", args...)
}

func (log testLogger) Infoln(args ...interface{}) {
	log.logln("INFO", args...)
}

func (log testLogger) Warnln(args ...interface{}) {
	log.logln("WARN", args...)
}

func (log testLogger) Errorln(args ...interface{}) {
	log.logln("ERROR", args...)
}

func (log testLogger) logf(level string, format string, args ...interface{}) {
	log.t.Logf("%s: %s", level, fmt.Sprintf(format, args...))
}

func (log testLogger) log(level string, args ...interface{}) {
	log.t.Logf("%s: %s", level, fmt.Sprint(args...))
}

func (log testLogger) logln(level string, args ...interface{}) {
	log.t.Logf("%s: %s", level, fmt.Sprintln(args...))
}
