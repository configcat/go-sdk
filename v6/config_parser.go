package configcat

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
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
	for key, entry := range conf.root.Entries {
		if entry.VariationID == variationId {
			return key, entry.Value
		}
		for _, rule := range entry.RolloutRules {
			if rule.VariationID == variationId {
				return key, rule.Value
			}
		}
		for _, rule := range entry.PercentageRules {
			if rule.VariationID == variationId {
				return key, rule.Value
			}
		}
	}
	return "", nil
}

func (conf *config) getAllKeys() []string {
	keys := make([]string, 0, len(conf.root.Entries))
	for k := range conf.root.Entries {
		keys = append(keys, k)
	}
	return keys
}

func (conf *config) getValueAndVariationId(logger Logger, key string, user *User) (interface{}, string, error) {
	if len(key) == 0 {
		return nil, "", fmt.Errorf("key cannot be empty")
	}
	if conf == nil {
		return nil, "", fmt.Errorf("no configuration available")
	}
	entryNode := conf.root.Entries[key]
	if entryNode == nil {
		return nil, "", &parseError{
			"Value not found for key " + key +
				". Here are the available keys: " + strings.Join(conf.getAllKeys(), ", "),
		}
	}

	parsed, variationId := conf.evaluate(logger, entryNode, key, user)
	if parsed == nil {
		return nil, "", &parseError{"Null evaluated for key " + key + "."}
	}

	return parsed, variationId, nil
}

func (conf *config) evaluate(logger Logger, node *entry, key string, user *User) (interface{}, string) {
	logger.Infof("Evaluating GetValue(%s).", key)

	if user == nil {
		if len(node.RolloutRules) > 0 || len(node.PercentageRules) > 0 {
			logger.Warnln("Evaluating GetValue(" + key + "). UserObject missing! You should pass a " +
				"UserObject to GetValueForUser() in order to make targeting work properly. " +
				"Read more: https://configcat.com/docs/advanced/user-object.")
		}

		result := node.Value
		logger.Infof("Returning %v.", result)
		return result, node.VariationID
	}

	logger.Infof("User object: %v", user)

	for _, rule := range node.RolloutRules {
		if rule == nil {
			continue
		}
		matched, err := matchRolloutRule(rule, user)
		if err != nil {
			logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => SKIP rule. Validation error: %v",
				rule.ComparisonAttribute,
				user,
				rule.Comparator,
				rule.Value,
				err,
			)
			continue
		}
		if matched {
			logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => match, returning: %v",
				rule.ComparisonAttribute,
				user,
				rule.Comparator,
				rule.ComparisonValue,
				rule.Value,
			)
			return rule.Value, rule.VariationID
		}
		logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => no match",
			rule.ComparisonAttribute,
			user,
			rule.Comparator,
			rule.Value,
		)
	}

	if len(node.PercentageRules) > 0 {
		sum := sha1.Sum([]byte(key + user.identifier))
		// Treat the first 4 bytes as a number, then knock
		// of the last 4 bits. This is equivalent to turning the
		// entire sum into hex, then decoding the first 7 digits.
		num := int64(binary.BigEndian.Uint32(sum[:4]))
		num >>= 4

		scaled := num % 100
		bucket := int64(0)
		for _, rule := range node.PercentageRules {
			bucket += rule.Percentage
			if scaled < bucket {
				result := rule.Value
				logger.Infof("Evaluating %% options. Returning %s", result)
				return result, rule.VariationID
			}
		}
	}

	result := node.Value
	logger.Infof("Returning %v.", result)
	return result, node.VariationID
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
