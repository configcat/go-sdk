package configcat

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/configcat/go-sdk/v7/internal/wireconfig"
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

	for _, op := range []wireconfig.Operator{
		wireconfig.OpEqNum,
		wireconfig.OpLessNum,
		wireconfig.OpLessEqNum,
		wireconfig.OpGreaterNum,
		wireconfig.OpGreaterEqNum,
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

	for _, op := range []wireconfig.Operator{
		wireconfig.OpEqNum,
		wireconfig.OpLessNum,
		wireconfig.OpLessEqNum,
		wireconfig.OpGreaterNum,
		wireconfig.OpGreaterEqNum,
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
		op     wireconfig.Operator
		cmpVal string
		want   bool
	}{
		{"1.5", wireconfig.OpEqNum, "1.5", true},
		{"1.5", wireconfig.OpEqNum, "1.6", false},
		{"", wireconfig.OpEqNum, "0", false},
		{"1.5", wireconfig.OpNotEqNum, "1.5", false},
		{"1.5", wireconfig.OpLessNum, "1.6", true},
		{"1.6", wireconfig.OpLessNum, "1.5", false},
		{"1.5", wireconfig.OpLessNum, "1.5", false},
		{"1.5", wireconfig.OpLessEqNum, "1.5", true},
		{"1.5", wireconfig.OpLessEqNum, "1.6", true},
		{"1.6", wireconfig.OpLessEqNum, "1.5", false},
		// Invalid numbers always compare false.
		{"bad", wireconfig.OpEqNum, "1.5", false},
		{"bad", wireconfig.OpNotEqNum, "1.5", false},
		{"bad", wireconfig.OpLessNum, "1.5", false},
		{"bad", wireconfig.OpLessEqNum, "1.5", false},
		{"bad", wireconfig.OpGreaterNum, "1.5", false},
		{"bad", wireconfig.OpGreaterEqNum, "1.5", false},
		// NaN always compares false.
		{"NaN", wireconfig.OpEqNum, "1.5", false},
		{"NaN", wireconfig.OpNotEqNum, "1.5", false},
		{"NaN", wireconfig.OpLessNum, "1.5", false},
		{"NaN", wireconfig.OpLessEqNum, "1.5", false},
		{"NaN", wireconfig.OpGreaterNum, "1.5", false},
		{"NaN", wireconfig.OpGreaterEqNum, "1.5", false},
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
	for _, op := range []wireconfig.Operator{
		wireconfig.OpOneOfSemver,
		wireconfig.OpNotOneOfSemver,
		wireconfig.OpLessSemver,
		wireconfig.OpLessEqSemver,
		wireconfig.OpGreaterSemver,
		wireconfig.OpGreaterEqSemver,
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
		op     wireconfig.Operator
		cmpVal string
		want   bool
	}{
		{"1.5.0", wireconfig.OpOneOfSemver, "1.5.0", true},
		{"1.5.0", wireconfig.OpOneOfSemver, "1.5.0,1.5.1", true},
		{"1.5.0", wireconfig.OpOneOfSemver, "1.5.1,1.5.2", false},
		{"1.5.0", wireconfig.OpOneOfSemver, "   1.5.0  ,1.5.2   ", true},
		{"1.5.0", wireconfig.OpNotOneOfSemver, "1.5.0", false},
		{"1.5.0", wireconfig.OpNotOneOfSemver, "1.5.0,1.5.1", false},
		{"1.5.0", wireconfig.OpNotOneOfSemver, "1.5.1,1.5.2", true},
		{"1.5.0", wireconfig.OpNotOneOfSemver, "   1.5.0  ,1.5.2   ", false},
		{"1.4.0", wireconfig.OpLessSemver, "1.5.0", true},
		{"1.4.0", wireconfig.OpLessSemver, " 1.5.0 ", true},
		{"1.4.0", wireconfig.OpLessSemver, "1.4.0", false},
		{"1.4.0", wireconfig.OpLessSemver, "1.3.0", false},
		{"1.4.0", wireconfig.OpGreaterSemver, "1.5.0", false},
		{"1.4.0", wireconfig.OpGreaterSemver, "1.4.0", false},
		{"1.4.0", wireconfig.OpGreaterSemver, "1.3.0", true},
		{"1.4.0", wireconfig.OpLessEqSemver, "1.5.0", true},
		{"1.4.0", wireconfig.OpLessEqSemver, "1.4.0", true},
		{"1.4.0", wireconfig.OpLessEqSemver, "1.3.0", false},
		{"1.4.0", wireconfig.OpGreaterEqSemver, "1.5.0", false},
		{"1.4.0", wireconfig.OpGreaterEqSemver, "1.4.0", true},
		{"1.4.0", wireconfig.OpGreaterEqSemver, "1.3.0", true},
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
		op:       wireconfig.OpOneOf,
		cmpVal:   "foo",
		want:     false,
	}).run(c, ectx, nil)
	(&opTest{
		testName: "nil-struct",
		op:       wireconfig.OpOneOf,
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
		op:       wireconfig.OpOneOf,
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
		&UserData{
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
	op wireconfig.Operator
	// cmpVal is the argument to the operator.
	cmpVal string
	// want holds the expected result of the test.
	want bool
}

func (test *opTest) run(c *qt.C, ectx *evalTestContext, user User) {
	c.Run(test.testName, func(c *qt.C) {
		ectx.logger.logFunc = c.Logf
		c.Logf("operator %v; cmpVal %v; want %v", test.op, test.cmpVal, test.want)
		ectx.srv.setResponseJSON(&wireconfig.RootNode{
			Entries: map[string]*wireconfig.Entry{
				"key": {
					VariationID: "testFallback",
					Value:       "false",
					RolloutRules: []*wireconfig.RolloutRule{{
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
		op:       wireconfig.OpOneOf,
		cmpVal:   s,
		want:     true,
	}, {
		testName: "with-extra-value",
		op:       wireconfig.OpOneOf,
		cmpVal:   s + "," + other,
		want:     true,
	}, {
		testName: "with-appended-value",
		op:       wireconfig.OpOneOf,
		cmpVal:   s + "x",
		want:     false,
	}, {
		testName: "empty-string",
		op:       wireconfig.OpOneOf,
		cmpVal:   "",
		want:     false,
	}}
	// Add tests for opNotOneOf.
	for _, test := range tests {
		tests = append(tests, opTest{
			testName: "not(" + test.testName + ")",
			op:       wireconfig.OpNotOneOf,
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
func numericCmpNumTests(x float64, op wireconfig.Operator, cmp func(a, b float64) bool) []opTest {
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
			op:       wireconfig.OpOneOf,
			cmpVal:   fmt.Sprint(x),
			want:     false,
		}}
	} else {
		tests = []opTest{{
			testName: "exact-value-as-float",
			op:       wireconfig.OpOneOf,
			cmpVal:   fmt.Sprint(x),
			want:     true,
		}}
		if !math.IsInf(x, 0) {
			tests = append(tests, opTest{
				testName: "small-increment",
				op:       wireconfig.OpOneOf,
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
			op:       wireconfig.OpOneOf,
			cmpVal:   fmt.Sprint(other) + "," + test.cmpVal,
			want:     test.want,
		}, opTest{
			testName: test.testName + "-with-space",
			op:       wireconfig.OpOneOf,
			cmpVal:   " " + test.cmpVal + " ",
			want:     test.want,
		})
	}
	// Add tests for opNotOneOf too.
	for _, test := range tests {
		tests = append(tests, opTest{
			testName: "not(" + test.testName + ")",
			op:       wireconfig.OpNotOneOf,
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
	cfg.PollingMode = Manual
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

func cmpFunc(op wireconfig.Operator) func(a, b float64) bool {
	return func(a, b float64) bool {
		switch op {
		case wireconfig.OpEqNum:
			return a == b
		case wireconfig.OpLessNum:
			return a < b
		case wireconfig.OpLessEqNum:
			return a <= b
		case wireconfig.OpGreaterNum:
			return a > b
		case wireconfig.OpGreaterEqNum:
			return a >= b
		default:
			panic(fmt.Errorf("unknown comparison operator %v", op))
		}
	}
}
