package configcattest

import (
	"fmt"
	"github.com/configcat/go-sdk/v8/internal/wireconfig"
)

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
	Comparator Operator

	// ComparisonValue holds the value to compare the
	// user attribute against.
	ComparisonValue string

	// Value holds the value for the flag if the rule is satisfied.
	// It should hold one of the types string, float64, int or bool
	// and be the same type as the default value of the
	// flag that it's associated with.
	Value interface{}
}

func (f *Flag) entry(key string) (*wireconfig.Entry, error) {
	ft := typeOf(f.Default)
	if ft == invalidEntry {
		return nil, fmt.Errorf("invalid type %T for default value %#v", f.Default, f.Default)
	}
	e := &wireconfig.Entry{
		VariationID:  "v_" + key,
		Type:         ft,
		Value:        f.Default,
		RolloutRules: make([]*wireconfig.RolloutRule, 0, len(f.Rules)),
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
		e.RolloutRules = append(e.RolloutRules, &wireconfig.RolloutRule{
			Value:               rule.Value,
			ComparisonAttribute: rule.ComparisonAttribute,
			Comparator:          wireconfig.Operator(rule.Comparator),
			ComparisonValue:     rule.ComparisonValue,
			VariationID:         fmt.Sprintf("v%d_%s", i, key),
		})
	}
	return e, nil
}
