package configcattest

import (
	"fmt"
	configcat "github.com/configcat/go-sdk/v9"
	"strconv"
	"strings"
)

const invalidType configcat.SettingType = -1

func typeOf(x interface{}) configcat.SettingType {
	switch x.(type) {
	case string:
		return configcat.StringSetting
	case int:
		return configcat.IntSetting
	case float64:
		return configcat.FloatSetting
	case bool:
		return configcat.BoolSetting
	}
	return invalidType
}

// Flag represents a configcat flag.
type Flag struct {
	// Default holds the default value for the flag.
	// It should hold one of the types string, float64, int or bool.
	Default interface{}

	// Rules holds a set of rules to check against in order.
	// If any rule is satisfied, its associated value is used,
	// otherwise the default value is used.
	Rules []Rule
}

type Rule struct {
	// ComparisonAttribute holds the user attribute to
	// check when evaluating the rule.
	ComparisonAttribute string

	// Comparator holds how the compare the above user
	// attribute to the comparison value.
	Comparator configcat.Comparator

	// ComparisonValue holds the value to compare the
	// user attribute against.
	ComparisonValue string

	// Value holds the value for the flag if the rule is satisfied.
	// It should hold one of the types string, float64, int or bool
	// and be the same type as the default value of the
	// flag that it's associated with.
	Value interface{}
}

func (f *Flag) entry(key string) (*configcat.Setting, error) {
	ft := typeOf(f.Default)
	if ft == invalidType {
		return nil, fmt.Errorf("invalid type %T for default value %#v", f.Default, f.Default)
	}
	e := &configcat.Setting{
		VariationID:    "v_" + key,
		Type:           ft,
		Value:          fromAnyValue(f.Default),
		TargetingRules: make([]*configcat.TargetingRule, 0, len(f.Rules)),
	}
	for i, rule := range f.Rules {
		if rule.Comparator.String() == "" {
			return nil, fmt.Errorf("invalid comparator value %d", rule.Comparator)
		}
		if rule.ComparisonAttribute == "" {
			return nil, fmt.Errorf("empty comparison attribute")
		}
		if rule.ComparisonValue == "" {
			return nil, fmt.Errorf("empty comparison value")
		}
		if typeOf(rule.Value) != ft {
			return nil, fmt.Errorf("rule value for rule (%q %v %q) has inconsistent type %T (value %#v) with flag default value %#v", rule.ComparisonAttribute, rule.Comparator, rule.ComparisonValue, rule.Value, rule.Value, f.Default)
		}
		cond := &configcat.UserCondition{
			ComparisonAttribute: rule.ComparisonAttribute,
			Comparator:          rule.Comparator,
		}
		if rule.Comparator.IsList() {
			if strings.Contains(rule.ComparisonValue, ",") {
				split := strings.Split(rule.ComparisonValue, ",")
				cond.StringArrayValue = split
			} else {
				cond.StringArrayValue = []string{rule.ComparisonValue}
			}
		} else if rule.Comparator.IsNumeric() {
			f, err := strconv.ParseFloat(strings.TrimSpace(rule.ComparisonValue), 64)
			if err == nil {
				cond.DoubleValue = &f
			}
		} else {
			cond.StringValue = &rule.ComparisonValue
		}
		e.TargetingRules = append(e.TargetingRules, &configcat.TargetingRule{
			Conditions: []*configcat.Condition{{
				UserCondition: cond,
			}},
			ServedValue: &configcat.ServedValue{
				Value:       fromAnyValue(rule.Value),
				VariationID: fmt.Sprintf("v%d_%s", i, key),
			},
		})
	}
	return e, nil
}

func fromAnyValue(value interface{}) *configcat.SettingValue {
	switch v := value.(type) {
	case bool:
		return &configcat.SettingValue{BoolValue: v}
	case string:
		return &configcat.SettingValue{StringValue: v}
	case float64:
		return &configcat.SettingValue{DoubleValue: v}
	case int:
		return &configcat.SettingValue{IntValue: v}
	default:
		return nil
	}
}
