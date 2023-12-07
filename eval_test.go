package configcat

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestOpOneOfWithStringValue(t *testing.T) {
	c := qt.New(t)
	ectx := newEvalTestContext(c)

	for _, s := range []string{"", "hello", "x"} {
		for _, user := range stringVariants(s) {
			c.Run(fmt.Sprintf("%#v-%v", user, s), func(c *qt.C) {
				for _, test := range stringOneOfTests(s) {
					test.run(c, ectx, user)
				}
			})
		}
	}
}

type typedString string

func TestOpCmpNumWithNumericValue(t *testing.T) {
	c := qt.New(t)
	ectx := newEvalTestContext(c)

	for _, op := range []Comparator{
		OpEqNum,
		OpLessNum,
		OpLessEqNum,
		OpGreaterNum,
		OpGreaterEqNum,
	} {
		for _, tv := range []interface{}{int8(0), int16(0), int32(0), int64(0), uint8(0), uint16(0), uint32(0), uint64(0), float32(0), float64(0)} {
			t := reflect.TypeOf(tv)
			lo, hi := numLimits(t)
			xnames := []string{
				"lowest",
				"highest",
				"zero",
			}
			for i, x := range []float64{lo, hi, 0} {
				c.Run(fmt.Sprintf("%v-%s-%v", op, t, xnames[i]), func(c *qt.C) {
					v := reflect.New(t).Elem()
					setValueFromFloat(v, x)
					user := newTestStruct(v)
					for _, test := range numericCmpNumTests(x, op, cmpFunc(op)) {
						test.run(c, ectx, user)
					}
				})
			}
		}
	}
}

func TestOpCmpNumWithInvalidCmpVal(t *testing.T) {
	c := qt.New(t)
	ectx := newEvalTestContext(c)

	for _, op := range []Comparator{
		OpEqNum,
		OpLessNum,
		OpLessEqNum,
		OpGreaterNum,
		OpGreaterEqNum,
	} {
		for _, tv := range []interface{}{int8(0), int16(0), int32(0), int64(0), uint8(0), uint16(0), uint32(0), uint64(0), float32(0), float64(0)} {
			(&opTest{
				testName: fmt.Sprintf("%T-%v", tv, op),
				op:       op,
				cmpVal:   "badnum",
				want:     false,
			}).run(c, ectx, newTestStruct(reflect.ValueOf(tv)))
		}
	}
}

func TestOpCmpNumWithStringValue(t *testing.T) {
	c := qt.New(t)
	ectx := newEvalTestContext(c)

	tests := []struct {
		s      string
		op     Comparator
		cmpVal string
		want   bool
	}{
		{"1.5", OpEqNum, "1.5", true},
		{"1.5", OpEqNum, "1.6", false},
		{"", OpEqNum, "0", false},
		{"1.5", OpNotEqNum, "1.5", false},
		{"1.5", OpLessNum, "1.6", true},
		{"1.6", OpLessNum, "1.5", false},
		{"1.5", OpLessNum, "1.5", false},
		{"1.5", OpLessEqNum, "1.5", true},
		{"1.5", OpLessEqNum, "1.6", true},
		{"1.6", OpLessEqNum, "1.5", false},
		// Invalid numbers always compare false.
		{"bad", OpEqNum, "1.5", false},
		{"bad", OpNotEqNum, "1.5", false},
		{"bad", OpLessNum, "1.5", false},
		{"bad", OpLessEqNum, "1.5", false},
		{"bad", OpGreaterNum, "1.5", false},
		{"bad", OpGreaterEqNum, "1.5", false},
	}
	for _, test := range tests {
		s := test.s
		for _, user := range stringVariants(test.s) {
			(&opTest{
				testName: fmt.Sprintf("%#v-%q-%v-%q", user, s, test.op, test.cmpVal),
				op:       test.op,
				cmpVal:   test.cmpVal,
				want:     test.want,
			}).run(c, ectx, user)
		}
	}
}

func TestOpSemverWithNonStringDoesNotMatch(t *testing.T) {
	c := qt.New(t)
	ectx := newEvalTestContext(c)
	// A non-string semver never satisfies a semver condition.
	user := &struct {
		X int
	}{1}
	for _, op := range []Comparator{
		OpOneOfSemver,
		OpNotOneOfSemver,
		OpLessSemver,
		OpLessEqSemver,
		OpGreaterSemver,
		OpGreaterEqSemver,
	} {
		(&opTest{
			testName: op.String(),
			op:       op,
			cmpVal:   "1.0.0",
			want:     false,
		}).run(c, ectx, user)
	}
}

func TestOpSemverWithString(t *testing.T) {
	c := qt.New(t)
	ectx := newEvalTestContext(c)

	tests := []struct {
		s      string
		op     Comparator
		cmpVal string
		want   bool
	}{
		{"1.5.0", OpOneOfSemver, "1.5.0", true},
		{"1.5.0", OpOneOfSemver, "1.5.0,1.5.1", true},
		{"1.5.0", OpOneOfSemver, "1.5.1,1.5.2", false},
		{"1.5.0", OpOneOfSemver, "   1.5.0  ,1.5.2   ", true},
		{"1.5.0", OpNotOneOfSemver, "1.5.0", false},
		{"1.5.0", OpNotOneOfSemver, "1.5.0,1.5.1", false},
		{"1.5.0", OpNotOneOfSemver, "1.5.1,1.5.2", true},
		{"1.5.0", OpNotOneOfSemver, "   1.5.0  ,1.5.2   ", false},
		{"1.4.0", OpLessSemver, "1.5.0", true},
		{"1.4.0", OpLessSemver, " 1.5.0 ", true},
		{"1.4.0", OpLessSemver, "1.4.0", false},
		{"1.4.0", OpLessSemver, "1.3.0", false},
		{"1.4.0", OpGreaterSemver, "1.5.0", false},
		{"1.4.0", OpGreaterSemver, "1.4.0", false},
		{"1.4.0", OpGreaterSemver, "1.3.0", true},
		{"1.4.0", OpLessEqSemver, "1.5.0", true},
		{"1.4.0", OpLessEqSemver, "1.4.0", true},
		{"1.4.0", OpLessEqSemver, "1.3.0", false},
		{"1.4.0", OpGreaterEqSemver, "1.5.0", false},
		{"1.4.0", OpGreaterEqSemver, "1.4.0", true},
		{"1.4.0", OpGreaterEqSemver, "1.3.0", true},
	}
	for _, test := range tests {
		s := test.s
		for _, user := range stringVariants(s) {
			(&opTest{
				testName: fmt.Sprintf("%#v-%q-%v-%q", user, s, test.op, test.cmpVal),
				op:       test.op,
				cmpVal:   test.cmpVal,
				want:     test.want,
			}).run(c, ectx, user)
		}
	}
}

func TestNoUser(t *testing.T) {
	c := qt.New(t)
	ectx := newEvalTestContext(c)
	(&opTest{
		testName: "nil-interface",
		op:       OpOneOf,
		cmpVal:   "foo",
		want:     false,
	}).run(c, ectx, nil)
	(&opTest{
		testName: "nil-struct",
		op:       OpOneOf,
		cmpVal:   "foo",
		want:     false,
	}).run(c, ectx, (*struct{ X string })(nil))
}

func TestNonPointerUserStruct(t *testing.T) {
	c := qt.New(t)
	c.Skip("this is an awkward case that we need to think about")
	ectx := newEvalTestContext(c)
	(&opTest{
		testName: "nil-struct",
		op:       OpOneOf,
		cmpVal:   "foo",
		want:     false,
	}).run(c, ectx, struct{ X string }{})
}

func stringVariants(s string) []User {
	vs := []interface{}{
		s,
		typedString(s),
		[]byte(s),
	}
	var us []User
	for _, v := range vs {
		us = append(us, newTestStruct(reflect.ValueOf(v)))
	}
	us = append(us,
		&UserData{
			Custom: map[string]interface{}{
				"X": s,
			},
		},
		&attributeGetter{
			v: s,
		})
	return us
}

type attributeGetter struct {
	v string
}

func (g *attributeGetter) GetAttribute(attr string) interface{} {
	if attr == "X" {
		return g.v
	}
	return ""
}

// opTest represents a test for a particular operator with respect to some
// user field value.
type opTest struct {
	testName string
	// op is the operator to test.
	op Comparator
	// cmpVal is the argument to the operator.
	cmpVal string
	// want holds the expected result of the test.
	want bool
}

func (test *opTest) run(c *qt.C, ectx *evalTestContext, user User) {
	c.Run(test.testName, func(c *qt.C) {
		ectx.logger.t = c
		c.Logf("operator %v; cmpVal %v; want %v", test.op, test.cmpVal, test.want)
		cond := &UserCondition{
			ComparisonAttribute: "X",
			Comparator:          test.op,
		}
		if test.op.IsList() {
			if strings.Contains(test.cmpVal, ",") {
				split := strings.Split(test.cmpVal, ",")
				cond.StringArrayValue = split
			} else {
				cond.StringArrayValue = []string{test.cmpVal}
			}
		} else if test.op.IsNumeric() {
			f, err := parseFloat(strings.TrimSpace(test.cmpVal))
			if err == nil {
				cond.DoubleValue = &f
			}
		} else {
			cond.StringValue = &test.cmpVal
		}
		ectx.srv.setResponseJSON(&ConfigJson{
			Settings: map[string]*Setting{
				"key": {
					Type:        StringSetting,
					VariationID: "testFallback",
					Value:       &SettingValue{StringValue: "false"},
					TargetingRules: []*TargetingRule{{
						Conditions: []*Condition{{
							UserCondition: cond,
						}},
						ServedValue: &ServedValue{
							VariationID: "test",
							Value:       &SettingValue{StringValue: "true"},
						},
					}},
				},
			},
		})
		ectx.client.Refresh(context.Background())
		want := "false"
		if test.want {
			want = "true"
		}
		c.Check(ectx.client.GetStringValue("key", "", user), qt.Equals, want, qt.Commentf("user: %#v", user))
	})
}

// stringOneOfTests returns a set of tests for opOneOf and
// opNotOneOf given a user value of s.
func stringOneOfTests(s string) []opTest {
	other := "x"
	if other == s {
		other = "y"
	}
	tests := []opTest{{
		testName: "exact-value",
		op:       OpOneOf,
		cmpVal:   s,
		want:     true,
	}, {
		testName: "with-extra-value",
		op:       OpOneOf,
		cmpVal:   s + "," + other,
		want:     true,
	}, {
		testName: "with-appended-value",
		op:       OpOneOf,
		cmpVal:   s + "x",
		want:     false,
	}, {
		testName: "empty-string",
		op:       OpOneOf,
		cmpVal:   "",
		want:     false,
	}}
	// Add tests for opNotOneOf.
	for _, test := range tests {
		tests = append(tests, opTest{
			testName: "not(" + test.testName + ")",
			op:       OpNotOneOf,
			cmpVal:   test.cmpVal,
			want:     !test.want,
		})
	}
	// When the comparison string is empty, all comparisons are false.
	if s == "" {
		for i := range tests {
			tests[i].want = false
		}
	}
	return tests
}

// numericCmpNumTests returns a set of tests for the
// comparison operator op given a User field with value x.
// cmp reports the result of the comparison operator.
func numericCmpNumTests(x float64, op Comparator, cmp func(a, b float64) bool) []opTest {
	tests := []opTest{{
		testName: "exact-value",
		op:       op,
		cmpVal:   fmt.Sprint(x),
		want:     cmp(x, x),
	}}
	if !math.IsInf(x, 0) {
		tests = append(tests, []opTest{{
			testName: "small-increment",
			op:       op,
			cmpVal:   fmt.Sprint(addSomethingSmall(x)),
			want:     cmp(x, addSomethingSmall(x)),
		}, {
			testName: "small-decrement",
			op:       op,
			cmpVal:   fmt.Sprint(subSomethingSmall(x)),
			want:     cmp(x, subSomethingSmall(x)),
		}, {
			testName: "double",
			op:       op,
			cmpVal:   fmt.Sprint(x * 2),
			want:     cmp(x, x*2),
		}, {
			testName: "half",
			op:       op,
			cmpVal:   fmt.Sprint(x / 2),
			want:     cmp(x, x/2),
		}}...)
	}
	if x != 0 {
		tests = append(tests, opTest{
			testName: "negate",
			op:       op,
			cmpVal:   fmt.Sprint(-x),
			want:     cmp(x, -x),
		})
	}
	for _, test := range tests {
		// commas are treated as decimal points.
		if strings.Contains(test.cmpVal, ".") {
			tests = append(tests, opTest{
				testName: test.testName + "-commas",
				op:       op,
				cmpVal:   strings.ReplaceAll(test.cmpVal, ".", ","),
				want:     test.want,
			})
		}
	}
	return tests
}

func addSomethingSmall(x float64) float64 {
	xinc := x
	for inc := 0.492342353422345; ; inc *= 2 {
		if xinc != x {
			return xinc
		}
		xinc = x + inc
	}
}

func subSomethingSmall(x float64) float64 {
	xinc := x
	for inc := -0.492342353422345; ; inc *= 2 {
		if xinc != x {
			return xinc
		}
		xinc = x + inc
	}
}

type evalTestContext struct {
	srv    *configServer
	client *Client
	logger *testLogger
}

func newEvalTestContext(c *qt.C) *evalTestContext {
	var ectx evalTestContext
	ectx.srv = newConfigServer(c)
	cfg := ectx.srv.config()
	cfg.PollingMode = Manual
	ectx.logger = newTestLogger(c).(*testLogger)
	cfg.Logger = ectx.logger
	cfg.LogLevel = LogLevelDebug
	ectx.client = NewCustomClient(cfg)
	return &ectx
}

// newTestStruct returns a struct with field X holding v.
func newTestStruct(v reflect.Value) User {
	userv := reflect.New(reflect.StructOf([]reflect.StructField{{
		Name: "X",
		Type: v.Type(),
	}}))
	userv.Elem().Field(0).Set(v)
	return userv.Interface()
}

// numLimits returns the numeric limits of the given numeric type.
// We don't return the actual limits for 64 bit types because
// there are too many unresolvable issues at those values.
func numLimits(t reflect.Type) (float64, float64) {
	switch t.Kind() {
	case reflect.Float64:
		return -1e30, 1e30
	case reflect.Float32:
		return -math.MaxFloat32, math.MaxFloat32
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		bits := uint(8 * t.Size())
		if bits == 64 {
			// Cop out of rounding errors.
			bits = 52
		}
		lim := int64(1 << (bits - 1))
		return float64(-lim), float64(lim - 1)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		bits := uint(8 * t.Size())
		if bits == 64 {
			// Cop out of rounding errors.
			bits = 52
		}
		return 0, float64(int64(1<<bits) - 1)
	default:
		panic(fmt.Errorf("unhandled type %v", t))
	}
}

func setValueFromFloat(v reflect.Value, f float64) {
	switch v.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(f))
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(f))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(f)
	default:
		panic(fmt.Errorf("unhandled type %v", v.Type()))
	}
}

func cmpFunc(op Comparator) func(a, b float64) bool {
	return func(a, b float64) bool {
		switch op {
		case OpEqNum:
			return a == b
		case OpLessNum:
			return a < b
		case OpLessEqNum:
			return a <= b
		case OpGreaterNum:
			return a > b
		case OpGreaterEqNum:
			return a >= b
		default:
			panic(fmt.Errorf("unknown comparison operator %v", op))
		}
	}
}
