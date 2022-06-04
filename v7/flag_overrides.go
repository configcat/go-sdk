package configcat

import (
	"bytes"
	"encoding/json"
	"github.com/configcat/go-sdk/v7/internal/wireconfig"
	"io"
	"io/ioutil"
)

// OverrideBehaviour describes how the overrides should behave.
type OverrideBehaviour int

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
	// Behaviour describes how the overrides should behave. Default is LocalOnly.
	Behaviour OverrideBehaviour

	// Values is a map that contains the overrides.
	Values map[string]interface{}

	// FilePath is the path to a JSON file that contains the overrides.
	FilePath string

	entries map[string]*wireconfig.Entry
}

func (f *FlagOverrides) isValid() bool {
	return f.Values != nil || f.FilePath != ""
}

func (f *FlagOverrides) preLoad(logger *leveledLogger) {
	if f.Behaviour != LocalOnly && f.Behaviour != LocalOverRemote && f.Behaviour != RemoteOverLocal {
		logger.Errorf("flag overrides behaviour configuration is invalid. 'Behavior' is %v.", f.Behaviour)
		return
	}
	if !f.isValid() {
		logger.Errorf("flag overrides configuration is invalid. 'Values' or 'FilePath' must be set.")
		return
	}

	f.entries = f.loadEntries(logger)
	f.fixEntries()
}

func (f *FlagOverrides) loadEntries(logger *leveledLogger) map[string]*wireconfig.Entry {
	if f.Values != nil {
		entries := make(map[string]*wireconfig.Entry, len(f.Values))
		for key, value := range f.Values {
			entries[key] = &wireconfig.Entry{
				Value:       value,
				VariationID: "",
			}
		}
		return entries
	}
	return f.loadEntriesFromFile(logger)
}

func (f *FlagOverrides) loadEntriesFromFile(logger *leveledLogger) map[string]*wireconfig.Entry {
	data, err := ioutil.ReadFile(f.FilePath)
	if err != nil {
		logger.Errorf("unable to read local JSON file: %v", err)
		return nil
	}
	var simplified wireconfig.SimplifiedConfig
	reader := bytes.NewReader(data)
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(&simplified); err == nil && simplified.Flags != nil {
		entries := make(map[string]*wireconfig.Entry, len(simplified.Flags))
		for key, value := range simplified.Flags {
			entries[key] = &wireconfig.Entry{
				Value:       value,
				VariationID: "",
			}
		}
		return entries
	}
	var root wireconfig.RootNode
	if _, err = reader.Seek(0, io.SeekStart); err != nil {
		logger.Errorf("error during reading local JSON file: %v", err)
		return nil
	}
	if err := decoder.Decode(&root); err != nil {
		logger.Errorf("error during reading local JSON file: %v", err)
		return nil
	}
	return root.Entries
}

func (f *FlagOverrides) fixEntries() {
	if f.entries == nil {
		return
	}
	for _, entry := range f.entries {
		if b, ok := entry.Value.(bool); ok {
			entry.Value = b
			entry.Type = wireconfig.BoolEntry
		} else if s, ok := entry.Value.(string); ok {
			entry.Value = s
			entry.Type = wireconfig.StringEntry
		} else if n, ok := entry.Value.(json.Number); ok {
			if i, err := n.Int64(); err == nil {
				entry.Value = int(i)
				entry.Type = wireconfig.IntEntry
			} else if fl, err := n.Float64(); err == nil {
				entry.Value = fl
				entry.Type = wireconfig.FloatEntry
			}
		}
	}
}
