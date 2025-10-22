package configcatcache

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func Test_Cache_Utils(t *testing.T) {
	c := qt.New(t)
	cacheEntry := `1686756435844
test-etag
{"p":{"u":"https://cdn-global.configcat.com","r":0,"s":"FUkC6RADjzF0vXrDSfJn7BcEBag9afw1Y6jkqjMP9BA="},"f":{"testKey":{"t":1,"v":{"s":"testValue"}}}}`

	ft, etag, configJson, err := CacheSegmentsFromBytes([]byte(cacheEntry))
	c.Assert(err, qt.IsNil)
	c.Assert(ft.UnixMilli(), qt.Equals, int64(1686756435844))
	c.Assert(etag, qt.Equals, "test-etag")
	c.Assert(string(configJson), qt.Equals, `{"p":{"u":"https://cdn-global.configcat.com","r":0,"s":"FUkC6RADjzF0vXrDSfJn7BcEBag9afw1Y6jkqjMP9BA="},"f":{"testKey":{"t":1,"v":{"s":"testValue"}}}}`)

	tn, _ := time.Parse(time.RFC3339Nano, "2023-06-14T15:27:15.8440000Z")
	serialized := CacheSegmentsToBytes(tn, "test-etag", []byte(`{"p":{"u":"https://cdn-global.configcat.com","r":0,"s":"FUkC6RADjzF0vXrDSfJn7BcEBag9afw1Y6jkqjMP9BA="},"f":{"testKey":{"t":1,"v":{"s":"testValue"}}}}`))
	c.Assert(string(serialized), qt.Equals, cacheEntry)
}
