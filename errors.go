package configcat

import (
	"fmt"
	"strings"
)

// ErrKeyNotFound is returned when a key is not found in the configuration.
type ErrKeyNotFound struct {
	Key           string
	AvailableKeys []string
}

func (e ErrKeyNotFound) Error() string {
	var availableKeys = ""
	if len(e.AvailableKeys) > 0 {
		availableKeys = "'" + strings.Join(e.AvailableKeys, "', '") + "'"
	}
	return fmt.Sprintf(
		"failed to evaluate setting '%s' (the key was not found in config JSON); available keys: [%s]",
		e.Key,
		availableKeys,
	)
}

// ErrSettingTypeMismatch is returned when a requested setting type doesn't match with the expected type.
type ErrSettingTypeMismatch struct {
	Key          string
	Value        interface{}
	ExpectedType string
}

func (e ErrSettingTypeMismatch) Error() string {
	return fmt.Sprintf(
		"the type of the setting '%s' doesn't match with the expected type; setting's type was '%T' but the expected type was '%s'",
		e.Key,
		e.Value,
		e.ExpectedType,
	)
}

// ErrConfigJsonMissing is returned when the config JSON is empty or missing.
type ErrConfigJsonMissing struct {
	Key string
}

func (e ErrConfigJsonMissing) Error() string {
	return fmt.Sprintf(
		"config JSON is not present when evaluating setting '%s'; returning the `defaultValue` parameter that you specified in your application",
		e.Key,
	)
}
