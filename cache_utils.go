package configcat

import (
	"bytes"
	"fmt"
	"strconv"
	"time"
)

const newLineByte byte = '\n'

func GetCacheSegments(cacheBytes []byte) (fetchTime time.Time, eTag string, config []byte, err error) {
	fetchTimeIndex := bytes.IndexByte(cacheBytes, newLineByte)
	if fetchTimeIndex == -1 {
		return time.Time{}, "", nil, fmt.Errorf("fetch time segment of the cache entry not found")
	}

	fetchTimeBytes := cacheBytes[:fetchTimeIndex]

	fetchTimeMs, err := strconv.ParseInt(string(fetchTimeBytes), 10, 64)
	if err != nil {
		return time.Time{}, "", nil, err
	}

	eTagIndex := bytes.IndexByte(cacheBytes[fetchTimeIndex+1:], newLineByte)
	if eTagIndex == -1 {
		return time.Time{}, "", nil, fmt.Errorf("etag segment of the cache entry not found")
	}

	eTagBytes := cacheBytes[fetchTimeIndex+1 : fetchTimeIndex+eTagIndex+1]

	configBytes := cacheBytes[eTagIndex+1+fetchTimeIndex+1:]

	return time.Unix(fetchTimeMs/1000, 0), string(eTagBytes), configBytes, nil
}

func SegmentsToByte(fetchTime time.Time, eTag string, config []byte) []byte {
	toCache := []byte(strconv.FormatInt(fetchTime.Unix()*1000, 10))
	toCache = append(toCache, newLineByte)
	toCache = append(toCache, eTag...)
	toCache = append(toCache, newLineByte)
	toCache = append(toCache, config...)
	return toCache
}
