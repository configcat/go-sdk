package configcat

import (
	"context"
	"fmt"
	"github.com/configcat/go-sdk/v8/internal/wireconfig"
	"strings"
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

var loggingTests = []struct {
	testName    string
	config      *wireconfig.RootNode
	key         string
	user        User
	expectValue interface{}
	expectLogs  []string
}{{
	testName:    "NoRules",
	config:      rootNodeWithKeyValue("key", "value", wireconfig.StringEntry),
	key:         "key",
	expectValue: "value",
	expectLogs: []string{
		"INFO: [5000] returning key=value",
	},
}, {
	testName: "RolloutRulesButNoUser",
	config: &wireconfig.RootNode{
		Entries: map[string]*wireconfig.Entry{
			"key": {
				Value: "defaultValue",
				Type:  wireconfig.StringEntry,
				RolloutRules: []*wireconfig.RolloutRule{{
					Value:               "e",
					ComparisonAttribute: "attr",
					ComparisonValue:     "x",
					Comparator:          wireconfig.OpContains,
				}},
			},
		},
	},
	key:         "key",
	expectValue: "defaultValue",
	expectLogs: []string{
		"WARN: [3001] cannot evaluate targeting rules and % options for setting 'key' (User Object is missing); you should pass a User Object to the evaluation methods like `GetValue()` in order to make targeting work properly; read more: https://configcat.com/docs/advanced/user-object/",
		"INFO: [5000] returning key=defaultValue",
	},
}, {
	testName: "RolloutRulesWithUser",
	config: &wireconfig.RootNode{
		Entries: map[string]*wireconfig.Entry{
			"key": {
				Value: "defaultValue",
				Type:  wireconfig.StringEntry,
				RolloutRules: []*wireconfig.RolloutRule{{
					Value:               "v1",
					ComparisonAttribute: "Identifier",
					Comparator:          wireconfig.OpContains,
					ComparisonValue:     "x",
				}, {
					Value:               "v2",
					ComparisonAttribute: "Identifier",
					Comparator:          wireconfig.OpContains,
					ComparisonValue:     "y",
				}},
			},
		},
	},
	key: "key",
	user: &UserData{
		Identifier: "y",
	},
	expectValue: "v2",
	expectLogs: []string{
		"INFO: [5000] evaluating rule: [Identifier:y] [CONTAINS] [x] => no match",
		"INFO: [5000] evaluating rule: [Identifier:y] [CONTAINS] [y] => match",
		"INFO: [5000] returning key=v2",
	},
}, {
	testName: "PercentageRulesButNoUser",
	config: &wireconfig.RootNode{
		Entries: map[string]*wireconfig.Entry{
			"key": {
				Value: "defaultValue",
				Type:  wireconfig.StringEntry,
				PercentageRules: []*wireconfig.PercentageRule{{
					Value:      "low-percent",
					Percentage: 30,
				}, {
					Value:      "high-percent",
					Percentage: 70,
				}},
			},
		},
	},
	key:         "key",
	expectValue: "defaultValue",
	expectLogs: []string{
		"WARN: [3001] cannot evaluate targeting rules and % options for setting 'key' (User Object is missing); you should pass a User Object to the evaluation methods like `GetValue()` in order to make targeting work properly; read more: https://configcat.com/docs/advanced/user-object/",
		"INFO: [5000] returning key=defaultValue",
	},
}, {
	testName: "PercentageRulesWithUser",
	config: &wireconfig.RootNode{
		Entries: map[string]*wireconfig.Entry{
			"key": {
				Value: "defaultValue",
				Type:  wireconfig.StringEntry,
				PercentageRules: []*wireconfig.PercentageRule{{
					Value:      "low-percent",
					Percentage: 1,
				}, {
					Value:      "high-percent",
					Percentage: 99,
				}},
			},
		},
	},
	key: "key",
	user: &UserData{
		Identifier: "y",
	},
	expectValue: "high-percent",
	expectLogs: []string{
		"INFO: [5000] returning key=high-percent",
	},
}, {
	testName: "MatchErrorInUser",
	config: &wireconfig.RootNode{
		Entries: map[string]*wireconfig.Entry{
			"key": {
				Value: "defaultValue",
				Type:  wireconfig.StringEntry,
				RolloutRules: []*wireconfig.RolloutRule{{
					Value:               "e",
					ComparisonAttribute: "Identifier",
					ComparisonValue:     "1.2.3",
					Comparator:          wireconfig.OpLessSemver,
				}},
			},
		},
	},
	key:         "key",
	expectValue: "defaultValue",
	user: &UserData{
		Identifier: "bogus",
	},
	expectLogs: []string{
		"INFO: [5000] evaluating rule: [Identifier:bogus] [< (SemVer)] [1.2.3] => SKIP rule; validation error: No Major.Minor.Patch elements found",
		"INFO: [5000] returning key=defaultValue",
	},
}, {
	testName: "MatchErrorRules",
	config: &wireconfig.RootNode{
		Entries: map[string]*wireconfig.Entry{
			"key": {
				Value: "defaultValue",
				Type:  wireconfig.StringEntry,
				RolloutRules: []*wireconfig.RolloutRule{{
					Value:               "e",
					ComparisonAttribute: "Identifier",
					ComparisonValue:     "bogus",
					Comparator:          wireconfig.OpLessSemver,
				}},
			},
		},
	},
	key:         "key",
	expectValue: "defaultValue",
	user: &UserData{
		Identifier: "1.2.3",
	},
	expectLogs: []string{
		"INFO: [5000] evaluating rule: [Identifier:1.2.3] [< (SemVer)] [bogus] => SKIP rule; validation error: No Major.Minor.Patch elements found",
		"INFO: [5000] returning key=defaultValue",
	},
}, {
	testName: "UnknownKey",
	config: &wireconfig.RootNode{
		Entries: map[string]*wireconfig.Entry{
			"key1": {
				Value: "v1",
				Type:  wireconfig.StringEntry,
			},
			"key2": {
				Value: "v2",
				Type:  wireconfig.StringEntry,
			},
		},
	},
	key:         "unknownKey",
	expectValue: nil,
	expectLogs: []string{
		"ERROR: [1001] failed to evaluate setting 'unknownKey' (the key was not found in config JSON); available keys: ['key1', 'key2']",
	},
}}

func TestLogging(t *testing.T) {
	c := qt.New(t)
	for _, test := range loggingTests {
		c.Run(test.testName, func(c *qt.C) {
			var logs []string
			srv := newConfigServer(t)
			cfg := srv.config()
			cfg.PollingMode = Manual
			cfg.Logger = &testLogger{
				logFunc: func(f string, a ...interface{}) {
					s := fmt.Sprintf(f, a...)
					if !strings.HasPrefix(s, "DEBUG: ") {
						logs = append(logs, s)
					}
				},
				level: LogLevelInfo,
			}
			client := NewCustomClient(cfg)
			defer client.Close()
			srv.setResponseJSON(test.config)
			client.Refresh(context.Background())

			// We'll always get a "fetching from" message at the start.
			c.Check(logs, qt.DeepEquals, []string{"INFO: [0] fetching from " + cfg.BaseURL})
			logs = nil

			snap := client.Snapshot(test.user)
			// Run the test twice to make sure that caching doesn't
			// interfere with the logging.
			for i := 0; i < 2; i++ {
				c.Logf("iteration %d", i)
				logs = nil
				value := snap.GetValue(test.key)
				c.Check(value, qt.Equals, test.expectValue)
				c.Check(logs, qt.DeepEquals, test.expectLogs)
			}
		})
	}
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
	snap, err := NewSnapshot(newTestLogger(t, LogLevelDebug), values)
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
	snap, err := NewSnapshot(newTestLogger(t, LogLevelDebug), map[string]interface{}{
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
