package configcat

import (
	"errors"
	"fmt"
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
	originalUser User
	user         reflect.Value
	allKeys      []string

	// values holds the value and eval details for each
	// possible value ID, as stored in config.values.
	values []valueDetails

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
	valuesSlice := make([]valueDetails, numKeys())
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
			valuesSlice1 := make([]valueDetails, id+1)
			copy(valuesSlice1, valuesSlice)
			valuesSlice = valuesSlice1
		}
		valuesSlice[id] = valueDetails{value: val}
		keys = append(keys, name)
	}
	// Save some allocations by using the same closure for every key.
	eval := func(id keyID, logger *leveledLogger, userv reflect.Value) valueID {
		return valueID(id) + 1
	}
	evaluators := make([]entryEvalFunc, len(valuesSlice))
	precalc := make([]valueID, len(valuesSlice))
	for i := range evaluators {
		evaluators[i] = eval
		precalc[i] = valueID(i) + 1
	}
	return &Snapshot{
		logger:     newLeveledLogger(logger),
		evaluators: evaluators,
		allKeys:    keys,
		values:     valuesSlice,
		precalc:    precalc,
	}, nil
}

func newSnapshot(cfg *config, user User, logger *leveledLogger) *Snapshot {
	if cfg != nil && (user == nil || user == cfg.defaultUser) {
		return cfg.defaultUserSnapshot
	}
	return _newSnapshot(cfg, user, logger)
}

// _newSnapshot is like newSnapshot except that it doesn't check
// whether user is nil. It should only be used by the parseConfig code
// for initializing config.noUserSnapshot.
func _newSnapshot(cfg *config, user User, logger *leveledLogger) *Snapshot {
	snap := &Snapshot{
		config:       cfg,
		user:         reflect.ValueOf(user),
		logger:       logger,
		originalUser: user,
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
		logger.Errorf("%v", err)
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
	return newSnapshot(snap.config, user, snap.logger)
}

func (snap *Snapshot) value(id keyID, key string) interface{} {
	if details, err := snap.evalDetails(id, key); err == nil {
		return details.value
	}
	return nil
}

func (snap *Snapshot) evalDetails(id keyID, key string) (*valueDetails, error) {
	if snap == nil {
		return nil, errors.New("snapshot is nil")
	}
	if snap.logger.enabled(LogLevelInfo) || int(id) >= len(snap.precalc) {
		// We want to see logs, or we don't know about the key so use the slow path.
		return snap.valueWithDetails(id, key)
	}
	valID := snap.precalc[id]
	if valID > 0 {
		// We've got a precalculated value for the key.
		return snap.valueForID(valID), nil
	}
	if valID == 0 {
		// Use the default implementation which will
		// get the appropriate error for us
		return snap.valueWithDetails(id, key)
	}
	// Look up the key in the cache.
	cacheIndex := int(-valID - 1)
	snap.initCache()
	if valID := atomic.LoadInt32(&snap.cache[cacheIndex]); valID > 0 {
		// We've got a previous result, so return it. Note that we can only do this
		// when we're not printing Info-level logs, because this avoids the usual
		// logging of rule evaluation.
		return snap.valueForID(valID), nil
	}

	return snap.valueWithDetails(id, key)
}

func (snap *Snapshot) initCache() {
	snap.makeCache.Do(func() {
		snap.cache = make([]valueID, snap.config.keysWithRules)
	})
}

func (snap *Snapshot) valueWithDetails(id keyID, key string) (*valueDetails, error) {
	if snap == nil {
		return nil, errors.New("snapshot is nil")
	}
	var eval entryEvalFunc
	if int(id) < len(snap.evaluators) {
		eval = snap.evaluators[id]
	}
	if eval == nil {
		err := fmt.Sprintf("error getting value: value not found for key %s."+
			" Here are the available keys: %s", key, strings.Join(snap.GetAllKeys(), ","))
		snap.logger.Errorf(err)
		return nil, errors.New(err)
	}
	valID := eval(id, snap.logger, snap.user)
	val := snap.valueForID(valID)
	if snap.logger.enabled(LogLevelInfo) {
		snap.logger.Infof("Returning %v=%v.", key, val.value)
	}
	if v := snap.precalc[id]; v < 0 {
		snap.initCache()
		cacheIndex := -v - 1
		atomic.StoreInt32(&snap.cache[cacheIndex], valID)
	}
	return val, nil
}

func (snap *Snapshot) evalDetailsForKeyId(id keyID, key string) EvaluationDetails {
	details, err := snap.evalDetails(id, key)
	if err != nil {
		return EvaluationDetails{Value: nil, Meta: EvaluationDetailsMeta{
			Key:            key,
			User:           snap.originalUser,
			IsDefaultValue: true,
			Error:          err,
			FetchTime:      snap.config.fetchTime,
		}}
	}

	return EvaluationDetails{Value: details.value, Meta: EvaluationDetailsMeta{
		Key:                             key,
		VariationId:                     details.variationId,
		User:                            snap.originalUser,
		IsDefaultValue:                  false,
		Error:                           nil,
		FetchTime:                       snap.config.fetchTime,
		MatchedEvaluationRule:           *details.rolloutRule,
		MatchedEvaluationPercentageRule: *details.percentageRule,
	}}
}

// valueForID returns the actual value corresponding to
// the given value ID.
func (snap *Snapshot) valueForID(id valueID) *valueDetails {
	return &snap.values[id-1]
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

// GetValueDetails returns the value and evaluation details of a feature flag or setting
// with respect to the current user, or nil if none is found.
func (snap *Snapshot) GetValueDetails(key string) EvaluationDetails {
	return snap.evalDetailsForKeyId(idForKey(key, false), key)
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

// GetVariationID returns the variation ID that will be used for the given key
// with respect to the current user, or the empty string if none is found.
func (snap *Snapshot) GetVariationID(key string) string {
	if details, err := snap.evalDetails(idForKey(key, false), key); err == nil {
		return details.variationId
	}
	return ""
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
		valId := snap.evaluators[id](id, snap.logger, snap.user)
		val := snap.valueForID(valId)
		ids = append(ids, val.variationId)
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
