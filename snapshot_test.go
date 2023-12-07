package configcat

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNilSnapshot(t *testing.T) {
	c := qt.New(t)
	var snap *Snapshot
	c.Assert(snap.WithUser(nil), qt.IsNil)
	c.Assert(snap.GetValueDetails("xxx").Data.VariationID, qt.Equals, "")
	c.Assert(snap.GetValue("xxx"), qt.Equals, nil)
	key, val := snap.GetKeyValueForVariationID("xxx")
	c.Assert(key, qt.Equals, "")
	c.Assert(val, qt.Equals, nil)
	c.Assert(snap.GetAllValueDetails(), qt.IsNil)
	c.Assert(snap.GetAllKeys(), qt.IsNil)

	// Test one flag type as proxy for the others, as they all use
	// the same underlying mechanism.
	c.Assert(Bool("hello", true).Get(nil), qt.Equals, true)
}

func TestNewSnapshot(t *testing.T) {
	c := qt.New(t)
	// Make sure there's another flag in there so even when we run
	// the test on its own, we're still testing the case where the
	// flag ids don't start from zero.
	Bool("something", false)
	values := map[string]interface{}{
		"intFlag":    1,
		"floatFlag":  2.0,
		"stringFlag": "three",
		"boolFlag":   true,
	}
	snap, err := NewSnapshot(newTestLogger(t), values)
	c.Assert(err, qt.IsNil)
	for key, want := range values {
		c.Check(snap.GetValue(key), qt.Equals, want)
	}
	// Sanity check that it works OK with Flag values.
	c.Assert(Int("intFlag", 0).Get(snap), qt.Equals, 1)
	c.Assert(Float("floatFlag", 0).Get(snap), qt.Equals, 2.0)
	c.Assert(String("stringFlag", "").Get(snap), qt.Equals, "three")
	c.Assert(Bool("boolFlag", false).Get(snap), qt.Equals, true)
	c.Assert(snap.GetAllKeys(), qt.ContentEquals, []string{
		"intFlag",
		"floatFlag",
		"stringFlag",
		"boolFlag",
	})
	c.Assert(snap.GetValueDetails("intFlag").Data.VariationID, qt.Equals, "")
	id, val := snap.GetKeyValueForVariationID("")
	c.Assert(id, qt.Equals, "")
	c.Assert(val, qt.Equals, nil)

	c.Assert(snap.WithUser(&UserData{}), qt.Equals, snap)
}

func TestNewSnapshotWithUnknownType(t *testing.T) {
	c := qt.New(t)
	snap, err := NewSnapshot(newTestLogger(t), map[string]interface{}{
		"badVal": int64(1),
	})
	c.Check(err, qt.ErrorMatches, `value for flag "badVal" has unexpected type int64 \(1\); must be bool, int, float64 or string`)
	c.Check(snap, qt.IsNil)
}

func TestFlagKey(t *testing.T) {
	c := qt.New(t)
	intFlag := Int("intFlag", 99)
	c.Assert(intFlag.Get(nil), qt.Equals, 99)
	c.Assert(intFlag.Key(), qt.Equals, "intFlag")

	floatFlag := Float("floatFlag", 2.5)
	c.Assert(floatFlag.Get(nil), qt.Equals, 2.5)
	c.Assert(floatFlag.Key(), qt.Equals, "floatFlag")

	stringFlag := String("stringFlag", "default")
	c.Assert(stringFlag.Get(nil), qt.Equals, "default")
	c.Assert(stringFlag.Key(), qt.Equals, "stringFlag")

	boolFlag := Bool("boolFlag", true)
	c.Assert(boolFlag.Get(nil), qt.Equals, true)
	c.Assert(boolFlag.Key(), qt.Equals, "boolFlag")
}
