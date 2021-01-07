package configcat

import (
	"sync"
)

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
		defaultValue: defaultValue,
	}
}

type BoolFlag struct {
	id           keyID
	key          string
	defaultValue bool
}

func (f BoolFlag) Get(snap *Snapshot) bool {
	if v, ok := snap.value(f.id, f.key).(bool); ok {
		return v
	}
	return f.defaultValue
}

// Int is like Bool but for int-valued flags.
func Int(key string, defaultValue int) IntFlag {
	return IntFlag{
		id:           idForKey(key, true),
		key:          key,
		defaultValue: defaultValue,
	}
}

type IntFlag struct {
	id           keyID
	key          string
	defaultValue int
}

func (f IntFlag) Get(snap *Snapshot) int {
	if v, ok := snap.value(f.id, f.key).(int); ok {
		return v
	}
	return f.defaultValue
}

// String is like Bool but for string-valued flags.
func String(key string, defaultValue string) StringFlag {
	return StringFlag{
		id:           idForKey(key, true),
		key:          key,
		defaultValue: defaultValue,
	}
}

type StringFlag struct {
	id           keyID
	key          string
	defaultValue string
}

func (f StringFlag) Get(snap *Snapshot) string {
	if v, ok := snap.value(f.id, f.key).(string); ok {
		return v
	}
	return f.defaultValue
}

// Float is like Bool but for float-valued flags.
func Float(key string, defaultValue float64) FloatFlag {
	return FloatFlag{
		id:           idForKey(key, true),
		key:          key,
		defaultValue: defaultValue,
	}
}

type FloatFlag struct {
	id           keyID
	key          string
	defaultValue float64
}

func (f FloatFlag) Get(snap *Snapshot) float64 {
	if v, ok := snap.value(f.id, f.key).(float64); ok {
		return v
	}
	return f.defaultValue
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
