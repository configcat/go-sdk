package configcat

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
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

// evaluator returns a function that returns the value and variation ID
// with a key and user with respect to the given root node.
func evaluator(root *rootNode) func(logger *leveledLogger, key string, user *User) (interface{}, string, error) {
	entryFuncs := make(map[string]func(logger *leveledLogger, user *User) (interface{}, string))
	for key, entry := range root.Entries {
		entryFuncs[key] = entryEvaluator(key, entry)
	}
	return func(logger *leveledLogger, key string, user *User) (interface{}, string, error) {
		if len(key) == 0 {
			return nil, "", fmt.Errorf("key cannot be empty")
		}
		f := entryFuncs[key]
		if f == nil {
			return nil, "", &parseError{
				"Value not found for key " + key +
					". Here are the available keys: " + strings.Join(keysForRootNode(root), ","),
			}
		}
		val, variationID := f(logger, user)
		return val, variationID, nil
	}
}

func entryEvaluator(key string, node *entry) func(logger *leveledLogger, user *User) (interface{}, string) {
	rules := node.RolloutRules
	matchers := make([]func(string) (bool, error), len(rules))
	for i, rule := range rules {
		matchers[i] = rolloutMatcher(rule)
	}

	return func(logger *leveledLogger, user *User) (interface{}, string) {
		if user == nil {
			if logger.enabled(LogLevelWarn) && (len(rules) > 0 || len(node.PercentageRules) > 0) {
				logger.Warnf("Evaluating GetValue(%s). UserObject missing! You should pass a "+
					"UserObject to GetValueForUser() in order to make targeting work properly. "+
					"Read more: https://configcat.com/docs/advanced/user-object.", key)
			}

			if logger.enabled(LogLevelInfo) {
				logger.Infof("Returning %v.", node.Value)
			}
			return node.Value, node.VariationID
		}
		if logger.enabled(LogLevelInfo) {
			logger.Infof("Evaluating GetValue(%s).", key)
			logger.Infof("User object: %v", user)
		}
		for i, matcher := range matchers {
			rule := rules[i]
			userValue := user.GetAttribute(rule.ComparisonAttribute)
			if userValue == "" {
				continue
			}
			matched, err := matcher(userValue)
			if matched {
				if logger.enabled(LogLevelInfo) {
					logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => match, returning: %v",
						rule.ComparisonAttribute,
						user,
						rule.Comparator,
						rule.ComparisonValue,
						rule.Value,
					)
				}
				return rule.Value, rule.VariationID
			} else {
				if logger.enabled(LogLevelInfo) {
					logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => no match",
						rule.ComparisonAttribute,
						user,
						rule.Comparator,
						rule.ComparisonValue,
					)
				}
			}
			if err != nil {
				if logger.enabled(LogLevelInfo) {
					logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => SKIP rule. Validation error: %v",
						rule.ComparisonAttribute,
						user,
						rule.Comparator,
						rule.Value,
						err,
					)
				}
			}
		}
		// evaluate percentage rules
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
					if logger.enabled(LogLevelInfo) {
						logger.Infof("Evaluating %% options. Returning %v", result)
					}
					return result, rule.VariationID
				}
			}
		}
		result := node.Value
		if logger.enabled(LogLevelInfo) {
			logger.Infof("Returning %v.", result)
		}
		return result, node.VariationID
	}
}

func rolloutMatcher(rule *rolloutRule) func(userValue string) (bool, error) {
	if rule.VariationID == "" {
		return func(userValue string) (bool, error) {
			return false, nil
		}
	}
	comparisonValue := rule.ComparisonValue
	switch rule.Comparator {
	case opOneOf, opNotOneOf:
		// These comparators are using Contains to determine whether the user value is matching to the
		// given rule. It's doing so just for compatibility reasons, in the next major version it'll use
		// equality comparison for simple values and Contains only for collection user value types.
		sep := strings.Split(rule.ComparisonValue, ",")
		set := make([]string, len(sep))
		for _, item := range sep {
			set = append(set, strings.TrimSpace(item))
		}
		needTrue := rule.Comparator == opOneOf
		return func(userValue string) (bool, error) {
			matched := false
			for _, item := range set {
				if strings.Contains(item, userValue) {
					matched = true
					break
				}
			}
			return matched == needTrue, nil
		}
	case opContains:
		return func(userValue string) (bool, error) {
			return strings.Contains(userValue, comparisonValue), nil
		}
	case opNotContains:
		return func(userValue string) (bool, error) {
			return !strings.Contains(userValue, comparisonValue), nil
		}
	case opOneOfSemver, opNotOneOfSemver:
		separated := strings.Split(rule.ComparisonValue, ",")
		versions := make([]semver.Version, 0, len(separated))
		for _, item := range separated {
			item := strings.TrimSpace(item)
			if len(item) == 0 {
				continue
			}
			semVer, err := semver.Make(item)
			if err != nil {
				return errorMatcher(err)
			}
			versions = append(versions, semVer)
		}
		needTrue := rule.Comparator == opOneOfSemver
		return func(userValue string) (bool, error) {
			userVersion, err := semver.Make(userValue)
			if err != nil {
				return false, err
			}
			matched := false
			for _, vers := range versions {
				if vers.EQ(userVersion) {
					matched = true
					break
				}
			}
			return matched == needTrue, nil
		}
	case opLessSemver, opLessEqSemver, opGreaterSemver, opGreaterEqSemver:
		cmpVersion, err := semver.Make(strings.TrimSpace(rule.ComparisonValue))
		if err != nil {
			return errorMatcher(err)
		}
		var cmp func(vers semver.Version) bool
		switch rule.Comparator {
		case opLessSemver:
			cmp = cmpVersion.GT
		case opLessEqSemver:
			cmp = cmpVersion.GTE
		case opGreaterSemver:
			cmp = cmpVersion.LT
		case opGreaterEqSemver:
			cmp = cmpVersion.LTE
		default:
			panic("unreachable")
		}
		return func(userValue string) (bool, error) {
			userVersion, err := semver.Make(userValue)
			if err != nil {
				return false, err
			}
			return cmp(userVersion), nil
		}
	case opEqNum, opNotEqNum, opLessNum, opLessEqNum, opGreaterNum, opGreaterEqNum:
		cmpNum, err := strconv.ParseFloat(strings.Replace(rule.ComparisonValue, ",", ".", -1), 64)
		if err != nil {
			return errorMatcher(err)
		}
		var cmp func(float64) bool
		switch rule.Comparator {
		case opEqNum:
			cmp = func(f float64) bool {
				return f == cmpNum
			}
		case opNotEqNum:
			cmp = func(f float64) bool {
				return f != cmpNum
			}
		case opLessNum:
			cmp = func(f float64) bool {
				return f < cmpNum
			}
		case opLessEqNum:
			cmp = func(f float64) bool {
				return f <= cmpNum
			}
		case opGreaterNum:
			cmp = func(f float64) bool {
				return f > cmpNum
			}
		case opGreaterEqNum:
			cmp = func(f float64) bool {
				return f >= cmpNum
			}
		default:
			panic("unreachable")
		}
		return func(userValue string) (bool, error) {
			userNum, err := strconv.ParseFloat(strings.Replace(userValue, ",", ".", -1), 64)
			if err != nil {
				return false, err
			}
			return cmp(userNum), nil
		}
	case opOneOfSensitive, opNotOneOfSensitive:
		separated := strings.Split(rule.ComparisonValue, ",")
		set := make(map[[sha1.Size]byte]bool)
		for _, item := range separated {
			var hash [sha1.Size]byte
			h, err := hex.DecodeString(strings.TrimSpace(item))
			if err != nil || len(h) != sha1.Size {
				// It can never match.
				continue
			}
			copy(hash[:], h)
			set[hash] = true
		}
		needTrue := rule.Comparator == opOneOfSensitive
		return func(userValue string) (bool, error) {
			hash := sha1.Sum([]byte(userValue))
			return set[hash] == needTrue, nil
		}
	default:
		return func(userValue string) (bool, error) {
			return false, nil
		}
	}
}

func errorMatcher(err error) func(string) (bool, error) {
	return func(string) (bool, error) {
		return false, err
	}
}

type keyValue struct {
	key   string
	value interface{}
}

func keyValuesForRootNode(root *rootNode) map[string]keyValue {
	m := make(map[string]keyValue)
	add := func(variationID string, key string, value interface{}) {
		if _, ok := m[variationID]; !ok {
			m[variationID] = keyValue{
				key:   key,
				value: value,
			}
		}
	}
	for key, entry := range root.Entries {
		add(entry.VariationID, key, entry.Value)
		for _, rule := range entry.RolloutRules {
			add(rule.VariationID, key, rule.Value)
		}
		for _, rule := range entry.PercentageRules {
			add(rule.VariationID, key, rule.Value)
		}
	}
	return m
}

func keysForRootNode(root *rootNode) []string {
	keys := make([]string, 0, len(root.Entries))
	for k := range root.Entries {
		keys = append(keys, k)
	}
	return keys
}
