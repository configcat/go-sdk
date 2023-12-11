package configcat

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/blang/semver/v4"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// noUserError is returned when the given user is nil.
type noUserError struct{}

func (n noUserError) Error() string {
	return "cannot evaluate, User Object is missing"
}

var noUser = &noUserError{}

// comparisonValueError is returned when the comparison value is nil.
type comparisonValueError struct {
	value interface{}
	attr  string
	err   error
}

func (n comparisonValueError) Error() string {
	result := fmt.Sprintf("comparison value '%v' is invalid", n.value)
	if n.err != nil {
		result += fmt.Sprintf(" (%s)", n.err.Error())
	}
	return result
}

type prerequisiteNotFoundErr struct {
	key string
}

func (p prerequisiteNotFoundErr) Error() string {
	return fmt.Sprintf("prerequisite '%s' not found", p.key)
}

func conditionsMatcher(conditions []*Condition, evaluators []settingEvalFunc, configJsonSalt []byte, contextSalt []byte) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	matchers := make([]func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error), len(conditions))
	for i, condition := range conditions {
		matchers[i] = conditionMatcher(condition, evaluators, configJsonSalt, contextSalt)
	}
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.newLineString("- ")
		}
		for i, matcher := range matchers {
			if builder != nil {
				if i == 0 {
					builder.append("IF ").incIndent()
				} else {
					builder.incIndent().newLineString("AND ")
				}
			}
			matched, err := matcher(user, info, builder, logger)
			if builder != nil {
				builder.append(" => ").append(fmt.Sprintf("%v", matched))
				if !matched {
					builder.append(", skipping the remaining AND conditions")
				}
				builder.decIndent()
			}
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

func conditionMatcher(condition *Condition, evaluators []settingEvalFunc, configJsonSalt []byte, contextSalt []byte) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if condition.UserCondition != nil {
		return userConditionMatcher(condition.UserCondition, configJsonSalt, contextSalt)
	}
	if condition.SegmentCondition != nil {
		return segmentConditionMatcher(condition.SegmentCondition, configJsonSalt)
	}
	if condition.PrerequisiteFlagCondition != nil {
		return prerequisiteConditionMatcher(condition.PrerequisiteFlagCondition, evaluators)
	}
	return falseResultMatcher(errors.New("condition isn't a type of user, segment, or prerequisite condition"))
}

func segmentConditionMatcher(segmentCondition *SegmentCondition, configJsonSalt []byte) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	matchers := make([]func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error), len(segmentCondition.relatedSegment.Conditions))
	for i, condition := range segmentCondition.relatedSegment.Conditions {
		matchers[i] = userConditionMatcher(condition, configJsonSalt, segmentCondition.relatedSegment.nameBytes)
	}
	name := "<invalid value>"
	if segmentCondition.relatedSegment != nil {
		name = segmentCondition.relatedSegment.Name
	}
	op := segmentCondition.Comparator
	needsTrue := segmentCondition.Comparator == OpSegmentIsIn
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.append(fmt.Sprintf("User %s '%s'", op.String(), name))
		}
		if info == nil {
			return false, noUser
		}
		if builder != nil {
			builder.newLineString("(").incIndent().newLine().
				append(fmt.Sprintf("Evaluating segment '%s':", name))
		}
		result := true
		var resErr error
		for i, matcher := range matchers {
			if builder != nil {
				builder.newLineString("- ")
			}
			if builder != nil {
				if i == 0 {
					builder.append("IF ").incIndent()
				} else {
					builder.incIndent().newLineString("AND ")
				}
			}
			matched, err := matcher(user, info, builder, logger)
			if builder != nil {
				builder.append(" => ").append(fmt.Sprintf("%v", matched))
				if !matched {
					builder.append(", skipping the remaining AND conditions")
				}
				builder.decIndent()
			}
			if err != nil {
				result = false
				resErr = err
				break
			}
			if !matched {
				result = false
				break
			}
		}
		if builder != nil {
			builder.newLineString("Segment evaluation result: ")
			if resErr != nil {
				builder.append(resErr.Error())
			} else {
				resOp := OpSegmentIsNotIn
				if result {
					resOp = OpSegmentIsIn
				}
				builder.append(fmt.Sprintf("User %s", resOp.String()))
			}
			builder.decIndent().newLineString(")")
		}
		return result == needsTrue, resErr
	}
}

func prerequisiteConditionMatcher(prerequisiteCondition *PrerequisiteFlagCondition, evaluators []settingEvalFunc) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	prerequisiteKey := prerequisiteCondition.FlagKey
	expectedValueId := prerequisiteCondition.valueID
	prerequisiteKeyId := idForKey(prerequisiteKey, true)
	prerequisiteType := prerequisiteCondition.prerequisiteSettingType
	prerequisiteValue := valueForSettingType(prerequisiteCondition.Value, prerequisiteType)
	op := prerequisiteCondition.Comparator

	needsTrue := op == OpPrerequisiteEq
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.append(fmt.Sprintf("Flag '%s' %s ", prerequisiteKey, op.String()))
			if prerequisiteValue == nil {
				builder.append("<invalid value>")
			} else {
				builder.append(fmt.Sprintf("'%v'", prerequisiteValue))
			}
		}
		if len(evaluators) <= int(prerequisiteKeyId) {
			return false, &prerequisiteNotFoundErr{key: prerequisiteKey}
		}
		prerequisiteEvalFunc := evaluators[prerequisiteKeyId]
		if prerequisiteEvalFunc == nil {
			return false, &prerequisiteNotFoundErr{key: prerequisiteKey}
		}
		if builder != nil {
			builder.newLineString("(").incIndent().newLine()
		}
		prerequisiteValueId, _, _, _, err := prerequisiteEvalFunc(prerequisiteKeyId, user, info, builder, logger)
		if builder != nil {
			builder.decIndent().newLineString(")")
		}
		return (expectedValueId == prerequisiteValueId) == needsTrue, err
	}
}

func userConditionMatcher(userCondition *UserCondition, configJsonSalt []byte, contextSalt []byte) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	op := userCondition.Comparator
	switch op {
	case OpEq, OpNotEq:
		return textEqualsMatcher(userCondition.ComparisonAttribute, userCondition.StringValue, op)
	case OpEqHashed, OpNotEqHashed:
		return sensitiveTextEqualsMatcher(userCondition.ComparisonAttribute, userCondition.StringValue, configJsonSalt, contextSalt, op)
	case OpOneOf, OpNotOneOf:
		return oneOfMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, op)
	case OpOneOfHashed, OpNotOneOfHashed:
		return sensitiveOneOfMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, configJsonSalt, contextSalt, op)
	case OpStartsWithAnyOf, OpNotStartsWithAnyOf:
		return startsEndsWithMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, true, op)
	case OpStartsWithAnyOfHashed, OpNotStartsWithAnyOfHashed:
		return sensitiveStartsEndsWithMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, configJsonSalt, contextSalt, true, op)
	case OpEndsWithAnyOf, OpNotEndsWithAnyOf:
		return startsEndsWithMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, false, op)
	case OpEndsWithAnyOfHashed, OpNotEndsWithAnyOfHashed:
		return sensitiveStartsEndsWithMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, configJsonSalt, contextSalt, false, op)
	case OpContains, OpNotContains:
		return containsMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, op)
	case OpOneOfSemver, OpNotOneOfSemver:
		return semverIsOneOfMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, op)
	case OpGreaterSemver, OpGreaterEqSemver, OpLessSemver, OpLessEqSemver:
		return semverCompareMatcher(userCondition.ComparisonAttribute, userCondition.StringValue, op)
	case OpEqNum, OpNotEqNum, OpGreaterNum, OpGreaterEqNum, OpLessNum, OpLessEqNum:
		return numberCompareMatcher(userCondition.ComparisonAttribute, userCondition.DoubleValue, op)
	case OpBeforeDateTime, OpAfterDateTime:
		return dateTimeMatcher(userCondition.ComparisonAttribute, userCondition.DoubleValue, op)
	case OpArrayContainsAnyOf, OpArrayNotContainsAnyOf:
		return arrayContainsMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, op)
	case OpArrayContainsAnyOfHashed, OpArrayNotContainsAnyOfHashed:
		return sensitiveArrayContainsMatcher(userCondition.ComparisonAttribute, userCondition.StringArrayValue, configJsonSalt, contextSalt, op)
	}
	return falseResultMatcher(errors.New("comparison operator is invalid"))
}

func textEqualsMatcher(comparisonAttribute string, comparisonValue *string, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValue, op, nil)
	}
	needsTrue := op == OpEq
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, *comparisonValue)
		}
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

func sensitiveTextEqualsMatcher(comparisonAttribute string, comparisonValue *string, configJsonSalt []byte, contextSalt []byte, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValue, op, nil)
	}
	hComp, err := hex.DecodeString(*comparisonValue)
	if err != nil || len(hComp) != sha256.Size {
		return falseWithCompErrorMatcher(comparisonAttribute, *comparisonValue, op, nil)
	}
	needsTrue := op == OpEqHashed
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, *comparisonValue)
		}
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

func oneOfMatcher(comparisonAttribute string, comparisonValues []string, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
	}
	values := make(map[string]bool, len(comparisonValues))
	for _, item := range comparisonValues {
		values[item] = true
	}
	needsTrue := op == OpOneOf
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, keys(values))
		}
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

func sensitiveOneOfMatcher(comparisonAttribute string, comparisonValues []string, configJsonSalt []byte, contextSalt []byte, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
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
	needsTrue := op == OpOneOfHashed
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, make([]string, len(values)))
		}
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

func startsEndsWithMatcher(comparisonAttribute string, comparisonValues []string, startsWith bool, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
	}
	var needsTrue bool
	if startsWith {
		needsTrue = op == OpStartsWithAnyOf
	} else {
		needsTrue = op == OpEndsWithAnyOf
	}
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, comparisonValues)
		}
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

func sensitiveStartsEndsWithMatcher(comparisonAttribute string, comparisonValues []string, configJsonSalt []byte, contextSalt []byte, startsWith bool, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
	}
	values := make([][sha256.Size]byte, len(comparisonValues))
	lengths := make([]int, len(comparisonValues))
	for i, item := range comparisonValues {
		var final [sha256.Size]byte
		parts := strings.Split(item, "_")
		if len(parts) != 2 {
			return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
		}
		length, err := strconv.Atoi(parts[0])
		if err != nil {
			return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
		}
		hashed, err := hex.DecodeString(parts[1])
		if err != nil {
			return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
		}
		copy(final[:], hashed)
		values[i] = final
		lengths[i] = length
	}
	var needsTrue bool
	if startsWith {
		needsTrue = op == OpStartsWithAnyOfHashed
	} else {
		needsTrue = op == OpEndsWithAnyOfHashed
	}
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, make([]string, len(values)))
		}
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

func containsMatcher(comparisonAttribute string, comparisonValues []string, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
	}
	needsTrue := op == OpContains
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, comparisonValues)
		}
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

func semverIsOneOfMatcher(comparisonAttribute string, comparisonValues []string, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
	}
	versions := make([]semver.Version, 0, len(comparisonValues))
	for _, item := range comparisonValues {
		ver := strings.TrimSpace(item)
		if ver == "" {
			continue
		}
		v, err := semver.Make(ver)
		if err != nil {
			return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, err)
		}
		versions = append(versions, v)
	}
	needsTrue := op == OpOneOfSemver
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, comparisonValues)
		}
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

func semverCompareMatcher(comparisonAttribute string, comparisonValue *string, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValue, op, nil)
	}
	compVer, err := semver.Make(strings.TrimSpace(*comparisonValue))
	if err != nil {
		return falseWithCompErrorMatcher(comparisonAttribute, *comparisonValue, op, err)
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
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, *comparisonValue)
		}
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

func numberCompareMatcher(comparisonAttribute string, comparisonValue *float64, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValue, op, nil)
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
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, *comparisonValue)
		}
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

func dateTimeMatcher(comparisonAttribute string, comparisonValue *float64, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValue == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValue, op, nil)
	}
	before := op == OpBeforeDateTime
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, *comparisonValue)
		}
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

func arrayContainsMatcher(comparisonAttribute string, comparisonValues []string, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
	}
	values := make(map[string]bool, len(comparisonValues))
	for _, item := range comparisonValues {
		values[item] = true
	}
	needsTrue := op == OpArrayContainsAnyOf
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, comparisonValues)
		}
		if info == nil {
			return false, noUser
		}
		attrVal, err := info.getSlice(user, comparisonAttribute)
		if err != nil {
			return false, err
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

func sensitiveArrayContainsMatcher(comparisonAttribute string, comparisonValues []string, configJsonSalt []byte, contextSalt []byte, op Comparator) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	if comparisonValues == nil {
		return falseWithCompErrorMatcher(comparisonAttribute, comparisonValues, op, nil)
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
	needsTrue := op == OpArrayContainsAnyOfHashed
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, make([]string, len(values)))
		}
		if info == nil {
			return false, noUser
		}
		attrVal, err := info.getSlice(user, comparisonAttribute)
		if err != nil {
			return false, err
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

func falseResultMatcher(err error) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		return false, err
	}
}

func falseWithCompErrorMatcher(comparisonAttribute string, comparisonValue interface{}, op Comparator, err error) func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
	return func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error) {
		if builder != nil {
			builder.appendUserCondition(comparisonAttribute, op, comparisonValue)
		}
		return false, &comparisonValueError{value: comparisonValue, attr: comparisonAttribute, err: err}
	}
}

func hashVal(val []byte, configJsonSalt []byte, contextSalt []byte) [sha256.Size]byte {
	cont := make([]byte, len(val)+len(configJsonSalt)+len(contextSalt))
	copy(cont, val)
	copy(cont[len(val):], configJsonSalt)
	copy(cont[len(val)+len(configJsonSalt):], contextSalt)
	return sha256.Sum256(cont)
}

func keys[M ~map[string]V, V any](m M) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
