package configcat

import (
	"encoding/json"
	"os"
)

// OverrideBehavior describes how the overrides should behave.
type OverrideBehavior int

const (
	// LocalOnly means that when evaluating values, the SDK will not use feature flags and settings from the
	// ConfigCat CDN, but it will use all feature flags and settings that are loaded from local-override sources.
	LocalOnly = 0

	// LocalOverRemote means that when evaluating values, the SDK will use all feature flags and settings that are
	// downloaded from the ConfigCat CDN, plus all feature flags and settings that are loaded from
	// local-override sources. If a feature flag or a setting is defined both in the fetched and the local-override
	// source then the local-override version will take precedence.
	LocalOverRemote = 1

	// RemoteOverLocal means when evaluating values, the SDK will use all feature flags and settings that are
	// downloaded from the ConfigCat CDN, plus all feature flags and settings that are loaded from local-override
	// sources. If a feature flag or a setting is defined both in the fetched and the local-override source then the
	// fetched version will take precedence.
	RemoteOverLocal = 2
)

// FlagOverrides describes feature flag and setting overrides. With flag overrides you can overwrite the
// feature flags downloaded from the ConfigCat CDN with local values.
//
// With Values, you can set up the SDK to load your feature flag overrides from a map.
//
// With FilePath, you can set up the SDK to load your feature flag overrides from a JSON file.
type FlagOverrides struct {
	// Behavior describes how the overrides should behave. Default is LocalOnly.
	Behavior OverrideBehavior

	// Values is a map that contains the overrides.
	// Each value must be one of the following types: bool, int, float64, or string.
	Values map[string]interface{}

	// FilePath is the path to a JSON file that contains the overrides.
	// The supported JSON file formats are documented here: https://configcat.com/docs/sdk-reference/go/#json-file-structure
	FilePath string

	// settings are populated by loadEntries from the above fields.
	settings map[string]*Setting
}

func (f *FlagOverrides) loadEntries(logger *leveledLogger) {
	if f.Behavior != LocalOnly && f.Behavior != LocalOverRemote && f.Behavior != RemoteOverLocal {
		logger.Errorf(0, "flag overrides behavior configuration is invalid; 'Behavior' is %v", f.Behavior)
		return
	}
	if f.Values == nil && f.FilePath == "" {
		logger.Errorf(0, "flag overrides configuration is invalid; 'Values' or 'FilePath' must be set")
		return
	}
	if f.Values == nil {
		f.loadEntriesFromFile(logger)
	} else {
		f.settings = make(map[string]*Setting, len(f.Values))
		for key, value := range f.Values {
			f.settings[key] = &Setting{
				Value: fromAnyValue(value),
				Type:  getSettingType(value),
			}
		}
	}
}

func (f *FlagOverrides) loadEntriesFromFile(logger *leveledLogger) {
	data, err := os.ReadFile(f.FilePath)
	if err != nil {
		logger.Errorf(1302, "failed to read the local config file '%s': %v", f.FilePath, err)
		return
	}
	// Try the simplified configuration first.
	var simplified SimplifiedConfig
	if err := json.Unmarshal(data, &simplified); err == nil && simplified.Flags != nil {
		f.settings = make(map[string]*Setting, len(simplified.Flags))
		for key, value := range simplified.Flags {
			f.settings[key] = &Setting{
				Value: fromAnyValue(value),
				Type:  getSettingType(value),
			}
		}
		return
	}
	// Fall back to using the full wire configuration.
	var root ConfigJson
	if err := json.Unmarshal(data, &root); err != nil {
		logger.Errorf(2302, "failed to decode JSON from the local config file '%s': %v", f.FilePath, err)
		return
	}
	fixupSegmentsAndSalt(&root)
	f.settings = root.Settings
}

func getSettingType(value interface{}) SettingType {
	switch value.(type) {
	case bool:
		return BoolSetting
	case string:
		return StringSetting
	case float64:
		return FloatSetting
	case int:
		return IntSetting
	default:
		return UnknownSetting
	}
}

func fromAnyValue(v interface{}) *SettingValue {
	if !isValidValue(v) {
		return &SettingValue{Value: nil, invalidValue: v}
	} else {
		return &SettingValue{Value: v}
	}
}

func isValidValue(v interface{}) bool {
	switch v.(type) {
	case bool, string, float64, int:
		return true
	}
	return false
}
