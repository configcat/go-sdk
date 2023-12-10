package configcat

import (
	"fmt"
	"github.com/blang/semver/v4"
	qt "github.com/frankban/quicktest"
	"reflect"
	"testing"
	"time"
)

type usrGetAttr struct {
	v interface{}
}

func (u *usrGetAttr) GetAttribute(attr string) interface{} {
	if attr == "X" {
		return u.v
	}
	return ""
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
		{uintptr(1), "1"},
		{-1, "-1"},
		{1.5, "1.5"},
		{float32(1.5), "1.5"},
		{"1", "1"},
		{"1.5", "1.5"},
		{"text", "text"},
		{[]byte("bytes"), "bytes"},
	}
	for _, test := range tests {
		for _, user := range testUsers(test.attr) {
			runTest(fmt.Sprintf("string-%v-%v", user, test.attr), user, test.exp, t, func(info *userTypeInfo, value reflect.Value) (interface{}, error) {
				return info.getString(value, "X")
			})
			str, _ := test.exp.(string)
			runTest(fmt.Sprintf("bytes-%v-%v", user, test.attr), user, []byte(str), t, func(info *userTypeInfo, value reflect.Value) (interface{}, error) {
				return info.getBytes(value, "X")
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
		{uintptr(1), float64(1)},
		{-1, float64(-1)},
		{1.5, 1.5},
		{float32(1.5), 1.5},
		{"1", float64(1)},
		{"1.5", 1.5},
		{time.UnixMilli(1702217953 * 1000), float64(1702217953)},
	}
	for _, test := range tests {
		for _, user := range testUsers(test.attr) {
			runTest(fmt.Sprintf("%v-%v", user, test.attr), user, test.exp, t, func(info *userTypeInfo, value reflect.Value) (interface{}, error) {
				return info.getFloat(value, "X")
			})
		}
	}
}

func TestGetSemver(t *testing.T) {
	tests := []usrTestCase{
		{"1.2.3", "1.2.3"},
		{[]byte("1.2.3"), "1.2.3"},
	}
	for _, test := range tests {
		for _, user := range testUsers(test.attr) {
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
		for _, user := range testUsers(test.attr) {
			runTest(fmt.Sprintf("%v-%v", user, test.attr), user, test.exp, t, func(info *userTypeInfo, value reflect.Value) (interface{}, error) {
				return info.getSlice(value, "X")
			})
		}
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

func testUsers(val interface{}) []User {
	return []User{
		newTestStruct(reflect.ValueOf(val)),
		&UserData{Custom: map[string]interface{}{"X": val}},
		&usrGetAttr{v: val},
	}
}
