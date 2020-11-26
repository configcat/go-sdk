package configcat

import (
	"encoding/json"
	"fmt"
	"time"
)

type parseError struct {
	msg string
}

func (p *parseError) Error() string {
	return p.msg
}

type config struct {
	jsonBody  string
	etag      string
	root      *rootNode
	evaluate  func(logger *leveledLogger, key string, user *User) (interface{}, string, error)
	allKeys   []string
	keyValues map[string]keyValue
	fetchTime time.Time
}

func parseConfig(jsonBody []byte, etag string, fetchTime time.Time) (*config, error) {
	var root rootNode
	if err := json.Unmarshal([]byte(jsonBody), &root); err != nil {
		return nil, err
	}
	return &config{
		jsonBody:  string(jsonBody),
		root:      &root,
		evaluate:  evaluator(&root),
		keyValues: keyValuesForRootNode(&root),
		allKeys:   keysForRootNode(&root),
		etag:      etag,
		fetchTime: fetchTime,
	}, nil
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
	return c.jsonBody
}

func (conf *config) getKeyAndValueForVariation(variationId string) (string, interface{}) {
	kv := conf.keyValues[variationId]
	return kv.key, kv.value
}

func (conf *config) getAllKeys() []string {
	return conf.allKeys
}

func (conf *config) getValueAndVariationId(logger *leveledLogger, key string, user *User) (interface{}, string, error) {
	if conf == nil {
		return nil, "", fmt.Errorf("no configuration available")
	}
	return conf.evaluate(logger, key, user)
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
	URL      string `json:"u"`
	Redirect *int   `json:"r"` // NoRedirect, ShouldRedirect or ForceRedirect
}
