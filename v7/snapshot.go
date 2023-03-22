package configcat

import (
	"errors"
	"fmt"
	"github.com/configcat/go-sdk/v7/internal/wireconfig"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Snapshot holds a snapshot of the Configcat configuration.
// A snapshot is immutable once taken.
//
// A nil snapshot is OK to use and acts like a configuration
// with no keys.
type Snapshot struct {
	logger       *leveledLogger
	config       *config
	hooks        *Hooks
	originalUser User
	user         reflect.Value
	allKeys      []string

	// values holds the value for each possible value ID, as stored in config.values.
	values []interface{}

	// precalc holds precalculated value IDs as stored in config.precalc.
	precalc []valueID

	// cache records the value IDs that have been
	// recorded for values that are not known ahead
	// of time. Zero IDs represent Zero IDs represent unevaluated results.
	// Entries are accessed and updated atomically which
	// provides a mechanism whereby the result for any given key
	// is only computed once for a given snapshot.
	//
	// The slot for a given key k is found at cache[-precalc[keyID(k)]-1].
	//
	// The cache is created by makeCache.
	cache     []valueID
	makeCache sync.Once

	// evaluators maps keyID to the evaluator for that key.
	evaluators []entryEvalFunc
}

// NewSnapshot returns a snapshot that always returns the given values.
//
// Each entry in the values map is keyed by a flag
// name and holds the value that the snapshot will return
// for that flag. Each value must be one of the types
// bool, int, float64, or string.
//
// The returned snapshot does not support variation IDs. That is, given a
// snapshot s returned by NewSnapshot:
// - s.GetKeyValueForVariationID returns "", nil.
// - s.GetVariationID returns "".
// - s.GetVariationIDs returns nil.
func NewSnapshot(logger Logger, values map[string]interface{}) (*Snapshot, error) {
	valuesSlice := make([]interface{}, numKeys())
	keys := make([]string, 0, len(values))
	for name, val := range values {
		switch val.(type) {
		case bool, int, float64, string:
		default:
			return nil, fmt.Errorf("value for flag %q has unexpected type %T (%#v); must be bool, int, float64 or string", name, val, val)
		}
		id := idForKey(name, true)
		if int(id) >= len(valuesSlice) {
			// We've added a new key, so expand the slices.
			// This should be a rare case, so don't worry about
			// this happening several times within this loop.
			valuesSlice1 := make([]interface{}, id+1)
			copy(valuesSlice1, valuesSlice)
			valuesSlice = valuesSlice1
		}
		valuesSlice[id] = val
		keys = append(keys, name)
	}
	// Save some allocations by using the same closure for every key.
	eval := func(id keyID, logger *leveledLogger, userv reflect.Value) (valueID, string, *wireconfig.RolloutRule, *wireconfig.PercentageRule) {
		return valueID(id) + 1, "", nil, nil
	}
	evaluators := make([]entryEvalFunc, len(valuesSlice))
	precalc := make([]valueID, len(valuesSlice))
	for i := range evaluators {
		evaluators[i] = eval
		precalc[i] = valueID(i) + 1
	}
	return &Snapshot{
		logger:     newLeveledLogger(logger, nil),
		evaluators: evaluators,
		allKeys:    keys,
		values:     valuesSlice,
		precalc:    precalc,
	}, nil
}

func newSnapshot(cfg *config, user User, logger *leveledLogger, hooks *Hooks) *Snapshot {
	if cfg != nil && (user == nil || user == cfg.defaultUser) {
		return cfg.defaultUserSnapshot
	}
	return _newSnapshot(cfg, user, logger, hooks)
}

// _newSnapshot is like newSnapshot except that it doesn't check
// whether user is nil. It should only be used by the parseConfig code
// for initializing config.noUserSnapshot.
func _newSnapshot(cfg *config, user User, logger *leveledLogger, hooks *Hooks) *Snapshot {
	snap := &Snapshot{
		config:       cfg,
		user:         reflect.ValueOf(user),
		logger:       logger,
		originalUser: user,
		hooks:        hooks,
	}
	var userType reflect.Type
	if user != nil {
		userType = snap.user.Type()
	}
	if cfg == nil {
		return snap
	}
	evaluators, err := cfg.evaluatorsForUserType(userType)
	if err != nil {
		logger.Errorf(0, "%v", err)
		return snap
	}
	snap.evaluators = evaluators
	snap.values = cfg.values
	snap.allKeys = cfg.allKeys
	snap.precalc = cfg.precalc
	return snap
}

// WithUser returns a copy of s associated with the
// given user. If snap is nil, it returns nil.
// If user is nil, it uses Config.DefaultUser.
func (snap *Snapshot) WithUser(user User) *Snapshot {
	if snap == nil || snap.config == nil {
		// Note: when there's no config, we know there are no
		// rules that can change the values returned, so no
		// need to do anything.
		return snap
	}
	if user == nil || user == snap.config.defaultUser {
		return snap.config.defaultUserSnapshot
	}
	return newSnapshot(snap.config, user, snap.logger, snap.hooks)
}

func (snap *Snapshot) value(id keyID, key string) interface{} {
	if snap == nil {
		return nil
	}
	if snap.logger.enabled(LogLevelInfo) || int(id) >= len(snap.precalc) || (snap.hooks != nil && snap.hooks.OnFlagEvaluated != nil) {
		// We want to see logs, or we don't know about the key so use the slow path.
		return snap.valueFromDetails(id, key)
	}
	valID := snap.precalc[id]
	if valID > 0 {
		// We've got a precalculated value for the key.
		return snap.valueForID(valID)
	}
	if valID == 0 {
		// The key isn't found in this configuration.
		if !snap.logger.enabled(LogLevelError) {
			return nil
		}
		// Use the default implementation which will do the
		// appropriate logging for us.
		return snap.valueFromDetails(id, key)
	}
	// Look up the key in the cache.
	cacheIndex := int(-valID - 1)
	snap.initCache()
	if valID := atomic.LoadInt32(&snap.cache[cacheIndex]); valID > 0 {
		// We've got a previous result, so return it. Note that we can only do this
		// when we're not printing Info-level logs, because this avoids the usual
		// logging of rule evaluation.
		return snap.valueForID(valID)
	}

	return snap.valueFromDetails(id, key)
}

func (snap *Snapshot) initCache() {
	snap.makeCache.Do(func() {
		snap.cache = make([]valueID, snap.config.keysWithRules)
	})
}

func (snap *Snapshot) valueFromDetails(id keyID, key string) interface{} {
	if value, _, _, _, err := snap.details(id, key); err == nil {
		return value
	}
	return nil
}

func (snap *Snapshot) details(id keyID, key string) (interface{}, string, *wireconfig.RolloutRule, *wireconfig.PercentageRule, error) {
	if snap == nil {
		return nil, "", nil, nil, errors.New("snapshot is nil")
	}
	var eval entryEvalFunc
	if int(id) < len(snap.evaluators) {
		eval = snap.evaluators[id]
	}
	if eval == nil {
		var message = "Failed to evaluate setting '%s' (the key was not found in config JSON). Available keys: %s."
		var availableKeys = strings.Join(snap.GetAllKeys(), ", ")
		snap.logger.Errorf(1001, message, key, availableKeys)
		return nil, "", nil, nil, fmt.Errorf(message, key, availableKeys)
	}
	valID, varID, rollout, percentage := eval(id, snap.logger, snap.user)
	val := snap.valueForID(valID)
	if snap.logger.enabled(LogLevelInfo) {
		snap.logger.Infof(5000, "Returning %v=%v.", key, val)
	}
	if v := snap.precalc[id]; v < 0 {
		snap.initCache()
		cacheIndex := -v - 1
		atomic.StoreInt32(&snap.cache[cacheIndex], valID)
	}
	if snap.hooks != nil && snap.hooks.OnFlagEvaluated != nil {
		go snap.hooks.OnFlagEvaluated(&EvaluationDetails{
			Value: val,
			Data: EvaluationDetailsData{
				Key:                             key,
				VariationID:                     varID,
				User:                            snap.originalUser,
				FetchTime:                       snap.FetchTime(),
				MatchedEvaluationRule:           newPublicRolloutRuleOrNil(rollout),
				MatchedEvaluationPercentageRule: newPublicPercentageRuleOrNil(percentage),
			},
		})
	}
	return val, varID, rollout, percentage, nil
}

func (snap *Snapshot) evalDetailsForKeyId(id keyID, key string) EvaluationDetails {
	value, varID, rollout, percentage, err := snap.details(id, key)
	if err != nil {
		return EvaluationDetails{Value: nil, Data: EvaluationDetailsData{
			Key:            key,
			User:           snap.originalUser,
			IsDefaultValue: true,
			Error:          err,
			FetchTime:      snap.FetchTime(),
		}}
	}

	return EvaluationDetails{Value: value, Data: EvaluationDetailsData{
		Key:                             key,
		VariationID:                     varID,
		User:                            snap.originalUser,
		FetchTime:                       snap.FetchTime(),
		MatchedEvaluationRule:           newPublicRolloutRuleOrNil(rollout),
		MatchedEvaluationPercentageRule: newPublicPercentageRuleOrNil(percentage),
	}}
}

// valueForID returns the actual value corresponding to
// the given value ID.
func (snap *Snapshot) valueForID(id valueID) interface{} {
	return snap.values[id-1]
}

// GetValue returns a feature flag value regardless of type. If there is no
// value found, it returns nil; otherwise the returned value
// has one of the dynamic types bool, int, float64, or string.
//
// To use obtain the value of a typed feature flag, use
// one of the typed feature flag functions. For example:
//
//	someFlag := configcat.Bool("someFlag", false)
//	value := someFlag.Get(snap)
func (snap *Snapshot) GetValue(key string) interface{} {
	return snap.value(idForKey(key, false), key)
}

// GetValueDetails returns the value and evaluation details of a feature flag or setting
// with respect to the current user, or nil if none is found.
func (snap *Snapshot) GetValueDetails(key string) EvaluationDetails {
	return snap.evalDetailsForKeyId(idForKey(key, false), key)
}

// GetAllValueDetails returns values along with evaluation details of all feature flags and settings.
func (snap *Snapshot) GetAllValueDetails() []EvaluationDetails {
	keys := snap.GetAllKeys()
	details := make([]EvaluationDetails, 0, len(keys))
	for _, key := range keys {
		details = append(details, snap.GetValueDetails(key))
	}
	return details
}

// GetKeyValueForVariationID returns the key and value that
// are associated with the given variation ID. If the
// variation ID isn't found, it returns "", nil.
func (snap *Snapshot) GetKeyValueForVariationID(id string) (string, interface{}) {
	if snap == nil {
		return "", nil
	}
	key, value := snap.config.getKeyAndValueForVariation(id)
	if key == "" {
		snap.logger.Errorf(2011, "Could not find the setting for the specified variation ID: '%s'. Returning nil.", id)
		return "", nil
	}
	return key, value
}

// GetVariationID returns the variation ID that will be used for the given key
// with respect to the current user, or the empty string if none is found.
// Deprecated: This method is obsolete and will be removed in a future major version. Please use GetValueDetails instead.
func (snap *Snapshot) GetVariationID(key string) string {
	if _, varID, _, _, err := snap.details(idForKey(key, false), key); err == nil {
		return varID
	}
	return ""
}

// GetVariationIDs returns all variation IDs in the current configuration
// that apply to the current user.
// Deprecated: This method is obsolete and will be removed in a future major version. Please use GetAllValueDetails instead.
func (snap *Snapshot) GetVariationIDs() []string {
	if snap == nil || snap.config == nil {
		return nil
	}
	keys := snap.config.keys()
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		id := idForKey(key, false)
		_, varID, _, _ := snap.evaluators[id](id, snap.logger, snap.user)
		ids = append(ids, varID)
	}
	return ids
}

// GetAllKeys returns all the known keys in arbitrary order.
func (snap *Snapshot) GetAllKeys() []string {
	if snap == nil {
		return nil
	}
	return snap.allKeys
}

// GetAllValues returns all keys and values in freshly allocated key-value map.
func (snap *Snapshot) GetAllValues() map[string]interface{} {
	keys := snap.GetAllKeys()
	values := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		values[key] = snap.GetValue(key)
	}
	return values
}

func (snap *Snapshot) FetchTime() time.Time {
	if snap == nil || snap.config == nil {
		return time.Time{}
	}
	return snap.config.fetchTime
}
