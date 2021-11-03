package configcat

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNilSnapshot(t *testing.T) {
	c := qt.New(t)
	var snap *Snapshot
	c.Assert(snap.WithUser(nil), qt.IsNil)
	c.Assert(snap.GetVariationID("xxx"), qt.Equals, "")
	c.Assert(snap.GetValue("xxx"), qt.Equals, nil)
	key, val := snap.GetKeyValueForVariationID("xxx")
	c.Assert(key, qt.Equals, "")
	c.Assert(val, qt.Equals, nil)
	c.Assert(snap.GetVariationIDs(), qt.IsNil)
	c.Assert(snap.GetAllKeys(), qt.IsNil)

	// Test one flag type as proxy for the others, as they all use
	// the same underlying mechanism.
	c.Assert(Bool("hello", true).Get(nil), qt.Equals, true)
}

func TestNewSnapshot(t *testing.T) {
	c := qt.New(t)
	values := map[string]interface{}{
		"intFlag":    1,
		"floatFlag":  2.0,
		"stringFlag": "three",
		"boolFlag":   true,
	}
	snap, err := NewSnapshot(newTestLogger(t, LogLevelDebug), values)
	c.Assert(err, qt.IsNil)
	for key, want := range values {
		c.Assert(snap.GetValue(key), qt.Equals, want)
	}
	// Sanity check that it works OK with Flag values.
	c.Assert(Int("intFlag", 0).Get(snap), qt.Equals, 1)
}

func TestNewSnapshotWithUnknownType(t *testing.T) {
	c := qt.New(t)
	snap, err := NewSnapshot(newTestLogger(t, LogLevelDebug), map[string]interface{}{
		"badVal": int64(1),
	})
	c.Check(err, qt.ErrorMatches, `value for flag "badVal" has unexpected type int64 \(1\); must be bool, int, float64 or string`)
	c.Check(snap, qt.IsNil)
}
