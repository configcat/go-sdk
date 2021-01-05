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

type customString struct {
	val string
}

func (s customString) String() string {
	return s.val
}

type customPtrString struct {
	val string
}

func (s *customPtrString) String() string {
	return s.val
}

func TestOpOneOfWithNumericValue(t *testing.T) {
	c := qt.New(t)
	ectx := newEvalTestContext(c)

	for _, tv := range []interface{}{int8(0), int16(0), int32(0), int64(0), uint8(0), uint16(0), uint32(0), uint64(0), float32(0), float64(0)} {
		t := reflect.TypeOf(tv)
		lo, hi := numLimits(t)
		xnames := []string{
			"lowest",
			"highest",
			"zero",
			"NaN",
			"+Infinity",
			"-Infinity",
		}
		for i, x := range []float64{lo, hi, 0, math.NaN(), math.Inf(1), math.Inf(-1)} {
			if (math.IsNaN(x) || math.IsInf(x, 0)) && t.Kind() != reflect.Float64 && t.Kind() != reflect.Float32 {
				// Can't put non-finite values in a non-float field.
				continue
			}
			c.Run(fmt.Sprintf("%s-%v", t, xnames[i]), func(c *qt.C) {
				v := reflect.New(t).Elem()
				setValueFromFloat(v, x)
				user := newTestStruct(v)
				for _, test := range numericOneOfTests(x) {
					test.run(c, ectx, user)
				}
			})
		}
	}
}

func TestOpCmpNumWithNumericValue(t *testing.T) {
	c := qt.New(t)
	ectx := newEvalTestContext(c)

	for _, op := range []operator{
		opEqNum,
		opLessNum,
		opLessEqNum,
		opGreaterNum,
		opGreaterEqNum,
	} {
		for _, tv := range []interface{}{int8(0), int16(0), int32(0), int64(0), uint8(0), uint16(0), uint32(0), uint64(0), float32(0), float64(0)} {
			t := reflect.TypeOf(tv)
			lo, hi := numLimits(t)
			xnames := []string{
				"lowest",
				"highest",
				"zero",
				"NaN",
				"+Infinity",
				"-Infinity",
			}
			for i, x := range []float64{lo, hi, 0, math.NaN(), math.Inf(1), math.Inf(-1)} {
				if (math.IsNaN(x) || math.IsInf(x, 0)) && t.Kind() != reflect.Float64 && t.Kind() != reflect.Float32 {
					// Can't put non-finite values in a non-float field.
					continue
				}
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

	for _, op := range []operator{
		opEqNum,
		opLessNum,
		opLessEqNum,
		opGreaterNum,
		opGreaterEqNum,
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
		op     operator
		cmpVal string
		want   bool
	}{
		{"1.5", opEqNum, "1.5", true},
		{"1.5", opEqNum, "1.6", false},
		{"", opEqNum, "0", false},
		{"1.5", opNotEqNum, "1.5", false},
		{"1.5", opLessNum, "1.6", true},
		{"1.6", opLessNum, "1.5", false},
		{"1.5", opLessNum, "1.5", false},
		{"1.5", opLessEqNum, "1.5", true},
		{"1.5", opLessEqNum, "1.6", true},
		{"1.6", opLessEqNum, "1.5", false},
		// Invalid numbers always compare false.
		{"bad", opEqNum, "1.5", false},
		{"bad", opNotEqNum, "1.5", false},
		{"bad", opLessNum, "1.5", false},
		{"bad", opLessEqNum, "1.5", false},
		{"bad", opGreaterNum, "1.5", false},
		{"bad", opGreaterEqNum, "1.5", false},
		// NaN always compares false.
		{"NaN", opEqNum, "1.5", false},
		{"NaN", opNotEqNum, "1.5", false},
		{"NaN", opLessNum, "1.5", false},
		{"NaN", opLessEqNum, "1.5", false},
		{"NaN", opGreaterNum, "1.5", false},
		{"NaN", opGreaterEqNum, "1.5", false},
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
	for _, op := range []operator{
		opOneOfSemver,
		opNotOneOfSemver,
		opLessSemver,
		opLessEqSemver,
		opGreaterSemver,
		opGreaterEqSemver,
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
		op     operator
		cmpVal string
		want   bool
	}{
		{"1.5.0", opOneOfSemver, "1.5.0", true},
		{"1.5.0", opOneOfSemver, "1.5.0,1.5.1", true},
		{"1.5.0", opOneOfSemver, "1.5.1,1.5.2", false},
		{"1.5.0", opOneOfSemver, "   1.5.0  ,1.5.2   ", true},
		{"1.5.0", opNotOneOfSemver, "1.5.0", false},
		{"1.5.0", opNotOneOfSemver, "1.5.0,1.5.1", false},
		{"1.5.0", opNotOneOfSemver, "1.5.1,1.5.2", true},
		{"1.5.0", opNotOneOfSemver, "   1.5.0  ,1.5.2   ", false},
		{"1.4.0", opLessSemver, "1.5.0", true},
		{"1.4.0", opLessSemver, " 1.5.0 ", true},
		{"1.4.0", opLessSemver, "1.4.0", false},
		{"1.4.0", opLessSemver, "1.3.0", false},
		{"1.4.0", opGreaterSemver, "1.5.0", false},
		{"1.4.0", opGreaterSemver, "1.4.0", false},
		{"1.4.0", opGreaterSemver, "1.3.0", true},
		{"1.4.0", opLessEqSemver, "1.5.0", true},
		{"1.4.0", opLessEqSemver, "1.4.0", true},
		{"1.4.0", opLessEqSemver, "1.3.0", false},
		{"1.4.0", opGreaterEqSemver, "1.5.0", false},
		{"1.4.0", opGreaterEqSemver, "1.4.0", true},
		{"1.4.0", opGreaterEqSemver, "1.3.0", true},
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
		op:       opOneOf,
		cmpVal:   "foo",
		want:     false,
	}).run(c, ectx, nil)
	(&opTest{
		testName: "nil-struct",
		op:       opOneOf,
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
		op:       opOneOf,
		cmpVal:   "foo",
		want:     false,
	}).run(c, ectx, struct{ X string }{})
}

func stringVariants(s string) []User {
	vs := []interface{}{
		s,
		typedString(s),
		customString{s},
		&customPtrString{s},
		customPtrString{s},
		[]byte(s),
	}
	var us []User
	for _, v := range vs {
		us = append(us, newTestStruct(reflect.ValueOf(v)))
	}
	us = append(us,
		&UserValue{
			Custom: map[string]string{
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

func (g *attributeGetter) GetAttribute(attr string) string {
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
	op operator
	// cmpVal is the argument to the operator.
	cmpVal string
	// want holds the expected result of the test.
	want bool
}

func (test *opTest) run(c *qt.C, ectx *evalTestContext, user User) {
	c.Run(test.testName, func(c *qt.C) {
		ectx.logger.t = c
		c.Logf("operator %v; cmpVal %v; want %v", test.op, test.cmpVal, test.want)
		ectx.srv.setResponseJSON(&rootNode{
			Entries: map[string]*entry{
				"key": {
					VariationID: "testFallback",
					Value:       "false",
					RolloutRules: []*rolloutRule{{
						ComparisonAttribute: "X",
						ComparisonValue:     test.cmpVal,
						Comparator:          test.op,
						VariationID:         "test",
						Value:               "true",
					}},
				},
			},
		})
		ectx.client.Refresh(context.Background())
		want := "false"
		if test.want {
			want = "true"
		}
		c.Check(ectx.client.String("key", "", user), qt.Equals, want, qt.Commentf("user: %#v", user))
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
		op:       opOneOf,
		cmpVal:   s,
		want:     true,
	}, {
		testName: "with-extra-value",
		op:       opOneOf,
		cmpVal:   s + "," + other,
		want:     true,
	}, {
		testName: "with-appended-value",
		op:       opOneOf,
		cmpVal:   s + "x",
		want:     false,
	}, {
		testName: "empty-string",
		op:       opOneOf,
		cmpVal:   "",
		want:     false,
	}}
	// Add tests for opNotOneOf.
	for _, test := range tests {
		tests = append(tests, opTest{
			testName: "not(" + test.testName + ")",
			op:       opNotOneOf,
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
func numericCmpNumTests(x float64, op operator, cmp func(a, b float64) bool) []opTest {
	if math.IsNaN(x) {
		return []opTest{{
			testName: "is-nan",
			op:       op,
			cmpVal:   "NaN",
			want:     false,
		}, {
			testName: "non-nan",
			op:       op,
			cmpVal:   "0",
			want:     false,
		}}
	}

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

// numericOneOfTests returns a set of tests for opOneOf and opNotOneOf given
// a User field with value x.
func numericOneOfTests(x float64) []opTest {
	var tests []opTest
	if math.IsNaN(x) {
		tests = []opTest{{
			testName: "exact-value-as-float",
			op:       opOneOf,
			cmpVal:   fmt.Sprint(x),
			want:     false,
		}}
	} else {
		tests = []opTest{{
			testName: "exact-value-as-float",
			op:       opOneOf,
			cmpVal:   fmt.Sprint(x),
			want:     true,
		}}
		if !math.IsInf(x, 0) {
			tests = append(tests, opTest{
				testName: "small-increment",
				op:       opOneOf,
				cmpVal:   fmt.Sprint(addSomethingSmall(x)),
				want:     false,
			})
		}
	}
	other := 0.0
	if x == other {
		other = 1
	}
	for _, test := range tests {
		tests = append(tests, opTest{
			testName: test.testName + "-with-extra-elem",
			op:       opOneOf,
			cmpVal:   fmt.Sprint(other) + "," + test.cmpVal,
			want:     test.want,
		}, opTest{
			testName: test.testName + "-with-space",
			op:       opOneOf,
			cmpVal:   " " + test.cmpVal + " ",
			want:     test.want,
		})
	}
	// Add tests for opNotOneOf too.
	for _, test := range tests {
		tests = append(tests, opTest{
			testName: "not(" + test.testName + ")",
			op:       opNotOneOf,
			cmpVal:   test.cmpVal,
			want:     !test.want,
		})
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
	cfg.RefreshMode = Manual
	ectx.logger = newTestLogger(c, LogLevelDebug).(*testLogger)
	cfg.Logger = ectx.logger
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

func cmpFunc(op operator) func(a, b float64) bool {
	return func(a, b float64) bool {
		switch op {
		case opEqNum:
			return a == b
		case opLessNum:
			return a < b
		case opLessEqNum:
			return a <= b
		case opGreaterNum:
			return a > b
		case opGreaterEqNum:
			return a >= b
		default:
			panic(fmt.Errorf("unknown comparison operator %v", op))
		}
	}
}
