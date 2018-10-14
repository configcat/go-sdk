package configcat

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"
)

type rolloutEvaluator struct {
}

func (evaluator *rolloutEvaluator) evaluate(json interface{}, key string, user *User) interface{} {
	node, ok := json.(map[string]interface{})
	if !ok {
		return nil
	}

	if user == nil {
		return node["Value"]
	}

	rules, ok := node["RolloutRules"].([]interface{})
	if ok {
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok {
				continue
			}

			comparisonAttribute, ok := rule["ComparisonAttribute"].(string)
			comparisonValue, ok := rule["ComparisonValue"].(string)
			comparator, ok := rule["Comparator"].(float64)
			userValue := user.getAttribute(comparisonAttribute)

			if !ok || len(userValue) == 0 {
				continue
			}

			switch comparator {
			case 0:
				separated := strings.Split(comparisonValue, ",")
				for _, item := range separated {
					if strings.Contains(strings.TrimSpace(item), userValue) {
						return rule["Value"]
					}
				}
			case 1:
				separated := strings.Split(comparisonValue, ",")
				found := false
				for _, item := range separated {
					if strings.Contains(strings.TrimSpace(item), userValue) {
						found = true
					}
				}

				if !found {
					return rule["Value"]
				}
			case 2:
				if strings.Contains(userValue, comparisonValue) {
					return rule["Value"]
				}
			case 3:
				if !strings.Contains(userValue, comparisonValue) {
					return rule["Value"]
				}
			}
		}
	}

	rules, ok = node["RolloutPercentageItems"].([]interface{})
	if ok && len(rules) > 0 {
		hashCandidate := key + user.identifier
		sha := sha1.New()
		sha.Write([]byte(hashCandidate))
		hash := hex.EncodeToString(sha.Sum(nil))[:15]
		num, err := strconv.ParseInt(hash, 16, 64)
		scaled := num % 100
		if err == nil {
			bucket := int64(0)
			for _, r := range rules {
				rule, ok := r.(map[string]interface{})
				if ok {
					p, ok := rule["Percentage"].(float64)
					if ok {
						percentage := int64(p)
						bucket += percentage
						if scaled < bucket {
							return rule["Value"]
						}
					}
				}
			}
		}
	}

	return node["Value"]
}
