package configcat

import (
	"crypto/sha1"
	"encoding/binary"
)

func newRolloutEvaluator(logger Logger) *rolloutEvaluator {
	return &rolloutEvaluator{
		logger: logger,
	}
}

type rolloutEvaluator struct {
	logger Logger
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
		if rule == nil {
			continue
		}
		matched, err := matchRolloutRule(rule, user)
		if err != nil {
			evaluator.logFormatError(user, rule, err)
			continue
		}
		if matched {
			evaluator.logMatch(user, rule)
			return rule.Value, rule.VariationID
		}
		evaluator.logNoMatch(user, rule)
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

func (evaluator *rolloutEvaluator) logMatch(userValue interface{}, rule *rolloutRule) {
	evaluator.logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => match, returning: %v",
		rule.ComparisonAttribute,
		userValue,
		rule.Comparator,
		rule.ComparisonValue,
		rule.Value,
	)
}

func (evaluator *rolloutEvaluator) logNoMatch(userValue interface{}, rule *rolloutRule) {
	evaluator.logger.Infof("Evaluating rule: [%s:%s] [%s] [%s] => no match",
		rule.ComparisonAttribute,
		userValue,
		rule.Comparator,
		rule.Value,
	)
}

func (evaluator *rolloutEvaluator) logFormatError(userValue interface{}, rule *rolloutRule, err error) {
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
