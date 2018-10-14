package configcat

import (
	"encoding/json"
)

// Describing JSON parsing related errors
type ParseError struct {
	msg string
}

// The error message
func (p *ParseError) Error() string {
	return p.msg
}

// Describing a JSON configuration parser
type ConfigParser struct {
	evaluator rolloutEvaluator
}

func newParser() *ConfigParser {
	return &ConfigParser{evaluator: rolloutEvaluator{}}
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

func (parser *ConfigParser) parse(jsonBody string, key string, user *User) (interface{}, error) {
	if len(key) == 0 {
		panic("Key cannot be empty")
	}

	var root interface{}
	err := json.Unmarshal([]byte(jsonBody), &root)
	if err != nil {
		return nil, err
	}

	rootNode, ok := root.(map[string]interface{})
	if !ok {
		return nil, &ParseError{"JSON parsing failed, json: " + jsonBody}
	}

	node := rootNode[key]
	if node == nil {
		return nil, &ParseError{"Key not found in json: " + key + ", json: " + jsonBody}
	}

	evaluated := parser.evaluator.evaluate(node, key, user)
	if evaluated == nil {
		return nil, &ParseError{"JSON parsing failed for (key: " + key + "), json: " + jsonBody}
	}

	return evaluated, nil
}
