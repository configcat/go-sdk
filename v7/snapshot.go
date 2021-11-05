package configcat

import (
	"fmt"
	"reflect"
	"strings"
)

// Snapshot holds a snapshot of the Configcat configuration.
// A snapshot is immutable once taken.
//
// A nil snapshot is OK to use and acts like a configuration
// with no keys.
type Snapshot struct {
	logger  *leveledLogger
	config  *config
	user    reflect.Value
	allKeys []string
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
	valueSlice := make([]interface{}, numKeys())
	keys := make([]string, 0, len(values))

	for name, val := range values {
		switch val.(type) {
		case bool, int, float64, string:
		default:
			return nil, fmt.Errorf("value for flag %q has unexpected type %T (%#v); must be bool, int, float64 or string", name, val, val)
		}
		id := idForKey(name, true)
		if int(id) >= len(valueSlice) {
			// We've added a new key, so expand the slices.
			// This should be a rare case, so don't worry about
			// this happening several times within this loop.
			valueSlice1 := make([]interface{}, id+1)
			copy(valueSlice1, valueSlice)
			valueSlice = valueSlice1
		}
		valueSlice[id] = val
		keys = append(keys, name)
	}
	// Save some allocations by using the same closure for every key.
	eval := func(id keyID, logger *leveledLogger, userv reflect.Value) (interface{}, string) {
		return valueSlice[id], ""
	}
	evaluators := make([]entryEvalFunc, len(valueSlice))
	for i := range evaluators {
		evaluators[i] = eval
	}
	return &Snapshot{
		logger:     newLeveledLogger(logger),
		evaluators: evaluators,
		allKeys:    keys,
	}, nil
}

func newSnapshot(cfg *config, user User, logger *leveledLogger) *Snapshot {
	snap := &Snapshot{
		config: cfg,
		user:   reflect.ValueOf(user),
		logger: logger,
	}
	var userType reflect.Type
	if user != nil {
		userType = snap.user.Type()
	}
	if cfg == nil {
		return snap
	}
	snap.allKeys = cfg.allKeys
	evaluators, err := cfg.evaluatorsForUserType(userType)
	if err != nil {
		logger.Errorf("%v", err)
		return snap
	}
	snap.evaluators = evaluators
	return snap
}

// WithUser returns a copy of s associated with the
// given user. If snap is nil, it returns nil.
func (snap *Snapshot) WithUser(user User) *Snapshot {
	if snap == nil || snap.config == nil {
		// Note: when there's no config, we know there are no
		// rules that can change the values returned, so no
		// need to do anything.
		return snap
	}
	return newSnapshot(snap.config, user, snap.logger)
}

func (snap *Snapshot) value(id keyID, key string) interface{} {
	val, _ := snap.valueAndVariationID(id, key)
	return val
}

// GetVariationID returns the variation ID that will be used for the given key
// with respect to the current user, or the empty string if none is found.
func (snap *Snapshot) GetVariationID(key string) string {
	_, variationID := snap.valueAndVariationID(idForKey(key, false), key)
	return variationID
}

func (snap *Snapshot) valueAndVariationID(id keyID, key string) (interface{}, string) {
	if snap == nil {
		return nil, ""
	}
	var eval entryEvalFunc
	if int(id) < len(snap.evaluators) {
		eval = snap.evaluators[id]
	}
	if eval == nil {
		snap.logger.Errorf("error getting value: value not found for key %s."+
			" Here are the available keys: %s", key, strings.Join(snap.GetAllKeys(), ","))
		return nil, ""
	}
	val, variationID := eval(id, snap.logger, snap.user)
	if snap.logger.enabled(LogLevelInfo) {
		snap.logger.Infof("Returning %v=%v.", key, val)
	}
	return val, variationID
}

// GetValue returns a feature flag value regardless of type. If there is no
// value found, it returns nil; otherwise the returned value
// has one of the dynamic types bool, int, float64, or string.
//
// To use obtain the value of a typed feature flag, use
// one of the typed feature flag functions. For example:
//
//	someFlag := configcat.Bool("someFlag", false)
// 	value := someFlag.Get(snap)
func (snap *Snapshot) GetValue(key string) interface{} {
	return snap.value(idForKey(key, false), key)
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
		snap.logger.Errorf("Evaluating GetKeyAndValue(%s) failed. Returning nil. Variation ID not found.", id)
		return "", nil
	}
	return key, value
}

// GetVariationIDs returns all variation IDs in the current configuration
// that apply to the current user.
func (snap *Snapshot) GetVariationIDs() []string {
	if snap == nil || snap.config == nil {
		return nil
	}
	keys := snap.config.keys()
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		id := idForKey(key, false)
		_, varID := snap.evaluators[id](id, snap.logger, snap.user)
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
