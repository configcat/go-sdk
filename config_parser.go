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
	fixupSegmentsAndSalt(&root)
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
	conf.fixup(make(map[interface{}]valueID))
	conf.checkCycles()
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
	for _, setting := range c.root.Settings {
		setting.valueID = c.idForValue(setting.Value.Value, valueMap)
		for _, rule := range setting.TargetingRules {
			if rule.ServedValue != nil {
				rule.ServedValue.valueID = c.idForValue(rule.ServedValue.Value.Value, valueMap)
			}
			if rule.Conditions != nil {
				for _, condition := range rule.Conditions {
					if condition.PrerequisiteFlagCondition != nil {
						if prerequisite, ok := c.root.Settings[condition.PrerequisiteFlagCondition.FlagKey]; ok {
							condition.PrerequisiteFlagCondition.valueID = c.idForValue(valueFor(condition.PrerequisiteFlagCondition.Value), valueMap)
							condition.PrerequisiteFlagCondition.prerequisiteSettingType = prerequisite.Type
						}
					}
				}
			}
			if rule.PercentageOptions != nil {
				for _, option := range rule.PercentageOptions {
					option.valueID = c.idForValue(option.Value.Value, valueMap)
				}
			}
		}
		for _, rule := range setting.PercentageOptions {
			rule.valueID = c.idForValue(rule.Value.Value, valueMap)
		}
	}
}

func (c *config) idForValue(val interface{}, valueMap map[interface{}]valueID) valueID {
	if id, ok := valueMap[val]; ok {
		return id
	}
	// Start at 1 so the zero value always means "not known yet"
	// so we can rely on zero-initialization of the evaluation context.
	id := valueID(len(c.values) + 1)
	valueMap[val] = id
	c.values = append(c.values, val)
	return id
}

type cycleTracker struct {
	visitedKeys []string
}

func (c *cycleTracker) contains(key string) bool {
	for _, v := range c.visitedKeys {
		if v == key {
			return true
		}
	}
	return false
}

func (c *cycleTracker) append(key string) {
	c.visitedKeys = append(c.visitedKeys, key)
}

func (c *cycleTracker) removeLast() {
	if len(c.visitedKeys) > 0 {
		c.visitedKeys = c.visitedKeys[:len(c.visitedKeys)-1]
	}
}

func (c *config) checkCycles() {
	for key, setting := range c.root.Settings {
		tracker := &cycleTracker{}
		if c.checkCyclesForSetting(setting, key, tracker) {
			setting.prerequisiteCycle = tracker.visitedKeys
		}
	}
}

func (c *config) checkCyclesForSetting(s *Setting, key string, tracker *cycleTracker) bool {
	tracker.append(key)
	if len(s.TargetingRules) > 0 {
		for _, rule := range s.TargetingRules {
			if len(rule.Conditions) > 0 {
				for _, cond := range rule.Conditions {
					if cond.PrerequisiteFlagCondition != nil {
						if prerequisite, ok := c.root.Settings[cond.PrerequisiteFlagCondition.FlagKey]; ok {
							if tracker.contains(cond.PrerequisiteFlagCondition.FlagKey) {
								tracker.append(cond.PrerequisiteFlagCondition.FlagKey)
								return true
							}
							if c.checkCyclesForSetting(prerequisite, cond.PrerequisiteFlagCondition.FlagKey, tracker) {
								return true
							} else {
								tracker.removeLast()
							}
						}
					}
				}
			}
		}
	}
	return false
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
	setting.Value.Value = toIntVal(setting.Value.Value)
	if len(setting.TargetingRules) > 0 {
		for _, rule := range setting.TargetingRules {
			if rule.ServedValue != nil {
				rule.ServedValue.Value.Value = toIntVal(rule.ServedValue.Value.Value)
			}
			if len(rule.PercentageOptions) > 0 {
				for _, opt := range rule.PercentageOptions {
					opt.Value.Value = toIntVal(opt.Value.Value)
				}
			}
		}
		if len(setting.PercentageOptions) > 0 {
			for _, opt := range setting.PercentageOptions {
				opt.Value.Value = toIntVal(opt.Value.Value)
			}
		}
	}
}

func toIntVal(v interface{}) interface{} {
	f, ok := v.(float64)
	if !ok {
		return v
	}
	return int(f)
}

func fixupSegmentsAndSalt(root *ConfigJson) {
	var saltBytes []byte
	if root.Preferences != nil {
		saltBytes = []byte(root.Preferences.Salt)
	} else {
		saltBytes = make([]byte, 0)
	}
	for _, setting := range root.Settings {
		setting.saltBytes = saltBytes
		for _, rule := range setting.TargetingRules {
			if rule.Conditions != nil {
				for _, condition := range rule.Conditions {
					if condition.SegmentCondition != nil {
						condition.SegmentCondition.relatedSegment = root.Segments[condition.SegmentCondition.Index]
						condition.SegmentCondition.relatedSegment.nameBytes = []byte(condition.SegmentCondition.relatedSegment.Name)
					}
				}
			}
		}
	}
}
