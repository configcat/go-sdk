package configcat

import (
	"context"
	"fmt"
	"strings"
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

var loggingTests = []struct {
	testName    string
	config      *rootNode
	key         string
	user        User
	expectValue interface{}
	expectLogs  []string
}{{
	testName:    "NoRules",
	config:      rootNodeWithKeyValue("key", "value", stringEntry),
	key:         "key",
	expectValue: "value",
	expectLogs: []string{
		"INFO: fetching from $HOST_URL",
		"INFO: Returning value.",
	},
}, {
	testName: "RolloutRulesButNoUser",
	config: &rootNode{
		Entries: map[string]*entry{
			"key": {
				Value: "defaultValue",
				Type:  stringEntry,
				RolloutRules: []*rolloutRule{{
					Value:               "e",
					ComparisonAttribute: "attr",
					ComparisonValue:     "x",
					Comparator:          opContains,
				}},
			},
		},
	},
	key:         "key",
	expectValue: "defaultValue",
	expectLogs: []string{
		"INFO: fetching from $HOST_URL",
		"WARN: Evaluating GetValue(key). UserObject missing! You should pass a UserObject to GetValueForUser() in order to make targeting work properly. Read more: https://configcat.com/docs/advanced/user-object.",
		"INFO: Returning defaultValue.",
	},
}, {
	testName: "RolloutRulesWithUser",
	config: &rootNode{
		Entries: map[string]*entry{
			"key": {
				Value: "defaultValue",
				Type:  stringEntry,
				RolloutRules: []*rolloutRule{{
					Value:               "v1",
					ComparisonAttribute: "Identifier",
					Comparator:          opContains,
					ComparisonValue:     "x",
				}, {
					Value:               "v2",
					ComparisonAttribute: "Identifier",
					Comparator:          opContains,
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
		"INFO: fetching from $HOST_URL",
		"INFO: Evaluating rule: [Identifier:y] [CONTAINS] [x] => no match",
		"INFO: Evaluating rule: [Identifier:y] [CONTAINS] [y] => match, returning: v2",
	},
}, {
	testName: "PercentageRulesButNoUser",
	config: &rootNode{
		Entries: map[string]*entry{
			"key": {
				Value: "defaultValue",
				Type:  stringEntry,
				PercentageRules: []percentageRule{{
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
		"INFO: fetching from $HOST_URL",
		"WARN: Evaluating GetValue(key). UserObject missing! You should pass a UserObject to GetValueForUser() in order to make targeting work properly. Read more: https://configcat.com/docs/advanced/user-object.",
		"INFO: Returning defaultValue.",
	},
}, {
	testName: "PercentageRulesWithUser",
	config: &rootNode{
		Entries: map[string]*entry{
			"key": {
				Value: "defaultValue",
				Type:  stringEntry,
				PercentageRules: []percentageRule{{
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
		"INFO: fetching from $HOST_URL",
		"INFO: Evaluating % options. Returning high-percent",
	},
}, {
	testName: "MatchErrorInUser",
	config: &rootNode{
		Entries: map[string]*entry{
			"key": {
				Value: "defaultValue",
				Type:  stringEntry,
				RolloutRules: []*rolloutRule{{
					Value:               "e",
					ComparisonAttribute: "Identifier",
					ComparisonValue:     "1.2.3",
					Comparator:          opLessSemver,
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
		"INFO: fetching from $HOST_URL",
		"INFO: Evaluating rule: [Identifier:bogus] [< (SemVer)] [1.2.3] => SKIP rule. Validation error: No Major.Minor.Patch elements found",
		"INFO: Returning defaultValue.",
	},
}, {
	testName: "MatchErrorRules",
	config: &rootNode{
		Entries: map[string]*entry{
			"key": {
				Value: "defaultValue",
				Type:  stringEntry,
				RolloutRules: []*rolloutRule{{
					Value:               "e",
					ComparisonAttribute: "Identifier",
					ComparisonValue:     "bogus",
					Comparator:          opLessSemver,
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
		"INFO: fetching from $HOST_URL",
		"INFO: Evaluating rule: [Identifier:1.2.3] [< (SemVer)] [bogus] => SKIP rule. Validation error: No Major.Minor.Patch elements found",
		"INFO: Returning defaultValue.",
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

			expectLogs := append([]string(nil), test.expectLogs...)
			for i := range expectLogs {
				expectLogs[i] = strings.ReplaceAll(expectLogs[i], "$HOST_URL", cfg.BaseURL)
			}

			value := client.GetStringValue(test.key, "", test.user)
			c.Check(value, qt.Equals, test.expectValue)
			c.Check(logs, qt.DeepEquals, expectLogs)
		})
	}
}
