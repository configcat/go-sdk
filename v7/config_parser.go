package configcat

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/configcat/go-sdk/v7/internal/wireconfig"
)

type config struct {
	jsonBody []byte
	etag     string
	root     *wireconfig.RootNode
	// Note: this is a pointer because the configuration
	// can be copied (with the withFetchTime method).
	evaluators *sync.Map // reflect.Type -> map[string]entryEvalFunc
	allKeys    []string
	keyValues  map[string]keyValue
	fetchTime  time.Time
	// values holds all the values that can be returned from the
	// configuration, keyed by valueID-1.
	values []interface{}

	// precalc holds value IDs for keys that we know
	// the values of ahead of time because they're not
	// dependent on the user value, indexed by key id.
	// For values that we don't know ahead of time, the
	// precalc entry contains a negative number n. (-n - 1) is the index
	// into the cache slice which will hold the value eventually
	// when it's been calculated for a particular user.
	precalc []valueID

	// keysWithRules holds the number of keys that have
	// rules (i.e. that do not have a value in precalc).
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

func parseConfig(jsonBody []byte, etag string, fetchTime time.Time, logger *leveledLogger, defaultUser User, overrides *FlagOverrides) (*config, error) {
	var root wireconfig.RootNode
	if err := json.Unmarshal(jsonBody, &root); err != nil {
		return nil, err
	}
	if overrides != nil {
		mergeEntriesWithOverrides(root.Entries, overrides.entries, overrides.Behaviour)
	}
	conf := &config{
		jsonBody:    jsonBody,
		root:        &root,
		evaluators:  new(sync.Map),
		keyValues:   keyValuesForRootNode(&root),
		allKeys:     keysForRootNode(&root),
		etag:        etag,
		fetchTime:   fetchTime,
		precalc:     make([]valueID, numKeys()),
		defaultUser: defaultUser,
	}
	conf.fixup(make(map[interface{}]valueID))
	conf.precalculate()
	conf.defaultUserSnapshot = _newSnapshot(conf, defaultUser, logger)
	return conf, nil
}

// precalculate populates conf.precalc with value IDs that
// are known ahead of time or negative indexes into the cache slice
// where the value will be stored later.
func (conf *config) precalculate() {
	for name, entry := range conf.root.Entries {
		id := idForKey(name, true)
		if int(id) >= len(conf.precalc) {
			precalc1 := make([]valueID, id+1)
			copy(precalc1, conf.precalc)
			conf.precalc = precalc1
		}
		if len(entry.RolloutRules) == 0 && len(entry.PercentageRules) == 0 {
			conf.precalc[id] = entry.ValueID
			continue
		}
		conf.keysWithRules++
		conf.precalc[id] = -valueID(conf.keysWithRules)
	}
}

func (conf *config) equal(c1 *config) bool {
	if conf == c1 || conf == nil || c1 == nil {
		return conf == c1
	}
	return conf.fetchTime.Equal(c1.fetchTime) && conf.etag == c1.etag && bytes.Equal(conf.jsonBody, c1.jsonBody)
}

func (conf *config) equalContent(c1 *config) bool {
	if conf == c1 || conf == nil || c1 == nil {
		return conf == c1
	}
	return bytes.Equal(conf.jsonBody, c1.jsonBody)
}

func (conf *config) withFetchTime(t time.Time) *config {
	c1 := *conf
	c1.fetchTime = t
	return &c1
}

func (conf *config) body() string {
	if conf == nil {
		return ""
	}
	return string(conf.jsonBody)
}

func (conf *config) getKeyAndValueForVariation(variationID string) (string, interface{}) {
	if conf == nil {
		return "", nil
	}
	kv := conf.keyValues[variationID]
	return kv.key, kv.value
}

func (conf *config) keys() []string {
	if conf == nil {
		return nil
	}
	return conf.allKeys
}

// fixup makes sure that int-valued entries have integer values
// and populates the ValueID fields in conf.root.
func (conf *config) fixup(valueMap map[interface{}]valueID) {
	for _, entry := range conf.root.Entries {
		entry.Value = fixValue(entry.Value, entry.Type)
		entry.ValueID = conf.idForValue(entry.Value, valueMap)
		for _, rule := range entry.RolloutRules {
			rule.Value = fixValue(rule.Value, entry.Type)
			rule.ValueID = conf.idForValue(rule.Value, valueMap)
		}
		for _, rule := range entry.PercentageRules {
			rule.Value = fixValue(rule.Value, entry.Type)
			rule.ValueID = conf.idForValue(rule.Value, valueMap)
		}
	}
}

func (conf *config) idForValue(v interface{}, valueMap map[interface{}]valueID) valueID {
	if id, ok := valueMap[v]; ok {
		return id
	}
	// Start at 1 so the zero value always means "not known yet"
	// so we can rely on zero-initialization of the evaluation context.
	id := valueID(len(conf.values) + 1)
	valueMap[v] = id
	conf.values = append(conf.values, v)
	return id
}

// fixValue fixes up int-valued entries, which will have the wrong type of value, so
// change them from float64 to int.
func fixValue(v interface{}, typ wireconfig.EntryType) interface{} {
	if typ != wireconfig.IntEntry {
		return v
	}
	f, ok := v.(float64)
	if !ok {
		// Shouldn't happen, but avoid a panic.
		return v
	}
	return int(f)
}

func mergeEntriesWithOverrides(remoteEntries map[string]*wireconfig.Entry, localEntries map[string]*wireconfig.Entry, behaviour OverrideBehaviour) {
	for key, entry := range localEntries {
		if behaviour == LocalOverRemote {
			remoteEntries[key] = entry
		} else if behaviour == RemoteOverLocal {
			if _, ok := remoteEntries[key]; !ok {
				remoteEntries[key] = entry
			}
		}
	}
}
