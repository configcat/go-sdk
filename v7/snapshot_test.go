package configcat

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNilSnapshot(t *testing.T) {
	c := qt.New(t)
	var snap *Snapshot
	c.Assert(snap.WithUser(nil), qt.IsNil)
	c.Assert(snap.VariationID("xxx"), qt.Equals, "")
	c.Assert(snap.Get("xxx"), qt.Equals, nil)
	key, val := snap.KeyValueForVariationID("xxx")
	c.Assert(key, qt.Equals, "")
	c.Assert(val, qt.Equals, nil)
	c.Assert(snap.VariationIDs(), qt.IsNil)
	c.Assert(snap.Keys(), qt.IsNil)

	// Test one flag type as proxy for the others, as they all use
	// the same underlying mechanism.
	c.Assert(Bool("hello", true).Get(nil), qt.Equals, true)
}
