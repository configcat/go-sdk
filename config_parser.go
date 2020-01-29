package configcat

import (
	"encoding/json"
	"strings"
)

// ParseError describes JSON parsing related errors.
type ParseError struct {
	msg string
}

// Error is the error message.
func (p *ParseError) Error() string {
	return p.msg
}

// ConfigParser describes a JSON configuration parser.
type ConfigParser struct {
	evaluator *rolloutEvaluator
	logger    Logger
}

func newParser(logger Logger) *ConfigParser {
	evaluator := newRolloutEvaluator(logger)
	return &ConfigParser{evaluator: evaluator, logger: logger}
}

// Parse converts a json element identified by a key from the given json string into an interface{} value.
func (parser *ConfigParser) Parse(jsonBody string, key string) (interface{}, error) {
	return parser.ParseWithUser(jsonBody, key, nil)
}

// ParseWithUser converts a json element identified by the key from the given json
// string into an interface{} value. Optional user argument can be passed to identify the caller.
func (parser *ConfigParser) ParseWithUser(jsonBody string, key string, user *User) (interface{}, error) {
	return parser.parse(jsonBody, key, user)
}

// GetAllKeys retrieves all the setting keys from the given json config.
func (parser *ConfigParser) GetAllKeys(jsonBody string) ([]string, error) {
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

func (parser *ConfigParser) parse(jsonBody string, key string, user *User) (interface{}, error) {
	if len(key) == 0 {
		panic("Key cannot be empty")
	}

	rootNode, err := parser.deserialize(jsonBody)
	if err != nil {
		return nil, &ParseError{"JSON parsing failed. " + err.Error() + "."}
	}

	node := rootNode[key]
	if node == nil {
		keys := make([]string, len(rootNode))
		i := 0
		for k := range rootNode {
			keys[i] = k
			i++
		}

		return nil, &ParseError{"Value not found for key " + key +
			". Here are the available keys: " + strings.Join(keys, ", ")}
	}

	parsed := parser.evaluator.evaluate(node, key, user)
	if parsed == nil {
		return nil, &ParseError{"Null evaluated for key " + key + "."}
	}

	return parsed, nil
}

func (parser *ConfigParser) deserialize(jsonBody string) (map[string]interface{}, error) {
	var root interface{}
	err := json.Unmarshal([]byte(jsonBody), &root)
	if err != nil {
		return nil, err
	}

	rootNode, ok := root.(map[string]interface{})
	if !ok {
		return nil, &ParseError{"JSON mapping failed, json: " + jsonBody}
	}

	return rootNode, nil
}
