package configcat

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"
)

type config struct {
	jsonBody []byte
	etag     string
	root     *ConfigJson
	// Note: this is a pointer because the configuration
	// can be copied (with the withFetchTime method).
	evaluators []settingEvalFunc
	allKeys    []string
	keyValues  map[string]keyValue
	fetchTime  time.Time
	userInfos  *sync.Map
	// values holds all the values that can be returned from the
	// configuration, keyed by valueID-1.
	values []interface{}

	// valueIds holds value IDs for keys that we know
	// the values of ahead of time because they're not
	// dependent on the user value, indexed by key id.
	// For values that we don't know ahead of time, the
	// valueIds entry contains a negative number n. (-n - 1) is the index
	// into the cache slice which will hold the value eventually
	// when it's been calculated for a particular user.
	valueIds []valueID

	// keysWithRules holds the number of keys that have
	// rules (i.e. that do not have a value in valueIds).
	keysWithRules int

	// defaultUserSnapshot holds a predefined snapshot of the
	// configuration with the default user.
	defaultUserSnapshot *Snapshot

	// defaultUser holds the user that defaultUserSnapshot was
	// created with.
	defaultUser User
}

// valueID holds an integer representation of a value that
// can be returned from a feature flag. It's one more
// than the index into the config.values or Snapshot.values slice.
type valueID = int32

func parseConfig(jsonBody []byte, etag string, fetchTime time.Time, logger *leveledLogger, defaultUser User, overrides *FlagOverrides, hooks *Hooks) (*config, error) {
	var root ConfigJson
	// Note: jsonBody can be nil when we've got overrides only.
	if jsonBody != nil {
		if err := json.Unmarshal(jsonBody, &root); err != nil {
			return nil, err
		}
	}
	mergeWithOverrides(&root, overrides)
	conf := &config{
		jsonBody:    jsonBody,
		root:        &root,
		keyValues:   keyValuesForRootNode(&root),
		allKeys:     keysForRootNode(&root),
		etag:        etag,
		fetchTime:   fetchTime,
		valueIds:    make([]valueID, numKeys()),
		defaultUser: defaultUser,
		userInfos:   new(sync.Map),
	}
	if conf.root.Preferences != nil {
		conf.root.Preferences.saltBytes = []byte(conf.root.Preferences.Salt)
	} else {
		conf.root.Preferences = &Preferences{saltBytes: make([]byte, 0)}
	}
	conf.fixup(make(map[interface{}]valueID))
	conf.preCalculateValueIds()
	conf.generateEvaluators()
	conf.defaultUserSnapshot = _newSnapshot(conf, defaultUser, logger, hooks)
	return conf, nil
}

// preCalculateValueIds populates valueIds with value IDs that
// are known ahead of time or negative indexes into the cache slice
// where the value will be stored later.
func (c *config) preCalculateValueIds() {
	for name, entry := range c.root.Settings {
		id := idForKey(name, true)
		if int(id) >= len(c.valueIds) {
			value := make([]valueID, id+1)
			copy(value, c.valueIds)
			c.valueIds = value
		}
		if len(entry.TargetingRules) == 0 && len(entry.PercentageOptions) == 0 {
			c.valueIds[id] = entry.valueID
			continue
		}
		c.keysWithRules++
		c.valueIds[id] = -valueID(c.keysWithRules)
	}
}

func (c *config) equal(c1 *config) bool {
	if c == c1 || c == nil || c1 == nil {
		return c == c1
	}
	return c.fetchTime.Equal(c1.fetchTime) && c.etag == c1.etag && bytes.Equal(c.jsonBody, c1.jsonBody)
}

func (c *config) equalContent(c1 *config) bool {
	if c == c1 || c == nil || c1 == nil {
		return c == c1
	}
	return bytes.Equal(c.jsonBody, c1.jsonBody)
}

func (c *config) withFetchTime(t time.Time) *config {
	c1 := *c
	c1.fetchTime = t
	return &c1
}

func (c *config) body() string {
	if c == nil {
		return ""
	}
	return string(c.jsonBody)
}

func (c *config) getKeyAndValueForVariation(variationID string) (string, interface{}) {
	if c == nil {
		return "", nil
	}
	kv := c.keyValues[variationID]
	return kv.key, kv.value
}

func (c *config) keys() []string {
	if c == nil {
		return nil
	}
	return c.allKeys
}

// fixup populates the valueID fields in conf.root and presets related fields.
func (c *config) fixup(valueMap map[interface{}]valueID) {
	for key, setting := range c.root.Settings {
		setting.valueID = c.idForValue(setting.Value, setting.Type, valueMap)
		setting.keyBytes = []byte(key)
		for _, rule := range setting.TargetingRules {
			if rule.ServedValue != nil {
				rule.ServedValue.valueID = c.idForValue(rule.ServedValue.Value, setting.Type, valueMap)
			}
			if rule.Conditions != nil {
				for _, condition := range rule.Conditions {
					if condition.PrerequisiteFlagCondition != nil {
						if prerequisite, ok := c.root.Settings[condition.PrerequisiteFlagCondition.FlagKey]; ok {
							condition.PrerequisiteFlagCondition.valueID = c.idForValue(condition.PrerequisiteFlagCondition.Value, prerequisite.Type, valueMap)
						}
					}
					if condition.SegmentCondition != nil {
						condition.SegmentCondition.relatedSegment = c.root.Segments[condition.SegmentCondition.Index]
					}
				}
			}
			if rule.PercentageOptions != nil {
				for _, option := range rule.PercentageOptions {
					option.valueID = c.idForValue(option.Value, setting.Type, valueMap)
				}
			}
		}
		for _, rule := range setting.PercentageOptions {
			rule.valueID = c.idForValue(rule.Value, setting.Type, valueMap)
		}
	}
	for _, segment := range c.root.Segments {
		segment.nameBytes = []byte(segment.Name)
	}
}

func (c *config) idForValue(v *SettingValue, settingType SettingType, valueMap map[interface{}]valueID) valueID {
	actualValue := valueForSettingType(v, settingType)
	if id, ok := valueMap[actualValue]; ok {
		return id
	}
	// Start at 1 so the zero value always means "not known yet"
	// so we can rely on zero-initialization of the evaluation context.
	id := valueID(len(c.values) + 1)
	valueMap[actualValue] = id
	c.values = append(c.values, actualValue)
	return id
}

func mergeWithOverrides(root *ConfigJson, overrides *FlagOverrides) {
	if overrides == nil {
		return
	}
	if overrides.Behavior == LocalOnly || len(root.Settings) == 0 {
		root.Settings = overrides.settings
		return
	}
	if root.Settings == nil {
		root.Settings = make(map[string]*Setting)
	}
	for key, localEntry := range overrides.settings {
		setting, ok := root.Settings[key]
		switch {
		case !ok:
			root.Settings[key] = localEntry
		case overrides.Behavior == RemoteOverLocal:
		case setting.Type == localEntry.Type:
			*setting = *localEntry
		case setting.Type == IntSetting && localEntry.Type == FloatSetting:
			*setting = *localEntry
			changeToInt(setting)
		default:
			// Type clash. Just override anyway.
			// TODO could return an error in this case, as it's likely to be a local config issue.
			*setting = *localEntry
		}
	}
}

func changeToInt(setting *Setting) {
	setting.Type = IntSetting
	setting.Value.IntValue = int(setting.Value.DoubleValue)
	if len(setting.TargetingRules) > 0 {
		for _, rule := range setting.TargetingRules {
			if rule.ServedValue != nil {
				rule.ServedValue.Value.IntValue = int(rule.ServedValue.Value.DoubleValue)
			}
			if len(rule.PercentageOptions) > 0 {
				for _, opt := range rule.PercentageOptions {
					opt.Value.IntValue = int(opt.Value.DoubleValue)
				}
			}
		}
		if len(setting.PercentageOptions) > 0 {
			for _, opt := range setting.PercentageOptions {
				opt.Value.IntValue = int(opt.Value.DoubleValue)
			}
		}
	}
}

func valueForSettingType(v *SettingValue, settingType SettingType) interface{} {
	switch settingType {
	case BoolSetting:
		return v.BoolValue
	case IntSetting:
		return v.IntValue
	case FloatSetting:
		return v.DoubleValue
	case StringSetting:
		return v.StringValue
	default:
		return nil
	}
}
