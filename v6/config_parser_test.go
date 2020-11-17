package configcat

import (
	"testing"
)

func TestConfigParser_Parse(t *testing.T) {
	jsonBody := `{ "f": { "keyDouble": { "v": 120.121238476, "p": [], "r": [], "i":"" }}}`
	config := mustParseConfig(jsonBody)
	val, _, err := config.getValueAndVariationId(DefaultLogger(LogLevelWarn), "keyDouble", nil)

	if err != nil || val != 120.121238476 {
		t.Error("Expecting 120.121238476 as interface")
	}
}

func TestConfigParser_BadJson(t *testing.T) {
	jsonBody := ""
	_, err := parseConfig([]byte(jsonBody))
	if err == nil {
		t.Error("Expecting JSON error")
	}
	t.Log(err)
}

func TestConfigParser_WrongKey(t *testing.T) {
	jsonBody := `{ "keyDouble": { "Value": 120.121238476, "SettingType": 0, "RolloutPercentageItems": [], "RolloutRules": [] }}`
	config := mustParseConfig(jsonBody)
	_, _, err := config.getValueAndVariationId(DefaultLogger(LogLevelWarn), "wrongKey", nil)
	if err == nil {
		t.Error("Expecting key not found error")
	}

	t.Log(err)
}

func TestConfigParser_EmptyNode(t *testing.T) {
	jsonBody := `{ "keyDouble": { }}`
	config := mustParseConfig(jsonBody)
	_, _, err := config.getValueAndVariationId(DefaultLogger(LogLevelWarn), "keyDouble", nil)
	if err == nil {
		t.Error("Expecting key not found error")
	}
	t.Log(err)
}
