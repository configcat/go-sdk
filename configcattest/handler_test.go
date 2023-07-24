package configcattest_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	configcat "github.com/configcat/go-sdk/v8"
	"github.com/configcat/go-sdk/v8/configcattest"
	qt "github.com/frankban/quicktest"
)

func TestHandlerSimple(t *testing.T) {
	c := qt.New(t)
	k := configcattest.RandomSDKKey()
	var h configcattest.Handler
	err := h.SetFlags(k, map[string]*configcattest.Flag{
		"intflag": {
			Default: 99,
		},
		"floatflag": {
			Default: 100.0,
		},
		"stringflag": {
			Default: "s",
		},
		"boolflag": {
			Default: true,
		},
	})
	c.Assert(err, qt.IsNil)
	srv := httptest.NewServer(&h)
	defer srv.Close()
	client := configcat.NewCustomClient(configcat.Config{
		BaseURL: srv.URL,
		SDKKey:  k,
	})
	defer client.Close()
	c.Assert(configcat.Int("intflag", 1).Get(client.Snapshot(nil)), qt.Equals, 99)
	c.Assert(configcat.Int("otherflag", 1).Get(client.Snapshot(nil)), qt.Equals, 1)

	c.Assert(configcat.Float("floatflag", 1.0).Get(client.Snapshot(nil)), qt.Equals, 100.0)
	c.Assert(configcat.String("stringflag", "").Get(client.Snapshot(nil)), qt.Equals, "s")
	c.Assert(configcat.Bool("boolflag", false).Get(client.Snapshot(nil)), qt.Equals, true)
}

func TestHandlerWithUser(t *testing.T) {
	c := qt.New(t)
	k := configcattest.RandomSDKKey()
	var h configcattest.Handler
	err := h.SetFlags(k, map[string]*configcattest.Flag{
		"someflag": {
			Default: 99,
			Rules: []configcattest.Rule{{
				ComparisonAttribute: "foo",
				Comparator:          configcattest.OpOneOf,
				ComparisonValue:     "something",
				Value:               88,
			}, {
				ComparisonAttribute: "foo",
				Comparator:          configcattest.OpContains,
				ComparisonValue:     "xxx",
				Value:               77,
			}},
		},
	})
	c.Assert(err, qt.IsNil)
	srv := httptest.NewServer(&h)
	defer srv.Close()
	client := configcat.NewCustomClient(configcat.Config{
		BaseURL: srv.URL,
		SDKKey:  k,
	})
	defer client.Close()
	flag := configcat.Int("someflag", 1)
	c.Assert(flag.Get(client.Snapshot(nil)), qt.Equals, 99)

	type user struct {
		Foo string `configcat:"foo"`
	}
	c.Assert(flag.Get(client.Snapshot(&user{
		Foo: "something",
	})), qt.Equals, 88)
	c.Assert(flag.Get(client.Snapshot(&user{
		Foo: "blahxxxblah",
	})), qt.Equals, 77)
	c.Assert(flag.Get(client.Snapshot(&user{
		Foo: "other",
	})), qt.Equals, 99)
}

var invalidFlagsTests = []struct {
	testName    string
	flag        *configcattest.Flag
	expectError string
}{{
	testName: "InvalidDefault",
	flag: &configcattest.Flag{
		Default: byte(1),
	},
	expectError: `invalid flag "foo": invalid type uint8 for default value 0x1`,
}, {
	testName: "InvalidComparator",
	flag: &configcattest.Flag{
		Default: 1,
		Rules: []configcattest.Rule{{
			ComparisonAttribute: "x",
			Comparator:          200,
			ComparisonValue:     "y",
			Value:               2,
		}},
	},
	expectError: `invalid flag "foo": invalid comparator value 200`,
}, {
	testName: "EmptyComparisonAttribute",
	flag: &configcattest.Flag{
		Default: 1,
		Rules: []configcattest.Rule{{
			ComparisonAttribute: "",
			Comparator:          configcattest.OpContains,
			ComparisonValue:     "y",
			Value:               2,
		}},
	},
	expectError: `invalid flag "foo": empty comparison attribute`,
}, {
	testName: "EmptyComparisonAttribute",
	flag: &configcattest.Flag{
		Default: 1,
		Rules: []configcattest.Rule{{
			ComparisonAttribute: "x",
			Comparator:          configcattest.OpContains,
			ComparisonValue:     "",
		}},
	},
	expectError: `invalid flag "foo": empty comparison value`,
}, {
	testName: "InconsistentRuleType",
	flag: &configcattest.Flag{
		Default: 1,
		Rules: []configcattest.Rule{{
			ComparisonAttribute: "x",
			Comparator:          configcattest.OpContains,
			ComparisonValue:     "y",
			Value:               "x",
		}},
	},
	expectError: `invalid flag "foo": rule value for rule \("x" CONTAINS "y"\) has inconsistent type string \(value "x"\) with flag default value 1`,
}}

func TestInvalidFlags(t *testing.T) {
	c := qt.New(t)
	k := configcattest.RandomSDKKey()
	for _, test := range invalidFlagsTests {
		c.Run(test.testName, func(c *qt.C) {
			var h configcattest.Handler
			err := h.SetFlags(k, map[string]*configcattest.Flag{
				"foo": test.flag,
			})
			c.Assert(err, qt.ErrorMatches, test.expectError)
		})
	}
}

func TestHandlerAddInvalidSDKKey(t *testing.T) {
	c := qt.New(t)
	var h configcattest.Handler
	err := h.SetFlags("", nil)
	c.Assert(err, qt.ErrorMatches, `empty SDK key passed to configcattest.Handler.SetFlags`)
}

func TestHandlerSDKKeyNotFound(t *testing.T) {
	c := qt.New(t)
	var h configcattest.Handler
	srv := httptest.NewServer(&h)
	defer srv.Close()
	client := configcat.NewCustomClient(configcat.Config{
		BaseURL: srv.URL,
		SDKKey:  configcattest.RandomSDKKey(),
	})
	defer client.Close()
	c.Assert(configcat.Int("i", 20).Get(client.Snapshot(nil)), qt.Equals, 20)
}

func TestHandlerWrongMethod(t *testing.T) {
	c := qt.New(t)
	var h configcattest.Handler
	srv := httptest.NewServer(&h)
	defer srv.Close()

	resp, err := http.Post(srv.URL, "", strings.NewReader("x"))
	c.Assert(err, qt.IsNil)
	resp.Body.Close()
	c.Assert(resp.StatusCode, qt.Equals, http.StatusMethodNotAllowed)
}
