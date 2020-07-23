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
	rootNode, err := parser.deserialize(jsonBody)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(rootNode))
	i := 0
	for k := range rootNode {
		keys[i] = k
		i++
	}

	return keys, nil
}

func (parser *configParser) parseKeyValue(jsonBody string, variationId string) (string, interface{}, error) {
	rootNode, err := parser.deserialize(jsonBody)
	if err != nil {
		return "", nil, &parseError{"JSON parsing failed. " + err.Error() + "."}
	}

	for key, value := range rootNode {
		node := value.(map[string]interface{})
		if node[settingVariationId].(string) == variationId {
			return key, node[settingValue], nil
		}

		rolloutRules := node[settingRolloutRules].([]interface{})
		percentageRules := node[settingRolloutPercentageItems].([]interface{})

		for _, rolloutItem := range rolloutRules {
			rule := rolloutItem.(map[string]interface{})
			if rule[rolloutVariationId].(string) == variationId {
				return key, rule[rolloutValue], nil
			}
		}

		for _, percentageItem := range percentageRules {
			rule := percentageItem.(map[string]interface{})
			if rule[percentageItemVariationId].(string) == variationId {
				return key, rule[percentageItemValue], nil
			}
		}
	}

	return "", nil, &parseError{"JSON parsing failed." }
}

func (parser *configParser) parseInternal(jsonBody string, key string, user *User) (interface{}, string, error) {
	if len(key) == 0 {
		panic("Key cannot be empty")
	}

	rootNode, err := parser.deserialize(jsonBody)
	if err != nil {
		return nil, "", &parseError{"JSON parsing failed. " + err.Error() + "."}
	}

	node := rootNode[key]
	if node == nil {
		keys := make([]string, len(rootNode))
		i := 0
		for k := range rootNode {
			keys[i] = k
			i++
		}

		return nil, "", &parseError{"Value not found for key " + key +
			". Here are the available keys: " + strings.Join(keys, ", ")}
	}

	parsed, variationId := parser.evaluator.evaluate(node, key, user)
	if parsed == nil {
		return nil, "", &parseError{"Null evaluated for key " + key + "."}
	}

	return parsed, variationId, nil
}

func (parser *configParser) deserialize(jsonBody string) (map[string]interface{}, error) {
	var root interface{}
	err := json.Unmarshal([]byte(jsonBody), &root)
	if err != nil {
		return nil, err
	}

	rootNode, ok := root.(map[string]interface{})
	if !ok {
		return nil, &parseError{"JSON mapping failed, json: " + jsonBody}
	}

	return rootNode, nil
}
