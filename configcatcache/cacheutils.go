// Package configcatcache holds utility functions for the SDK's caching.
package configcatcache

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

const newLineByte byte = '\n'

// CacheSegmentsFromBytes deserializes a cache entry from a specific format used by the SDK.
func CacheSegmentsFromBytes(cacheBytes []byte) (fetchTime time.Time, eTag string, config []byte, err error) {
	fetchTimeIndex := bytes.IndexByte(cacheBytes, newLineByte)
	eTagIndex := bytes.IndexByte(cacheBytes[fetchTimeIndex+1:], newLineByte)
	if fetchTimeIndex == -1 || eTagIndex == -1 {
		return time.Time{}, "", nil, fmt.Errorf("number of values is fewer than expected")
	}

	fetchTimeBytes := cacheBytes[:fetchTimeIndex]

	fetchTimeMs, err := strconv.ParseInt(string(fetchTimeBytes), 10, 64)
	if err != nil {
		return time.Time{}, "", nil, fmt.Errorf("invalid fetch time: %s. %v", fetchTimeBytes, err)
	}

	eTagBytes := cacheBytes[fetchTimeIndex+1 : fetchTimeIndex+eTagIndex+1]

	configBytes := cacheBytes[eTagIndex+1+fetchTimeIndex+1:]

	return time.UnixMilli(fetchTimeMs), string(eTagBytes), configBytes, nil
}

// CacheSegmentsToBytes serializes the input parameters to a specific format used for caching by the SDK.
func CacheSegmentsToBytes(fetchTime time.Time, eTag string, config []byte) []byte {
	toCache := []byte(strconv.FormatInt(fetchTime.UnixMilli(), 10))
	toCache = append(toCache, newLineByte)
	toCache = append(toCache, eTag...)
	toCache = append(toCache, newLineByte)
	toCache = append(toCache, config...)
	return toCache
}

const ConfigJSONCacheVersion = "v2"
const ConfigJSONName = "config_v5.json"

// ProduceCacheKey constructs a cache key from an SDK key used to identify a cache entry.
func ProduceCacheKey(sdkKey string) string {
	h := sha1.New()
	h.Write([]byte(sdkKey + "_" + ConfigJSONName + "_" + ConfigJSONCacheVersion))
	return hex.EncodeToString(h.Sum(nil))
}
