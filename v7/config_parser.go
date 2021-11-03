package configcat

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"
)

type config struct {
	jsonBody []byte
	etag     string
	root     *rootNode
	// Note: this is a pointer because the configuration
	// can be copied (with the withFetchTime method).
	evaluators *sync.Map // reflect.Type -> map[string]entryEvalFunc
	allKeys    []string
	keyValues  map[string]keyValue
	fetchTime  time.Time
}

func parseConfig(jsonBody []byte, etag string, fetchTime time.Time) (*config, error) {
	var root rootNode
	if err := json.Unmarshal([]byte(jsonBody), &root); err != nil {
		return nil, err
	}
	fixupRootNodeValues(&root)
	return newConfig(&root, jsonBody, etag, fetchTime), nil
}

func newConfig(root *rootNode, jsonBody []byte, etag string, fetchTime time.Time) *config {
	return &config{
		jsonBody:   jsonBody,
		root:       root,
		evaluators: new(sync.Map),
		keyValues:  keyValuesForRootNode(root),
		allKeys:    keysForRootNode(root),
		etag:       etag,
		fetchTime:  fetchTime,
	}
}

func (conf *config) equal(c1 *config) bool {
	if conf == c1 || conf == nil || c1 == nil {
		return conf == c1
	}
	return conf.fetchTime.Equal(c1.fetchTime) && conf.etag == c1.etag && bytes.Equal(conf.jsonBody, c1.jsonBody)
}

func (conf *config) equalContent(c1 *config) bool {
	if conf == c1 || conf == nil || c1 == nil {
		return conf == c1
	}
	return bytes.Equal(conf.jsonBody, c1.jsonBody)
}

func (conf *config) withFetchTime(t time.Time) *config {
	c1 := *conf
	c1.fetchTime = t
	return &c1
}

func (conf *config) body() string {
	if conf == nil {
		return ""
	}
	return string(conf.jsonBody)
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

func fixupRootNodeValues(n *rootNode) {
	for _, entry := range n.Entries {
		entry.Value = fixValue(entry.Value, entry.Type)
		for _, rule := range entry.RolloutRules {
			rule.Value = fixValue(rule.Value, entry.Type)
		}
		for _, rule := range entry.PercentageRules {
			rule.Value = fixValue(rule.Value, entry.Type)
		}
	}
}

// fixValue fixes up int-valued entries, which will have the wrong type of value, so
// change them from float64 to int.
func fixValue(v interface{}, typ entryType) interface{} {
	if typ != intEntry {
		return v
	}
	f, ok := v.(float64)
	if !ok {
		// Shouldn't happen, but avoid a panic.
		return v
	}
	return int(f)
}

type rootNode struct {
	Entries     map[string]*entry `json:"f"`
	Preferences *preferences      `json:"p"`
}

type entry struct {
	VariationID     string           `json:"i"`
	Value           interface{}      `json:"v"`
	Type            entryType        `json:"t"`
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

type entryType int

const (
	boolEntry   entryType = 0
	stringEntry entryType = 1
	intEntry    entryType = 2
	floatEntry  entryType = 3
)
