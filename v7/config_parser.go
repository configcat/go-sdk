package configcat

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"
)

type parseError struct {
	msg string
}

func (p *parseError) Error() string {
	return p.msg
}

type config struct {
	jsonBody   []byte
	etag       string
	root       *rootNode
	evaluators sync.Map // reflect.Type -> map[string]entryEvalFunc
	allKeys    []string
	keyValues  map[string]keyValue
	fetchTime  time.Time
}

func parseConfig(jsonBody []byte, etag string, fetchTime time.Time) (*config, error) {
	var root rootNode
	if err := json.Unmarshal([]byte(jsonBody), &root); err != nil {
		return nil, err
	}
	return &config{
		jsonBody:  jsonBody,
		root:      &root,
		keyValues: keyValuesForRootNode(&root),
		allKeys:   keysForRootNode(&root),
		etag:      etag,
		fetchTime: fetchTime,
	}, nil
}

func (c *config) equal(c1 *config) bool {
	if c == c1 || c == nil || c1 == nil {
		return c == c1
	}
	return c.fetchTime.Equal(c1.fetchTime) && c.etag == c1.etag && bytes.Equal(c.jsonBody, c1.jsonBody)
}

func (c *config) equalContent(c1 *config) bool {
	if c == c1 || c == nil || c1 == nil {
		return c == c1
	}
	return bytes.Equal(c.jsonBody, c1.jsonBody)
}

func (c *config) withFetchTime(t time.Time) *config {
	c1 := *c
	c1.fetchTime = t
	return &c1
}

func (c *config) body() string {
	if c == nil {
		return ""
	}
	return string(c.jsonBody)
}

func (conf *config) getKeyAndValueForVariation(variationID string) (string, interface{}) {
	kv := conf.keyValues[variationID]
	return kv.key, kv.value
}

func (conf *config) keys() []string {
	if conf == nil {
		return nil
	}
	return conf.allKeys
}

type rootNode struct {
	Entries     map[string]*entry `json:"f"`
	Preferences *preferences      `json:"p"`
}

type entry struct {
	VariationID     string           `json:"i"`
	Value           interface{}      `json:"v"`
	RolloutRules    []*rolloutRule   `json:"r"`
	PercentageRules []percentageRule `json:"p"`
}

type rolloutRule struct {
	VariationID         string      `json:"i"`
	Value               interface{} `json:"v"`
	ComparisonAttribute string      `json:"a"`
	ComparisonValue     string      `json:"c"`
	Comparator          operator    `json:"t"`
}

type percentageRule struct {
	VariationID string      `json:"i"`
	Value       interface{} `json:"v"`
	Percentage  int64       `json:"p"`
}

type preferences struct {
	URL      string           `json:"u"`
	Redirect *redirectionKind `json:"r"` // NoRedirect, ShouldRedirect or ForceRedirect
}

type redirectionKind int

const (
	// noRedirect indicates that the configuration is available
	// in this request, but that the next request should be
	// made to the redirected address.
	noRedirect redirectionKind = 0

	// shouldRedirect indicates that there is no configuration
	// available at this address, and that the client should
	// redirect immediately. This does not take effect when
	// talking to a custom URL.
	shouldRedirect redirectionKind = 1

	// forceRedirect indicates that there is no configuration
	// available at this address, and that the client should redirect
	// immediately even when talking to a custom URL.
	forceRedirect redirectionKind = 2
)
