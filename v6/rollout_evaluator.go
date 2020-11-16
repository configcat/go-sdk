package configcat

import (
	"crypto/sha1"
	"encoding/binary"
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

type rolloutEvaluator struct {
	logger Logger
}

func newRolloutEvaluator(logger Logger) *rolloutEvaluator {
	return &rolloutEvaluator{
		logger: logger,
	}
}

func (evaluator *rolloutEvaluator) evaluate(node *entry, key string, user *User) (interface{}, string) {
	evaluator.logger.Infof("Evaluating GetValue(%s).", key)

	if user == nil {
		if len(node.RolloutRules) > 0 || len(node.PercentageRules) > 0 {
			evaluator.logger.Warnln("Evaluating GetValue(" + key + "). UserObject missing! You should pass a " +
				"UserObject to GetValueForUser() in order to make targeting work properly. " +
				"Read more: https://configcat.com/docs/advanced/user-object.")
		}

		result := node.Value
		evaluator.logger.Infof("Returning %v.", result)
		return result, evaluator.extractVariationId(node.VariationID)
	}

	evaluator.logger.Infof("User object: %v", user)

	for _, rule := range node.RolloutRules {
		userValue := user.GetAttribute(rule.ComparisonAttribute)
		if rule.VariationID == "" || userValue == "" {
			evaluator.logNoMatch(userValue, rule)
			continue
		}

		switch rule.Comparator {
		case opOneOf:
			separated := strings.Split(rule.ComparisonValue, ",")
			for _, item := range separated {
				if strings.Contains(strings.TrimSpace(item), userValue) {
					evaluator.logMatch(userValue, rule)
					return rule.Value, rule.VariationID
				}
			}
		case opNotOneOf:
			separated := strings.Split(rule.ComparisonValue, ",")
			found := false
			for _, item := range separated {
				if strings.Contains(strings.TrimSpace(item), userValue) {
					found = true
				}
			}

			if !found {
				evaluator.logMatch(userValue, rule)
				return rule.Value, rule.VariationID
			}
		case opContains:
			if strings.Contains(userValue, rule.ComparisonValue) {
				evaluator.logMatch(userValue, rule)
				return rule.Value, rule.VariationID
			}
		case opNotContains:
			if !strings.Contains(userValue, rule.ComparisonValue) {
				evaluator.logMatch(userValue, rule)
				return rule.Value, rule.VariationID
			}
		case opOneOfSemver, opNotOneOfSemver:
			separated := strings.Split(rule.ComparisonValue, ",")
			userVersion, err := semver.Make(userValue)
			if err != nil {
				evaluator.logFormatError(userValue, rule, err)
				continue
			}
			matched := false
			shouldContinue := false
			for _, item := range separated {
				cmpItem := strings.TrimSpace(item)
				if len(cmpItem) == 0 {
					continue
				}

				semVer, err := semver.Make(cmpItem)
				if err != nil {
					evaluator.logFormatError(userValue, rule, err)
					shouldContinue = true
					break
				}

				matched = userVersion.EQ(semVer) || matched
			}

			if shouldContinue {
				continue
			}
			if rule.Comparator == opNotOneOfSemver {
				matched = !matched
			}
			if matched {
				evaluator.logMatch(userValue, rule)
				return rule.Value, rule.VariationID
			}
		case opLessSemver, opLessEqSemver, opGreaterSemver, opGreaterEqSemver:
			userVersion, err := semver.Make(userValue)
			if err != nil {
				evaluator.logFormatError(userValue, rule, err)
				continue
			}

			cmpVersion, err := semver.Make(strings.TrimSpace(rule.ComparisonValue))
			if err != nil {
				evaluator.logFormatError(userValue, rule, err)
				continue
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
			if ok {
				evaluator.logMatch(userValue, rule)
				return rule.Value, rule.VariationID
			}
		case opEqNum, opNotEqNum, opLessNum, opLessEqNum, opGreaterNum, opGreaterEqNum:
			userDouble, err := strconv.ParseFloat(strings.Replace(userValue, ",", ".", -1), 64)
			if err != nil {
				evaluator.logFormatError(userValue, rule, err)
				continue
			}

			cmpDouble, err := strconv.ParseFloat(strings.Replace(rule.ComparisonValue, ",", ".", -1), 64)
			if err != nil {
				evaluator.logFormatError(userValue, rule, err)
				continue
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
			if ok {
				evaluator.logMatch(userValue, rule)
				return rule.Value, rule.VariationID
			}
		case opOneOfSensitive:
			separated := strings.Split(rule.ComparisonValue, ",")
			sha := sha1.New()
			sha.Write([]byte(userValue))
			hash := hex.EncodeToString(sha.Sum(nil))
			for _, item := range separated {
				if strings.Contains(strings.TrimSpace(item), hash) {
					evaluator.logMatch(userValue, rule)
					return rule.Value, rule.VariationID
				}
			}
		case opNotOneOfSensitive:
			separated := strings.Split(rule.ComparisonValue, ",")
			found := false
			sha := sha1.New()
			sha.Write([]byte(userValue))
			hash := hex.EncodeToString(sha.Sum(nil))
			for _, item := range separated {
				if strings.Contains(strings.TrimSpace(item), hash) {
					found = true
				}
			}

			if !found {
				evaluator.logMatch(userValue, rule)
				return rule.Value, rule.VariationID
			}
		}

		evaluator.logNoMatch(userValue, rule)
	}

	if len(node.PercentageRules) > 0 {
		sum := sha1.Sum([]byte(key + user.identifier))
		// Treat the first 4 bytes as a number, then knock
		// of the last 4 bits. This is equivalent to turning the
		// entire sum into hex, then decoding the first 7 digits.
		num := int64(binary.BigEndian.Uint32(sum[:4]))
		num >>= 4

		scaled := num % 100
		bucket := int64(0)
		for _, rule := range node.PercentageRules {
			bucket += rule.Percentage
			if scaled < bucket {
				result := rule.Value
				evaluator.logger.Infof("Evaluating %% options. Returning %s", result)
				return result, evaluator.extractVariationId(rule.VariationID)
			}
		}
	}

	result := node.Value
	evaluator.logger.Infof("Returning %v.", result)
	return result, evaluator.extractVariationId(node.VariationID)
}

func (evaluator *rolloutEvaluator) logMatch(userValue interface{}, rule rolloutRule) {
	evaluator.logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => match, returning: %v",
		rule.ComparisonAttribute,
		userValue,
		rule.Comparator,
		rule.ComparisonValue,
		rule.Value,
	)
}

func (evaluator *rolloutEvaluator) logNoMatch(userValue interface{}, rule rolloutRule) {
	evaluator.logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => no match",
		rule.ComparisonAttribute,
		userValue,
		rule.Comparator,
		rule.Value,
	)
}

func (evaluator *rolloutEvaluator) logFormatError(userValue interface{}, rule rolloutRule, err error) {
	evaluator.logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => SKIP rule. Validation error: %v",
		rule.ComparisonAttribute,
		userValue,
		rule.Comparator,
		rule.Value,
		err,
	)
}

func (evaluator *rolloutEvaluator) extractVariationId(variationId interface{}) string {
	result, ok := variationId.(string)
	if !ok {
		return ""
	}
	return result
}
