package configcat

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/blang/semver"
)

type operator int

const (
	opOneOf             operator = 0
	opNotOneOf          operator = 1
	opContains          operator = 2
	opNotContains       operator = 3
	opOneOfSemver       operator = 4
	opNotOneOfSemver    operator = 5
	opLessSemver        operator = 6
	opLessEqSemver      operator = 7
	opGreaterSemver     operator = 8
	opGreaterEqSemver   operator = 9
	opEqNum             operator = 10
	opNotEqNum          operator = 11
	opLessNum           operator = 12
	opLessEqNum         operator = 13
	opGreaterNum        operator = 14
	opGreaterEqNum      operator = 15
	opOneOfSensitive    operator = 16
	opNotOneOfSensitive operator = 17
)

var opStrings = []string{
	opOneOf:             "IS ONE OF",
	opNotOneOf:          "IS NOT ONE OF",
	opContains:          "CONTAINS",
	opNotContains:       "DOES NOT CONTAIN",
	opOneOfSemver:       "IS ONE OF (SemVer)",
	opNotOneOfSemver:    "IS NOT ONE OF (SemVer)",
	opLessSemver:        "< (SemVer)",
	opLessEqSemver:      "<= (SemVer)",
	opGreaterSemver:     "> (SemVer)",
	opGreaterEqSemver:   ">= (SemVer)",
	opEqNum:             "= (Number)",
	opNotEqNum:          "<> (Number)",
	opLessNum:           "< (Number)",
	opLessEqNum:         "<= (Number)",
	opGreaterNum:        "> (Number)",
	opGreaterEqNum:      ">= (Number)",
	opOneOfSensitive:    "IS ONE OF (Sensitive)",
	opNotOneOfSensitive: "IS NOT ONE OF (Sensitive)",
}

func (op operator) String() string {
	return opStrings[op]
}

func matchRolloutRule(rule *rolloutRule, user *User) (bool, error) {
	userValue := user.GetAttribute(rule.ComparisonAttribute)
	if rule.VariationID == "" || userValue == "" {
		return false, nil
	}

	switch rule.Comparator {
	case opOneOf, opNotOneOf:
		separated := strings.Split(rule.ComparisonValue, ",")
		for _, item := range separated {
			if strings.Contains(strings.TrimSpace(item), userValue) {
				return rule.Comparator == opOneOf, nil
			}
		}
		return rule.Comparator == opNotOneOf, nil
	case opContains:
		return strings.Contains(userValue, rule.ComparisonValue), nil
	case opNotContains:
		return !strings.Contains(userValue, rule.ComparisonValue), nil
	case opOneOfSemver, opNotOneOfSemver:
		separated := strings.Split(rule.ComparisonValue, ",")
		userVersion, err := semver.Make(userValue)
		if err != nil {
			return false, err
		}
		matched := false
		for _, item := range separated {
			cmpItem := strings.TrimSpace(item)
			if len(cmpItem) == 0 {
				continue
			}
			semVer, err := semver.Make(cmpItem)
			if err != nil {
				return false, err
			}
			if userVersion.EQ(semVer) {
				matched = true
				// Note: we can't break out early here because
				// that would influence the result when a later
				// item being compared against is an invalid
				// semver.
			}
		}

		if rule.Comparator == opNotOneOfSemver {
			matched = !matched
		}

		return matched, nil
	case opLessSemver, opLessEqSemver, opGreaterSemver, opGreaterEqSemver:
		userVersion, err := semver.Make(userValue)
		if err != nil {
			return false, err
		}

		cmpVersion, err := semver.Make(strings.TrimSpace(rule.ComparisonValue))
		if err != nil {
			return false, err
		}
		ok := false
		switch rule.Comparator {
		case opLessSemver:
			ok = userVersion.LT(cmpVersion)
		case opLessEqSemver:
			ok = userVersion.LTE(cmpVersion)
		case opGreaterSemver:
			ok = userVersion.GT(cmpVersion)
		case opGreaterEqSemver:
			ok = userVersion.GTE(cmpVersion)
		}
		return ok, nil
	case opEqNum, opNotEqNum, opLessNum, opLessEqNum, opGreaterNum, opGreaterEqNum:
		userDouble, err := strconv.ParseFloat(strings.Replace(userValue, ",", ".", -1), 64)
		if err != nil {
			return false, err
		}

		cmpDouble, err := strconv.ParseFloat(strings.Replace(rule.ComparisonValue, ",", ".", -1), 64)
		if err != nil {
			return false, err
		}

		ok := false
		switch rule.Comparator {
		case opEqNum:
			ok = userDouble == cmpDouble
		case opNotEqNum:
			ok = userDouble != cmpDouble
		case opLessNum:
			ok = userDouble < cmpDouble
		case opLessEqNum:
			ok = userDouble <= cmpDouble
		case opGreaterNum:
			ok = userDouble > cmpDouble
		case opGreaterEqNum:
			ok = userDouble >= cmpDouble
		}
		return ok, nil
	case opOneOfSensitive, opNotOneOfSensitive:
		separated := strings.Split(rule.ComparisonValue, ",")
		sha := sha1.New()
		sha.Write([]byte(userValue))
		hash := hex.EncodeToString(sha.Sum(nil))
		for _, item := range separated {
			if strings.Contains(strings.TrimSpace(item), hash) {
				return rule.Comparator == opOneOfSensitive, nil
			}
		}
		return rule.Comparator == opNotOneOfSensitive, nil
	default:
		return false, nil
	}
}
