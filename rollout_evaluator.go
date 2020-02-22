package configcat

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/blang/semver"
)

type rolloutEvaluator struct {
	logger          Logger
	comparatorTexts []string
}

func newRolloutEvaluator(logger Logger) *rolloutEvaluator {
	return &rolloutEvaluator{logger: logger,
		comparatorTexts: []string{
			"IS ONE OF",
			"IS NOT ONE OF",
			"CONTAINS",
			"DOES NOT CONTAIN",
			"IS ONE OF (SemVer)",
			"IS NOT ONE OF (SemVer)",
			"< (SemVer)",
			"<= (SemVer)",
			"> (SemVer)",
			">= (SemVer)",
			"= (Number)",
			"<> (Number)",
			"< (Number)",
			"<= (Number)",
			"> (Number)",
			">= (Number)",
		}}
}

func (evaluator *rolloutEvaluator) evaluate(json interface{}, key string, user *User) interface{} {

	node, ok := json.(map[string]interface{})
	if !ok {
		return nil
	}

	evaluator.logger.Infof("Evaluating GetValue(%s).", key)

	rolloutRules, rolloutOk := node["r"].([]interface{})
	percentageRules, percentageOk := node["p"].([]interface{})

	if user == nil {
		if (rolloutOk && len(rolloutRules) > 0) || (percentageOk && len(percentageRules) > 0) {
			evaluator.logger.Warnln("Evaluating GetValue(" + key + "). UserObject missing! You should pass a " +
				"UserObject to GetValueForUser() in order to make targeting work properly. " +
				"Read more: https://configcat.com/docs/advanced/user-object.")
		}

		result := node["v"]
		evaluator.logger.Infof("Returning %v.", result)
		return result
	}

	evaluator.logger.Infof("User object: %v", user)

	if rolloutOk {
		for _, r := range rolloutRules {
			rule, ok := r.(map[string]interface{})
			if !ok {
				continue
			}

			comparisonAttribute, ok := rule["a"].(string)
			comparisonValue, ok := rule["c"].(string)
			comparator, ok := rule["t"].(float64)
			userValue := user.GetAttribute(comparisonAttribute)
			value := rule["v"]

			if !ok || len(userValue) == 0 {
				evaluator.logNoMatch(comparisonAttribute, userValue, comparator, comparisonValue)
				continue
			}

			switch comparator {
			//IS ONE OF
			case 0:
				separated := strings.Split(comparisonValue, ",")
				for _, item := range separated {
					if strings.Contains(strings.TrimSpace(item), userValue) {
						evaluator.logMatch(comparisonAttribute, userValue, comparator, comparisonValue, value)
						return value
					}
				}
			//IS NOT ONE OF
			case 1:
				separated := strings.Split(comparisonValue, ",")
				found := false
				for _, item := range separated {
					if strings.Contains(strings.TrimSpace(item), userValue) {
						found = true
					}
				}

				if !found {
					evaluator.logMatch(comparisonAttribute, userValue, comparator, comparisonValue, value)
					return value
				}
			//CONTAINS
			case 2:
				if strings.Contains(userValue, comparisonValue) {
					evaluator.logMatch(comparisonAttribute, userValue, comparator, comparisonValue, value)
					return value
				}
			//DOES NOT CONTAIN
			case 3:
				if !strings.Contains(userValue, comparisonValue) {
					evaluator.logMatch(comparisonAttribute, userValue, comparator, comparisonValue, value)
					return value
				}
			//IS ONE OF, IS NOT ONE OF (SemVer)
			case 4, 5:
				separated := strings.Split(comparisonValue, ",")
				userVersion, err := semver.Make(userValue)
				if err != nil {
					evaluator.logFormatError(comparisonAttribute, userValue, comparator, comparisonValue, err.Error())
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
						evaluator.logFormatError(comparisonAttribute, userValue, comparator, comparisonValue, err.Error())
						shouldContinue = true
						break
					}

					matched = userVersion.EQ(semVer) || matched
				}

				if shouldContinue {
					continue
				}

				if (matched && comparator == 4) || (!matched && comparator == 5) {
					evaluator.logMatch(comparisonAttribute, userValue, comparator, comparisonValue, value)
					return value
				}
			//LESS THAN, LESS THAN OR EQUALS TO, GREATER THAN, GREATER THAN OR EQUALS TO (SemVer)
			case 6, 7, 8, 9:
				userVersion, err := semver.Make(userValue)
				if err != nil {
					evaluator.logFormatError(comparisonAttribute, userValue, comparator, comparisonValue, err.Error())
					continue
				}

				cmpVersion, err := semver.Make(strings.TrimSpace(comparisonValue))
				if err != nil {
					evaluator.logFormatError(comparisonAttribute, userValue, comparator, comparisonValue, err.Error())
					continue
				}

				if (comparator == 6 && userVersion.LT(cmpVersion)) ||
					(comparator == 7 && userVersion.LTE(cmpVersion)) ||
					(comparator == 8 && userVersion.GT(cmpVersion)) ||
					(comparator == 9 && userVersion.GTE(cmpVersion)) {
					evaluator.logMatch(comparisonAttribute, userValue, comparator, comparisonValue, value)
					return value
				}
			//LESS THAN, LESS THAN OR EQUALS TO, GREATER THAN, GREATER THAN OR EQUALS TO (SemVer)
			case 10, 11, 12, 13, 14, 15:
				userDouble, err := strconv.ParseFloat(strings.Replace(userValue, ",", ".", -1), 64)
				if err != nil {
					evaluator.logFormatError(comparisonAttribute, userValue, comparator, comparisonValue, err.Error())
					continue
				}

				cmpDouble, err := strconv.ParseFloat(strings.Replace(comparisonValue, ",", ".", -1), 64)
				if err != nil {
					evaluator.logFormatError(comparisonAttribute, userValue, comparator, comparisonValue, err.Error())
					continue
				}

				if (comparator == 10 && userDouble == cmpDouble) ||
					(comparator == 11 && userDouble != cmpDouble) ||
					(comparator == 12 && userDouble < cmpDouble) ||
					(comparator == 13 && userDouble <= cmpDouble) ||
					(comparator == 14 && userDouble > cmpDouble) ||
					(comparator == 15 && userDouble >= cmpDouble) {
					evaluator.logMatch(comparisonAttribute, userValue, comparator, comparisonValue, value)
					return value
				}
			//IS ONE OF (Sensitive)
			case 16:
				separated := strings.Split(comparisonValue, ",")
				sha := sha1.New()
				sha.Write([]byte(userValue))
				hash := hex.EncodeToString(sha.Sum(nil))
				for _, item := range separated {
					if strings.Contains(strings.TrimSpace(item), hash) {
						evaluator.logMatch(comparisonAttribute, userValue, comparator, comparisonValue, value)
						return value
					}
				}
			//IS NOT ONE OF (Sensitive)
			case 17:
				separated := strings.Split(comparisonValue, ",")
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
					evaluator.logMatch(comparisonAttribute, userValue, comparator, comparisonValue, value)
					return value
				}
			}

			evaluator.logNoMatch(comparisonAttribute, userValue, comparator, comparisonValue)
		}
	}

	if percentageOk && len(percentageRules) > 0 {
		hashCandidate := key + user.identifier
		sha := sha1.New()
		sha.Write([]byte(hashCandidate))
		hash := hex.EncodeToString(sha.Sum(nil))[:7]
		num, err := strconv.ParseInt(hash, 16, 64)
		scaled := num % 100
		if err == nil {
			bucket := int64(0)
			for _, r := range percentageRules {
				rule, ok := r.(map[string]interface{})
				if ok {
					p, ok := rule["p"].(float64)
					if ok {
						percentage := int64(p)
						bucket += percentage
						if scaled < bucket {
							result := rule["v"]
							evaluator.logger.Infof("Evaluating %% options. Returning %s", result)
							return result
						}
					}
				}
			}
		}
	}

	result := node["v"]
	evaluator.logger.Infof("Returning %v.", result)
	return result
}

func (evaluator *rolloutEvaluator) logMatch(comparisonAttribute string, userValue interface{},
	comparator float64, comparisonValue string, value interface{}) {
	evaluator.logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => match, returning: %v",
		comparisonAttribute, userValue, evaluator.comparatorTexts[int(comparator)], comparisonValue, value)
}

func (evaluator *rolloutEvaluator) logNoMatch(comparisonAttribute string, userValue interface{},
	comparator float64, comparisonValue string) {
	evaluator.logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => no match",
		comparisonAttribute, userValue, evaluator.comparatorTexts[int(comparator)], comparisonValue)
}

func (evaluator *rolloutEvaluator) logFormatError(comparisonAttribute string, userValue interface{},
	comparator float64, comparisonValue string, error string) {
	evaluator.logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => SKIP rule. Validation error: %s",
		comparisonAttribute, userValue, evaluator.comparatorTexts[int(comparator)], comparisonValue, error)
}
