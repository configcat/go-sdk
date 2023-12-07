package configcattest

import (
	configcat "github.com/configcat/go-sdk/v8"
)

// Operator defines a comparison operator.
type Operator configcat.Comparator

func (op Operator) String() string {
	return configcat.Comparator(op).String()
}

const invalidEntry configcat.SettingType = -1

func typeOf(x interface{}) configcat.SettingType {
	switch x.(type) {
	case string:
		return configcat.StringSetting
	case int:
		return configcat.IntSetting
	case float64:
		return configcat.FloatSetting
	case bool:
		return configcat.BoolSetting
	}
	return invalidEntry
}
