package configcat

import (
	"encoding/json"
	"io/ioutil"

	"github.com/configcat/go-sdk/v7/internal/wireconfig"
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

	// entries is populated by loadEntries from the above fields.
	entries map[string]*wireconfig.Entry
}

func (f *FlagOverrides) loadEntries(logger *leveledLogger) {
	if f.Behavior != LocalOnly && f.Behavior != LocalOverRemote && f.Behavior != RemoteOverLocal {
		logger.Errorf("flag overrides behavior configuration is invalid. 'Behavior' is %v.", f.Behavior)
		return
	}
	if f.Values == nil && f.FilePath == "" {
		logger.Errorf("flag overrides configuration is invalid. 'Values' or 'FilePath' must be set.")
		return
	}
	if f.Values == nil {
		f.entries = f.loadEntriesFromFile(logger)
		return
	}
	f.entries = make(map[string]*wireconfig.Entry, len(f.Values))
	for key, value := range f.Values {
		f.entries[key] = &wireconfig.Entry{
			Value: value,
		}
	}
	f.fixEntries(logger)
}

func (f *FlagOverrides) loadEntriesFromFile(logger *leveledLogger) map[string]*wireconfig.Entry {
	data, err := ioutil.ReadFile(f.FilePath)
	if err != nil {
		logger.Errorf("unable to read local JSON file: %v", err)
		return nil
	}
	// Try the simplified configuration first.
	var simplified wireconfig.SimplifiedConfig
	if err := json.Unmarshal(data, &simplified); err == nil && simplified.Flags != nil {
		entries := make(map[string]*wireconfig.Entry, len(simplified.Flags))
		for key, value := range simplified.Flags {
			entries[key] = &wireconfig.Entry{
				Value: value,
			}
		}
		return entries
	}
	// Fall back to using the full wire configuration.
	var root wireconfig.RootNode
	if err := json.Unmarshal(data, &root); err != nil {
		logger.Errorf("error reading local JSON file %q: %v", f.FilePath, err)
		return nil
	}
	return root.Entries
}

func (f *FlagOverrides) fixEntries(logger *leveledLogger) {
	for key, entry := range f.entries {
		switch value := entry.Value.(type) {
		case bool:
			entry.Type = wireconfig.BoolEntry
		case string:
			entry.Type = wireconfig.StringEntry
		case float64:
			entry.Type = wireconfig.FloatEntry
		case int:
			entry.Type = wireconfig.IntEntry
		default:
			logger.Errorf("ignoring override value for flag %q with unexpected type %T (%#v); must be bool, int, float64 or string", key, value, value)
			delete(f.entries, key)
		}
	}
}
