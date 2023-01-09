package configcat

import (
	"fmt"
	"sync"
)

// Flag is the interface implemented by all flag types.
type Flag interface {
	// Key returns the flag's key.
	Key() string

	// GetValue returns the flag's value. It will always
	// return the appropriate type for the flag (never nil).
	GetValue(snap *Snapshot) interface{}

	// GetValueDetails returns the evaluation details
	// along with the flag's value. Its Value field always
	// have the appropriate type for the flag.
	GetValueDetails(snap *Snapshot) EvaluationDetails
}

// Bool returns a representation of a boolean-valued flag.
// This can to be used as the value of a global variable
// for a specific flag; for example:
//
//	var fooFlag = configcat.Bool("foo", false)
//
//	func someRequest(client *configcat.Client) {
//		if fooFlag.Get(client.Snapshot()) {
//			// foo is enabled.
//		}
//	}
func Bool(key string, defaultValue bool) BoolFlag {
	return BoolFlag{
		id:           idForKey(key, true),
		key:          key,
		defaultValue: defaultValue,
	}
}

var _ Flag = BoolFlag{}

type BoolFlag struct {
	id           keyID
	key          string
	defaultValue interface{}
}

// Key returns the name of the flag as passed to Bool.
func (f BoolFlag) Key() string {
	return f.key
}

// Get reports whether the flag is enabled with respect to the
// given snapshot. It returns the flag's default value if snap is nil
// or the key isn't in the configuration.
func (f BoolFlag) Get(snap *Snapshot) bool {
	return f.GetValue(snap).(bool)
}

// GetWithDetails returns the evaluation details along with the flag's value.
// It returns BoolEvaluationDetails with the flag's default value if snap is nil
// or the key isn't in the configuration.
func (f BoolFlag) GetWithDetails(snap *Snapshot) BoolEvaluationDetails {
	details := f.GetValueDetails(snap)
	return BoolEvaluationDetails{Data: details.Data, Value: details.Value.(bool)}
}

// GetValue implements Flag.GetValue.
func (f BoolFlag) GetValue(snap *Snapshot) interface{} {
	v := snap.value(f.id, f.key)
	if _, ok := v.(bool); ok {
		return v
	}
	return f.defaultValue
}

// GetValueDetails implements Flag.GetValueDetails.
func (f BoolFlag) GetValueDetails(snap *Snapshot) EvaluationDetails {
	details := snap.evalDetailsForKeyId(f.id, f.key)
	boolVal, ok := details.Value.(bool)
	if !ok {
		return EvaluationDetails{
			Value: f.defaultValue,
			Data: EvaluationDetailsData{
				Key:            f.key,
				Error:          fmt.Errorf("could not convert %s to bool", details.Value),
				User:           snap.originalUser,
				FetchTime:      snap.FetchTime(),
				IsDefaultValue: true,
			},
		}
	}
	details.Value = boolVal
	return details
}

// Int is like Bool but for int-valued flags.
func Int(key string, defaultValue int) IntFlag {
	return IntFlag{
		id:           idForKey(key, true),
		key:          key,
		defaultValue: defaultValue,
	}
}

var _ Flag = IntFlag{}

type IntFlag struct {
	id           keyID
	key          string
	defaultValue interface{}
}

// Key returns the name of the flag as passed to Int.
func (f IntFlag) Key() string {
	return f.key
}

// Get reports the value of the flag with respect to the
// given snapshot. It returns the flag's default value if snap is nil
// or the key isn't in the configuration.
func (f IntFlag) Get(snap *Snapshot) int {
	return f.GetValue(snap).(int)
}

// GetWithDetails returns the evaluation details along with the flag's value.
// It returns IntEvaluationDetails with the flag's default value if snap is nil
// or the key isn't in the configuration.
func (f IntFlag) GetWithDetails(snap *Snapshot) IntEvaluationDetails {
	details := f.GetValueDetails(snap)
	return IntEvaluationDetails{Data: details.Data, Value: details.Value.(int)}
}

// GetValue implements Flag.GetValue.
func (f IntFlag) GetValue(snap *Snapshot) interface{} {
	v := snap.value(f.id, f.key)
	if res, ok := convertInt(v); ok {
		return res
	}
	return f.defaultValue
}

// GetValueDetails implements Flag.GetValueDetails.
func (f IntFlag) GetValueDetails(snap *Snapshot) EvaluationDetails {
	details := snap.evalDetailsForKeyId(f.id, f.key)
	intVal, ok := convertInt(details.Value)
	if !ok {
		return EvaluationDetails{
			Value: f.defaultValue,
			Data: EvaluationDetailsData{
				Key:            f.key,
				Error:          fmt.Errorf("could not convert %s to int", details.Value),
				User:           snap.originalUser,
				FetchTime:      snap.FetchTime(),
				IsDefaultValue: true,
			},
		}
	}
	details.Value = intVal
	return details
}

// String is like Bool but for string-valued flags.
func String(key string, defaultValue string) StringFlag {
	return StringFlag{
		id:           idForKey(key, true),
		key:          key,
		defaultValue: defaultValue,
	}
}

var _ Flag = StringFlag{}

type StringFlag struct {
	id           keyID
	key          string
	defaultValue interface{}
}

// Key returns the name of the flag as passed to String.
func (f StringFlag) Key() string {
	return f.key
}

// Get reports the value of the flag with respect to the
// given snapshot. It returns the flag's default value if snap is nil
// or the key isn't in the configuration.
func (f StringFlag) Get(snap *Snapshot) string {
	return f.GetValue(snap).(string)
}

// GetWithDetails returns the evaluation details along with the flag's value.
// It returns StringEvaluationDetails with the flag's default value if snap is nil
// or the key isn't in the configuration.
func (f StringFlag) GetWithDetails(snap *Snapshot) StringEvaluationDetails {
	details := f.GetValueDetails(snap)
	return StringEvaluationDetails{Data: details.Data, Value: details.Value.(string)}
}

// GetValue implements Flag.GetValue.
func (f StringFlag) GetValue(snap *Snapshot) interface{} {
	v := snap.value(f.id, f.key)
	if _, ok := v.(string); ok {
		return v
	}
	return f.defaultValue
}

// GetValueDetails implements Flag.GetValueDetails.
func (f StringFlag) GetValueDetails(snap *Snapshot) EvaluationDetails {
	details := snap.evalDetailsForKeyId(f.id, f.key)
	stringVal, ok := details.Value.(string)
	if !ok {
		return EvaluationDetails{
			Value: f.defaultValue,
			Data: EvaluationDetailsData{
				Key:            f.key,
				Error:          fmt.Errorf("could not convert %s to string", details.Value),
				User:           snap.originalUser,
				FetchTime:      snap.FetchTime(),
				IsDefaultValue: true,
			},
		}
	}
	details.Value = stringVal
	return details
}

// Float is like Bool but for float-valued flags.
func Float(key string, defaultValue float64) FloatFlag {
	return FloatFlag{
		id:           idForKey(key, true),
		key:          key,
		defaultValue: defaultValue,
	}
}

var _ Flag = FloatFlag{}

type FloatFlag struct {
	id           keyID
	key          string
	defaultValue interface{}
}

// Key returns the name of the flag as passed to Float.
func (f FloatFlag) Key() string {
	return f.key
}

// Get reports the value of the flag with respect to the
// given snapshot. It returns the flag's default value if snap is nil
// or the key isn't in the configuration.
func (f FloatFlag) Get(snap *Snapshot) float64 {
	return f.GetValue(snap).(float64)
}

// GetWithDetails returns the evaluation details along with the flag's value.
// It returns FloatEvaluationDetails with the flag's default value if snap is nil
// or the key isn't in the configuration.
func (f FloatFlag) GetWithDetails(snap *Snapshot) FloatEvaluationDetails {
	details := f.GetValueDetails(snap)
	return FloatEvaluationDetails{Data: details.Data, Value: details.Value.(float64)}
}

// GetValue implements Flag.GetValue.
func (f FloatFlag) GetValue(snap *Snapshot) interface{} {
	v := snap.value(f.id, f.key)
	if _, ok := v.(float64); ok {
		return v
	}
	return f.defaultValue
}

// GetValueDetails implements Flag.GetValueDetails.
func (f FloatFlag) GetValueDetails(snap *Snapshot) EvaluationDetails {
	details := snap.evalDetailsForKeyId(f.id, f.key)
	floatVal, ok := details.Value.(float64)
	if !ok {
		return EvaluationDetails{
			Value: f.defaultValue,
			Data: EvaluationDetailsData{
				Key:            f.key,
				Error:          fmt.Errorf("could not convert %s to float64", details.Value),
				User:           snap.originalUser,
				FetchTime:      snap.FetchTime(),
				IsDefaultValue: true,
			},
		}
	}
	details.Value = floatVal
	return details
}

type keyID uint32

var keyIDs struct {
	ids sync.Map // string -> keyID
	mu  sync.Mutex
	max keyID
}

const unknownKey = 0xffffffff

func idForKey(key string, add bool) keyID {
	id, ok := keyIDs.ids.Load(key)
	if ok {
		return id.(keyID)
	}
	if !add {
		return unknownKey
	}
	keyIDs.mu.Lock()
	defer keyIDs.mu.Unlock()
	id, ok = keyIDs.ids.Load(key)
	if ok {
		return id.(keyID)
	}
	id1 := keyIDs.max
	keyIDs.ids.Store(key, id1)
	keyIDs.max++
	return id1
}

func numKeys() int {
	keyIDs.mu.Lock()
	defer keyIDs.mu.Unlock()
	return int(keyIDs.max)
}

func convertInt(val interface{}) (interface{}, bool) {
	switch v1 := val.(type) {
	case int:
		return val, true
	case float64:
		// This can happen when a numeric override flag is used
		// with SimplifiedConfig, which can't tell the difference
		// between int and float64.
		return int(v1), true
	}
	return nil, false
}
