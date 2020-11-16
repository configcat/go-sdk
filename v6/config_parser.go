package configcat

import (
	"encoding/json"
	"strings"
)

type parseError struct {
	msg string
}

func (p *parseError) Error() string {
	return p.msg
}

type configParser struct {
	evaluator *rolloutEvaluator
	logger    Logger
}

func newParser(logger Logger) *configParser {
	evaluator := newRolloutEvaluator(logger)
	return &configParser{evaluator: evaluator, logger: logger}
}

func (parser *configParser) parse(jsonBody string, key string, user *User) (interface{}, error) {
	result, _, err := parser.parseInternal(jsonBody, key, user)
	return result, err
}

func (parser *configParser) parseVariationId(jsonBody string, key string, user *User) (string, error) {
	_, variationId, err := parser.parseInternal(jsonBody, key, user)
	return variationId, err
}

func (parser *configParser) getAllKeys(jsonBody string) ([]string, error) {
	entries, err := parser.getEntries(jsonBody)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(entries))
	i := 0
	for k := range entries {
		keys[i] = k
		i++
	}

	return keys, nil
}

func (parser *configParser) parseKeyValue(jsonBody string, variationId string) (string, interface{}, error) {
	entries, err := parser.getEntries(jsonBody)
	if err != nil {
		return "", nil, &parseError{"JSON parsing failed. " + err.Error() + "."}
	}

	for key, entry := range entries {
		if entry.VariationID == variationId {
			return key, entry.Value, nil
		}
		for _, rule := range entry.RolloutRules {
			if rule.VariationID == variationId {
				return key, rule.Value, nil
			}
		}
		for _, rule := range entry.PercentageRules {
			if rule.VariationID == variationId {
				return key, rule.Value, nil
			}
		}
	}

	return "", nil, &parseError{"JSON parsing failed."}
}

func (parser *configParser) parseInternal(jsonBody string, key string, user *User) (interface{}, string, error) {
	if len(key) == 0 {
		panic("Key cannot be empty")
	}

	entries, err := parser.getEntries(jsonBody)
	if err != nil {
		return nil, "", &parseError{"JSON parsing failed. " + err.Error() + "."}
	}

	entryNode := entries[key]
	if entryNode == nil {
		keys := make([]string, len(entries))
		i := 0
		for k := range entries {
			keys[i] = k
			i++
		}

		return nil, "", &parseError{"Value not found for key " + key +
			". Here are the available keys: " + strings.Join(keys, ", ")}
	}

	parsed, variationId := parser.evaluator.evaluate(entryNode, key, user)
	if parsed == nil {
		return nil, "", &parseError{"Null evaluated for key " + key + "."}
	}

	return parsed, variationId, nil
}

func (parser *configParser) getEntries(jsonBody string) (map[string]*entry, error) {
	root, err := parser.deserialize(jsonBody)
	if err != nil {
		return nil, err
	}
	return root.Entries, nil
}

func (parser *configParser) deserialize(jsonBody string) (*rootNode, error) {
	var root rootNode
	if err := json.Unmarshal([]byte(jsonBody), &root); err != nil {
		return nil, err
	}
	return &root, nil
}

type rootNode struct {
	Entries     map[string]*entry `json:"f"`
	Preferences *preferences      `json:"p"`
}

type entry struct {
	VariationID     string           `json:"i"`
	Value           interface{}      `json:"v"`
	RolloutRules    []rolloutRule    `json:"r"`
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
