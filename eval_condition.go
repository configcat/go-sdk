package configcat

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/blang/semver/v4"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// NoUserError is returned when the given user is nil.
type NoUserError struct{}

func (n NoUserError) Error() string {
	return "no user passed"
}

// ComparisonValueError is returned when the comparison value is nil.
type ComparisonValueError struct{}

func (n ComparisonValueError) Error() string {
	return "comparison value missing"
}

var noUser = &NoUserError{}
var compValMiss = &ComparisonValueError{}

func conditionsMatcher(conditions []*Condition, evaluators []settingEvalFunc, configJsonSalt []byte, contextSalt []byte) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	matchers := make([]func(user reflect.Value, info *userTypeInfo) (bool, error), len(conditions))
	for i, condition := range conditions {
		matchers[i] = conditionMatcher(condition, evaluators, configJsonSalt, contextSalt)
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		for _, matcher := range matchers {
			matched, err := matcher(user, info)
			if err != nil {
				return false, err
			}
			if !matched {
				return false, nil
			}
		}
		return true, nil
	}
}

func conditionMatcher(condition *Condition, evaluators []settingEvalFunc, configJsonSalt []byte, contextSalt []byte) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if condition.UserCondition != nil {
		return userConditionMatcher(condition.UserCondition, configJsonSalt, contextSalt)
	}
	if condition.SegmentCondition != nil {
		return segmentConditionMatcher(condition.SegmentCondition, configJsonSalt)
	}
	if condition.PrerequisiteFlagCondition != nil {
		return prerequisiteConditionMatcher(condition.PrerequisiteFlagCondition, evaluators)
	}
	return falseResultMatcher
}

func segmentConditionMatcher(segmentCondition *SegmentCondition, configJsonSalt []byte) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	matchers := make([]func(user reflect.Value, info *userTypeInfo) (bool, error), len(segmentCondition.relatedSegment.Conditions))
	for i, condition := range segmentCondition.relatedSegment.Conditions {
		matchers[i] = userConditionMatcher(condition, configJsonSalt, segmentCondition.relatedSegment.nameBytes)
	}
	needsTrue := segmentCondition.Comparator == OpSegmentIsIn
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		for _, matcher := range matchers {
			matched, err := matcher(user, info)
			if err != nil {
				return false, err
			}
			if matched {
				return needsTrue, nil
			}
		}
		return !needsTrue, nil
	}
}

func prerequisiteConditionMatcher(prerequisiteCondition *PrerequisiteFlagCondition, evaluators []settingEvalFunc) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	prerequisiteKey := prerequisiteCondition.FlagKey
	expectedValueId := prerequisiteCondition.valueID
	prerequisiteKeyId := idForKey(prerequisiteKey, true)

	needsTrue := prerequisiteCondition.Comparator == OpPrerequisiteEq
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if len(evaluators) <= int(prerequisiteKeyId) {
			return false, fmt.Errorf("prerequisite not found")
		}
		prerequisiteEvalFunc := evaluators[prerequisiteKeyId]
		prerequisiteValueId, _, _, _ := prerequisiteEvalFunc(prerequisiteKeyId, user, info)
		return (expectedValueId == prerequisiteValueId) == needsTrue, nil
	}
}

func userConditionMatcher(userCondition *UserCondition, configJsonSalt []byte, contextSalt []byte) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	op := userCondition.Comparator
	switch op {
	case OpEq, OpNotEq:
		return textEqualsMatcher(userCondition.ComparisonAttribute, userCondition.StringValue, op == OpEq)
	case OpEqHashed, OpNotEqHashed:
		return sensitiveTextEqualsMatcher(userCondition.ComparisonAttribute, userCondition.StringValue, configJsonSalt, contextSalt, op == OpEqHashed)
	case OpOneOf, OpNotOneOf:
		return oneOfMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, op == OpOneOf)
	case OpOneOfHashed, OpNotOneOfHashed:
		return sensitiveOneOfMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, configJsonSalt, contextSalt, op == OpOneOfHashed)
	case OpStartsWithAnyOf, OpNotStartsWithAnyOf:
		return startsEndsWithMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, true, op == OpStartsWithAnyOf)
	case OpStartsWithAnyOfHashed, OpNotStartsWithAnyOfHashed:
		return sensitiveStartsEndsWithMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, configJsonSalt, contextSalt, true, op == OpStartsWithAnyOfHashed)
	case OpEndsWithAnyOf, OpNotEndsWithAnyOf:
		return startsEndsWithMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, false, op == OpEndsWithAnyOf)
	case OpEndsWithAnyOfHashed, OpNotEndsWithAnyOfHashed:
		return sensitiveStartsEndsWithMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, configJsonSalt, contextSalt, false, op == OpEndsWithAnyOfHashed)
	case OpContains, OpNotContains:
		return containsMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, op == OpContains)
	case OpOneOfSemver, OpNotOneOfSemver:
		return semverIsOneOfMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, op == OpOneOfSemver)
	case OpGreaterSemver, OpGreaterEqSemver, OpLessSemver, OpLessEqSemver:
		return semverCompareMatcher(userCondition.ComparisonAttribute, userCondition.StringValue, op)
	case OpEqNum, OpNotEqNum, OpGreaterNum, OpGreaterEqNum, OpLessNum, OpLessEqNum:
		return numberCompareMatcher(userCondition.ComparisonAttribute, userCondition.DoubleValue, op)
	case OpBeforeDateTime, OpAfterDateTime:
		return dateTimeMatcher(userCondition.ComparisonAttribute, userCondition.DoubleValue, op == OpBeforeDateTime)
	case OpArrayContainsAnyOf, OpArrayNotContainsAnyOf:
		return arrayContainsMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, op == OpArrayContainsAnyOf)
	case OpArrayContainsAnyOfHashed, OpArrayNotContainsAnyOfHashed:
		return sensitiveArrayContainsMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, configJsonSalt, contextSalt, op == OpArrayContainsAnyOfHashed)
	}
	return falseResultMatcher
}

func textEqualsMatcher(comparisonAttribute string, comparisonValue *string, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompMissErrorMatcher
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		attrVal, err := info.getString(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		return (*comparisonValue == attrVal) == needsTrue, nil
	}
}

func sensitiveTextEqualsMatcher(comparisonAttribute string, comparisonValue *string, configJsonSalt []byte, contextSalt []byte, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompMissErrorMatcher
	}
	hComp, err := hex.DecodeString(*comparisonValue)
	if err != nil || len(hComp) != sha256.Size {
		return falseResultMatcher
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		attrVal, err := info.getBytes(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		usrHash := hashVal(attrVal, configJsonSalt, contextSalt)
		return (bytes.Equal(hComp, usrHash[:])) == needsTrue, nil
	}
}

func oneOfMatcher(comparisonAttribute string, comparisonValues []string, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompMissErrorMatcher
	}
	values := make(map[string]bool, len(comparisonValues))
	for _, item := range comparisonValues {
		values[item] = true
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		attrVal, err := info.getString(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		return values[attrVal] == needsTrue, nil
	}
}

func sensitiveOneOfMatcher(comparisonAttribute string, comparisonValues []string, configJsonSalt []byte, contextSalt []byte, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompMissErrorMatcher
	}
	values := make(map[[sha256.Size]byte]bool, len(comparisonValues))
	for _, item := range comparisonValues {
		var final [sha256.Size]byte
		hashed, err := hex.DecodeString(item)
		if err != nil {
			continue
		}
		copy(final[:], hashed)
		values[final] = true
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		attrVal, err := info.getBytes(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		usrHash := hashVal(attrVal, configJsonSalt, contextSalt)
		return values[usrHash] == needsTrue, nil
	}
}

func startsEndsWithMatcher(comparisonAttribute string, comparisonValues []string, startsWith bool, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompMissErrorMatcher
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		attrVal, err := info.getString(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		for _, item := range comparisonValues {
			var match bool
			if startsWith {
				match = strings.HasPrefix(attrVal, item)
			} else {
				match = strings.HasSuffix(attrVal, item)
			}
			if match {
				return needsTrue, nil
			}
		}
		return !needsTrue, nil
	}
}

func sensitiveStartsEndsWithMatcher(comparisonAttribute string, comparisonValues []string, configJsonSalt []byte, contextSalt []byte, startsWith bool, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompMissErrorMatcher
	}
	values := make([][sha256.Size]byte, len(comparisonValues))
	lengths := make([]int, len(comparisonValues))
	for i, item := range comparisonValues {
		var final [sha256.Size]byte
		parts := strings.Split(item, "_")
		if len(parts) != 2 {
			return falseResultMatcher
		}
		length, err := strconv.Atoi(parts[0])
		if err != nil {
			return falseResultMatcher
		}
		hashed, err := hex.DecodeString(parts[1])
		if err != nil {
			return falseResultMatcher
		}
		copy(final[:], hashed)
		values[i] = final
		lengths[i] = length
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		attrVal, err := info.getBytes(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		for i, item := range values {
			var match bool
			length := lengths[i]
			if len(attrVal) < length {
				continue
			}
			if startsWith {
				chunk := attrVal[:length]
				hashedChunk := hashVal(chunk, configJsonSalt, contextSalt)
				match = bytes.Equal(hashedChunk[:], item[:])
			} else {
				chunk := attrVal[len(attrVal)-length:]
				hashedChunk := hashVal(chunk, configJsonSalt, contextSalt)
				match = bytes.Equal(hashedChunk[:], item[:])
			}
			if match {
				return needsTrue, nil
			}
		}
		return !needsTrue, nil
	}
}

func containsMatcher(comparisonAttribute string, comparisonValues []string, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompMissErrorMatcher
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		attrVal, err := info.getString(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		for _, item := range comparisonValues {
			if strings.Contains(attrVal, item) {
				return needsTrue, nil
			}
		}
		return !needsTrue, nil
	}
}

func semverIsOneOfMatcher(comparisonAttribute string, comparisonValues []string, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompMissErrorMatcher
	}
	versions := make([]semver.Version, 0, len(comparisonValues))
	for _, item := range comparisonValues {
		ver := strings.TrimSpace(item)
		if ver == "" {
			continue
		}
		v, err := semver.Make(ver)
		if err != nil {
			return falseResultMatcher
		}
		versions = append(versions, v)
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		uv, err := info.getSemver(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		for _, ver := range versions {
			if ver.EQ(*uv) {
				return needsTrue, nil
			}
		}
		return !needsTrue, nil
	}
}

func semverCompareMatcher(comparisonAttribute string, comparisonValue *string, op Comparator) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompMissErrorMatcher
	}
	compVer, err := semver.Make(strings.TrimSpace(*comparisonValue))
	if err != nil {
		return falseResultMatcher
	}
	var cmpFunc func(a *semver.Version, b semver.Version) bool
	switch op {
	case OpGreaterSemver:
		cmpFunc = func(a *semver.Version, b semver.Version) bool {
			return a.GT(b)
		}
	case OpGreaterEqSemver:
		cmpFunc = func(a *semver.Version, b semver.Version) bool {
			return a.GTE(b)
		}
	case OpLessSemver:
		cmpFunc = func(a *semver.Version, b semver.Version) bool {
			return a.LT(b)
		}
	case OpLessEqSemver:
		cmpFunc = func(a *semver.Version, b semver.Version) bool {
			return a.LTE(b)
		}
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		uVer, err := info.getSemver(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		return cmpFunc(uVer, compVer), nil
	}
}

func numberCompareMatcher(comparisonAttribute string, comparisonValue *float64, op Comparator) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompMissErrorMatcher
	}
	var cmpFunc func(a, b float64) bool
	switch op {
	case OpEqNum:
		cmpFunc = func(a, b float64) bool {
			return a == b
		}
	case OpNotEqNum:
		cmpFunc = func(a, b float64) bool {
			return a != b
		}
	case OpGreaterNum:
		cmpFunc = func(a, b float64) bool {
			return a > b
		}
	case OpGreaterEqNum:
		cmpFunc = func(a, b float64) bool {
			return a >= b
		}
	case OpLessNum:
		cmpFunc = func(a, b float64) bool {
			return a < b
		}
	case OpLessEqNum:
		cmpFunc = func(a, b float64) bool {
			return a <= b
		}
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		userVal, err := info.getFloat(user, comparisonAttribute)
		if err != nil {
			return false, err
		}
		return cmpFunc(userVal, *comparisonValue), nil
	}
}

func dateTimeMatcher(comparisonAttribute string, comparisonValue *float64, before bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompMissErrorMatcher
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		userVal, err := info.getFloat(user, comparisonAttribute)
		if err != nil || math.IsNaN(userVal) {
			return false, err
		}
		if before {
			return userVal < *comparisonValue, nil
		} else {
			return userVal > *comparisonValue, nil
		}
	}
}

func arrayContainsMatcher(comparisonAttribute string, comparisonValues []string, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompMissErrorMatcher
	}
	values := make(map[string]bool, len(comparisonValues))
	for _, item := range comparisonValues {
		values[item] = true
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		attrVal := info.getSlice(user, comparisonAttribute)
		if len(attrVal) == 0 {
			return false, fmt.Errorf("user attribute '%s' not found", comparisonAttribute)
		}
		for _, item := range attrVal {
			matched := values[item]
			if matched {
				return needsTrue, nil
			}
		}
		return !needsTrue, nil
	}
}

func sensitiveArrayContainsMatcher(comparisonAttribute string, comparisonValues []string, configJsonSalt []byte, contextSalt []byte, needsTrue bool) func(user reflect.Value, info *userTypeInfo) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompMissErrorMatcher
	}
	values := make(map[[sha256.Size]byte]bool, len(comparisonValues))
	for _, item := range comparisonValues {
		var final [sha256.Size]byte
		hashed, err := hex.DecodeString(item)
		if err != nil {
			continue
		}
		copy(final[:], hashed)
		values[final] = true
	}
	return func(user reflect.Value, info *userTypeInfo) (bool, error) {
		if info == nil {
			return false, noUser
		}
		attrVal := info.getSlice(user, comparisonAttribute)
		if len(attrVal) == 0 {
			return false, fmt.Errorf("user attribute '%s' not found", comparisonAttribute)
		}
		for _, item := range attrVal {
			usrHash := hashVal([]byte(item), configJsonSalt, contextSalt)
			matched := values[usrHash]
			if matched {
				return needsTrue, nil
			}
		}
		return !needsTrue, nil
	}
}

func falseResultMatcher(_ reflect.Value, _ *userTypeInfo) (bool, error) {
	return false, nil
}

func falseWithCompMissErrorMatcher(_ reflect.Value, _ *userTypeInfo) (bool, error) {
	return false, compValMiss
}

func hashVal(val []byte, configJsonSalt []byte, contextSalt []byte) [sha256.Size]byte {
	cont := make([]byte, len(val)+len(configJsonSalt)+len(contextSalt))
	copy(cont, val)
	copy(cont[len(val):], configJsonSalt)
	copy(cont[len(val)+len(configJsonSalt):], contextSalt)
	return sha256.Sum256(cont)
}
