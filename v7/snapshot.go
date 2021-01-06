package configcat

import (
	"reflect"
	"strings"
)

// Snapshot holds a snapshot of the Configcat configuration.
// A snapshot is immutable once taken.
type Snapshot struct {
	logger *leveledLogger
	config *config
	user   reflect.Value
	// evaluators maps keyID to the evaluator for that key.
	evaluators []entryEvalFunc
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
	evaluators, err := cfg.evaluatorsForUserType(userType)
	if err != nil {
		logger.Errorf("%v", err)
		return snap
	}
	snap.evaluators = evaluators
	return snap
}

// WithUser returns a copy of s associated with the
// given user.
func (snap *Snapshot) WithUser(user User) *Snapshot {
	return newSnapshot(snap.config, user, snap.logger)
}

func (snap *Snapshot) value(id keyID, key string) interface{} {
	val, _ := snap.valueAndVariationID(id, key)
	return val
}

// VariationID returns the variation ID that will be used for the given key
// with respect to the current user, or the empty string if none is found.
func (snap *Snapshot) VariationID(key string) string {
	_, variationID := snap.valueAndVariationID(idForKey(key, false), key)
	return variationID
}

func (snap *Snapshot) valueAndVariationID(id keyID, key string) (interface{}, string) {
	var eval entryEvalFunc
	if int(id) < len(snap.evaluators) {
		eval = snap.evaluators[id]
	}
	if eval == nil {
		snap.logger.Errorf("error getting value: value not found for key %s."+
			" Here are the available keys: %s", key, strings.Join(snap.Keys(), ","))
		return nil, ""
	}
	return eval(snap.logger, snap.user)
}

// KeyValueForVariationID returns the key and value that
// are associated with the given variation ID. If the
// variation ID isn't found, it returns "", nil.
func (snap *Snapshot) KeyValueForVariationID(id string) (string, interface{}) {
	key, value := snap.config.getKeyAndValueForVariation(id)
	if key == "" {
		snap.logger.Errorf("Evaluating GetKeyAndValue(%s) failed. Returning nil. Variation ID not found.")
		return "", nil
	}
	return key, value
}

// VariationIDs returns all variation IDs in the current configuration
// that apply to the current user.
func (snap *Snapshot) VariationIDs() []string {
	keys := snap.config.keys()
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		_, varID := snap.evaluators[idForKey(key, false)](snap.logger, snap.user)
		ids = append(ids, varID)
	}
	return ids
}

// Keys returns all the known keys.
func (snap *Snapshot) Keys() []string {
	return snap.config.keys()
}
