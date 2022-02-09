// Package configcattest provides an HTTP handler that
// can be used to test configcat scenarios in tests.
package configcattest

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/configcat/go-sdk/v7/internal/wireconfig"
)

// Handler is an http.Handler that serves up configcat flags.
// The zero value is OK to use and has no flag configurations.
// Use SetFlags to add or update the set of flags served.
type Handler struct {
	mu       sync.Mutex
	contents map[string][]byte
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(w, "only GET is allowed", http.StatusMethodNotAllowed)
		return
	}
	h.mu.Lock()
	content := h.contents[req.URL.Path]
	h.mu.Unlock()
	if content == nil {
		http.NotFound(w, req)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(content)
}

// SetFlags sets or updates the flags served by the handler for the
// given SDK key. It can be called concurrently with other Handler methods.
//
// Use RandomSDKKey to create a new SDK key.
func (h *Handler) SetFlags(sdkKey string, flags map[string]*Flag) error {
	if sdkKey == "" {
		return fmt.Errorf("empty SDK key passed to configcattest.Handler.SetFlags")
	}
	data, err := makeContent(flags)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.contents == nil {
		h.contents = make(map[string][]byte)
	}
	h.contents["/configuration-files/"+sdkKey+"/config_v5.json"] = data
	return nil
}

func makeContent(flags map[string]*Flag) ([]byte, error) {
	root := &wireconfig.RootNode{
		Entries: make(map[string]*wireconfig.Entry, len(flags)),
	}
	for name, flag := range flags {
		e, err := flag.entry()
		if err != nil {
			return nil, fmt.Errorf("invalid flag %q: %v", name, err)
		}
		root.Entries[name] = e
	}
	data, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal configuration: %v", err)
	}
	return data, nil
}

// RandomSDKKey returns a new randomly generated SDK key
// suitable for passing to SetFlags.
func RandomSDKKey() string {
	var k sdkKey
	if _, err := rand.Read(k.org[:]); err != nil {
		panic(err)
	}
	if _, err := rand.Read(k.product[:]); err != nil {
		panic(err)
	}
	return k.String()
}

type sdkKey struct {
	org, product [16]byte
}

func (k sdkKey) String() string {
	enc := base64.RawURLEncoding
	n := enc.EncodedLen(len(k.org))
	b := make([]byte, n*2+1)
	enc.Encode(b, k.org[:])
	b[n] = '/'
	enc.Encode(b[n+1:], k.product[:])
	return string(b)
}
