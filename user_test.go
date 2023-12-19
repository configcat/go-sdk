package configcat

import (
	"context"
	"fmt"
	"github.com/blang/semver/v4"
	qt "github.com/frankban/quicktest"
	"math"
	"reflect"
	"testing"
	"time"
)

type usrGetAttr struct {
	v   interface{}
	key string
	id  string
}

func (u *usrGetAttr) GetAttribute(attr string) interface{} {
	if attr == u.key {
		return u.v
	}
	if attr == "Identifier" {
		return u.id
	}
	return nil
}

type usrTestCase struct {
	attr interface{}
	exp  interface{}
}

func TestGetStringOrBytes(t *testing.T) {
	tests := []usrTestCase{
		{1, "1"},
		{int8(1), "1"},
		{int16(1), "1"},
		{int32(1), "1"},
		{int64(1), "1"},
		{uint8(1), "1"},
		{uint16(1), "1"},
		{uint32(1), "1"},
		{uint64(1), "1"},
		{-1, "-1"},
		{1.5, "1.5"},
		{float32(1.5), "1.5"},
		{"1", "1"},
		{"1.5", "1.5"},
		{"text", "text"},
		{[]byte("bytes"), "bytes"},
	}
	for _, test := range tests {
		for _, user := range testUsers(test.attr, "X") {
			runTest(fmt.Sprintf("string-%v-%v", user, test.attr), user, test.exp, t, func(info *userTypeInfo, value reflect.Value) (interface{}, error) {
				res, _, err := info.getString(value, "X")
				return res, err
			})
			str, _ := test.exp.(string)
			runTest(fmt.Sprintf("bytes-%v-%v", user, test.attr), user, []byte(str), t, func(info *userTypeInfo, value reflect.Value) (interface{}, error) {
				res, _, err := info.getBytes(value, "X")
				return res, err
			})
		}
	}
}

func TestGetFloat(t *testing.T) {
	tests := []usrTestCase{
		{1, float64(1)},
		{int8(1), float64(1)},
		{int16(1), float64(1)},
		{int32(1), float64(1)},
		{int64(1), float64(1)},
		{uint8(1), float64(1)},
		{uint16(1), float64(1)},
		{uint32(1), float64(1)},
		{uint64(1), float64(1)},
		{-1, float64(-1)},
		{1.5, 1.5},
		{float32(1.5), 1.5},
		{"1", float64(1)},
		{"1.5", 1.5},
		{time.UnixMilli(1702217953 * 1000), float64(1702217953)},
	}
	for _, test := range tests {
		for _, user := range testUsers(test.attr, "X") {
			runTest(fmt.Sprintf("%v-%v", user, test.attr), user, test.exp, t, func(info *userTypeInfo, value reflect.Value) (interface{}, error) {
				return info.getFloat(value, "X", true)
			})
		}
	}
}

func TestGetFloatShouldNotRecogniseTime(t *testing.T) {
	c := qt.New(t)
	val := time.UnixMilli(1702217953 * 1000)
	for _, user := range testUsers(val, "X") {
		t.Run(fmt.Sprintf("%v", user), func(t *testing.T) {
			userVal := reflect.ValueOf(user)
			info, err := newUserTypeInfo(userVal.Type())
			c.Assert(err, qt.IsNil)
			usr := userVal
			if info.deref {
				usr = userVal.Elem()
			}
			actual, err := info.getFloat(usr, "X", false)
			c.Assert(actual, qt.Equals, float64(0))
			c.Assert(err.Error(), qt.Contains, "cannot evaluate, the User.X attribute is invalid")
			c.Assert(err.Error(), qt.Contains, "is not a valid decimal number")
		})
	}
}

func TestGetSemver(t *testing.T) {
	tests := []usrTestCase{
		{"1.2.3", "1.2.3"},
		{[]byte("1.2.3"), "1.2.3"},
	}
	for _, test := range tests {
		for _, user := range testUsers(test.attr, "X") {
			str, _ := test.exp.(string)
			ver, err := semver.New(str)
			qt.Assert(t, err, qt.IsNil)
			runTest(fmt.Sprintf("%v-%v", user, test.attr), user, ver, t, func(info *userTypeInfo, value reflect.Value) (interface{}, error) {
				return info.getSemver(value, "X")
			})
		}
	}
}

func TestGetSlice(t *testing.T) {
	tests := []usrTestCase{
		{"[\"a\",\"b\"]", []string{"a", "b"}},
		{[]byte("[\"a\",\"b\"]"), []string{"a", "b"}},
		{[]string{"a", "b"}, []string{"a", "b"}},
	}
	for _, test := range tests {
		for _, user := range testUsers(test.attr, "X") {
			runTest(fmt.Sprintf("%v-%v", user, test.attr), user, test.exp, t, func(info *userTypeInfo, value reflect.Value) (interface{}, error) {
				return info.getSlice(value, "X")
			})
		}
	}
}

func TestTextComparisons(t *testing.T) {
	k := "configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ"
	srv := newConfigServerWithKey(t, k)
	srv.setResponse(configResponse{body: contentForIntegrationTestKey(k)})
	cfg := srv.config()
	cfg.PollingMode = Manual
	logger := newTestLogger(t).(*testLogger)
	cfg.Logger = logger
	cfg.LogLevel = LogLevelWarn
	client := NewCustomClient(cfg)
	_ = client.Refresh(context.Background())
	defer client.Close()

	for _, user := range testUsersWithId(42, "Custom1", "12345") {
		t.Run(fmt.Sprintf("%v", user), func(t *testing.T) {
			logger.Clear()
			val := client.Snapshot(user).GetValue("boolTextEqualsNumber")
			qt.Assert(t, val, qt.IsTrue)

			msg := logger.Logs()[0]
			qt.Assert(t, msg, qt.Equals, "WARN: [3005] evaluation of 'boolTextEqualsNumber' may not produce the expected result (the User.Custom1 attribute is not a string value, thus it was automatically converted to the string value '42'); please make sure that using a non-string value was intended")
		})
	}
}

func TestGetIntegration(t *testing.T) {
	tests := []struct {
		sdkKey string
		key    string
		attr   interface{}
		exp    interface{}
	}{
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/iV8vH2MBakKxkFZylxHmTg", "lessThanWithPercentage", "0.0", "20%"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/iV8vH2MBakKxkFZylxHmTg", "lessThanWithPercentage", "0.9.9", "< 1.0.0"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/iV8vH2MBakKxkFZylxHmTg", "lessThanWithPercentage", "1.0.0", "20%"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/iV8vH2MBakKxkFZylxHmTg", "lessThanWithPercentage", "1.1", "20%"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/iV8vH2MBakKxkFZylxHmTg", "lessThanWithPercentage", 0, "20%"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/iV8vH2MBakKxkFZylxHmTg", "lessThanWithPercentage", 0.9, "20%"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/iV8vH2MBakKxkFZylxHmTg", "lessThanWithPercentage", 2, "20%"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int8(-1), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int8(2), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int8(3), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int8(5), ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint8(2), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint8(3), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint8(5), ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int8(-1), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int8(2), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int8(3), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int8(5), ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint16(2), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint16(3), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint16(5), ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", -1, "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", 2, "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", 3, "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", 5, ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint(2), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint(3), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint(5), ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", math.MinInt64, "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int64(2), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int64(3), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", int64(5), ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", math.MaxInt64, ">5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint64(2), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint64(3), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint64(5), ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", uint64(math.MaxUint64), ">5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", float32(-1), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", float32(2), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", float32(2.1), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", float32(3), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", float32(5), ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", math.Inf(1), ">5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", math.NaN(), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", math.Inf(-1), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", float64(-1), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", float64(2), "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", 2.1, "<=2,1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", float64(3), "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", float64(5), ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "-Infinity", "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "-1", "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "2", "<2.1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "2.1", "<=2,1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "2,1", "<=2,1"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "3", "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "5", ">=5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "Infinity", ">5"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "NaN", "<>4.2"},
		{"configcat-sdk-1/PKDVCLf-Hq-h-kCzMp-L7Q/FCWN-k1dV0iBf8QZrDgjdw", "numberWithPercentage", "NaNa", "80%"},

		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", parseTime("2023-03-31T23:59:59.999"), false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", parseTimeTZ("2023-04-01T01:59:59.999 +0200"), false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", parseTime("2023-04-01T00:00:00.001"), true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", parseTimeTZ("2023-04-01T02:00:00.001 +0200"), true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", parseTime("2023-04-30T23:59:59.999"), true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", parseTimeTZ("2023-05-01T01:59:59.999 +0200"), true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", parseTime("2023-05-01T00:00:00.001"), false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", parseTimeTZ("2023-05-01T02:00:00.001 +0200"), false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", math.Inf(-1), false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", 1680307199.999, false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", 1680307200.001, true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", 1682899199.999, true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", 1682899200.001, false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", math.Inf(1), false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", math.NaN(), false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", 1680307199, false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", 1680307201, true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", 1682899199, true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", 1682899201, false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", "-Infinity", false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", "1680307199.999", false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", "1680307200.001", true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", "1682899199.999", true},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", "1682899200.001", false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", "+Infinity", false},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "boolTrueIn202304", "NaN", false},

		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "stringArrayContainsAnyOfDogDefaultCat", []string{"x", "read"}, "Dog"},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "stringArrayContainsAnyOfDogDefaultCat", []string{"x", "Read"}, "Cat"},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "stringArrayContainsAnyOfDogDefaultCat", "[\"x\", \"read\"]", "Dog"},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "stringArrayContainsAnyOfDogDefaultCat", "[\"x\", \"Read\"]", "Cat"},
		{"configcat-sdk-1/JcPbCGl_1E-K9M-fJOyKyQ/OfQqcTjfFUGBwMKqtyEOrQ", "stringArrayContainsAnyOfDogDefaultCat", "x, read", "Cat"},
	}

	for _, test := range tests {
		srv := newConfigServerWithKey(t, test.sdkKey)
		srv.setResponse(configResponse{body: contentForIntegrationTestKey(test.sdkKey)})
		cfg := srv.config()
		cfg.PollingMode = Manual
		cfg.Logger = newTestLogger(t)
		cfg.LogLevel = LogLevelError
		client := NewCustomClient(cfg)
		_ = client.Refresh(context.Background())

		for _, user := range testUsersWithId(test.attr, "Custom1", "12345") {
			t.Run(fmt.Sprintf("%v-%T%v-%v-%v", test.key, test.attr, test.attr, test.exp, user), func(t *testing.T) {
				val := client.Snapshot(user).GetValue(test.key)
				qt.Assert(t, val, qt.Equals, test.exp)
			})
		}
		client.Close()
	}
}

func runTest(name string, user User, exp interface{}, t *testing.T, getFunc func(info *userTypeInfo, value reflect.Value) (interface{}, error)) {
	t.Run(name, func(t *testing.T) {
		c := qt.New(t)
		userVal := reflect.ValueOf(user)
		info, err := newUserTypeInfo(userVal.Type())
		c.Assert(err, qt.IsNil)
		usr := userVal
		if info.deref {
			usr = userVal.Elem()
		}
		actual, err := getFunc(info, usr)
		c.Assert(err, qt.IsNil)
		c.Assert(actual, qt.DeepEquals, exp)
	})
}

func testUsers(val interface{}, attr string) []User {
	return []User{
		newTestStructWithAttr(reflect.ValueOf(val), attr),
		&UserData{Custom: map[string]interface{}{attr: val}},
		&usrGetAttr{v: val, key: attr},
	}
}

func testUsersWithId(val interface{}, attr string, id string) []User {
	return []User{
		newTestStructWithAttrAndId(reflect.ValueOf(val), attr, id),
		&UserData{Identifier: id, Custom: map[string]interface{}{attr: val}},
		&usrGetAttr{v: val, key: attr, id: id},
	}
}

func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05.000", s)
	return t
}

func parseTimeTZ(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05.000 -0700", s)
	return t
}
