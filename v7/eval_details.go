package configcat

import (
	"github.com/configcat/go-sdk/v7/internal/wireconfig"
	"time"
)

// EvaluationDetailsMeta holds the additional evaluation information of a feature flag or setting.
type EvaluationDetailsMeta struct {
	Key                             string
	VariationId                     string
	User                            User
	IsDefaultValue                  bool
	Error                           error
	FetchTime                       time.Time
	MatchedEvaluationRule           *RolloutRule
	MatchedEvaluationPercentageRule *PercentageRule
}

// EvaluationDetails holds the additional evaluation information along with the value of a feature flag or setting.
type EvaluationDetails struct {
	Meta  EvaluationDetailsMeta
	Value interface{}
}

// BoolEvaluationDetails holds the additional evaluation information along with the value of a bool flag.
type BoolEvaluationDetails struct {
	Meta  EvaluationDetailsMeta
	Value bool
}

// IntEvaluationDetails holds the additional evaluation information along with the value of a whole number flag.
type IntEvaluationDetails struct {
	Meta  EvaluationDetailsMeta
	Value int
}

// StringEvaluationDetails holds the additional evaluation information along with the value of a string flag.
type StringEvaluationDetails struct {
	Meta  EvaluationDetailsMeta
	Value string
}

// FloatEvaluationDetails holds the additional evaluation information along with the value of a decimal number flag.
type FloatEvaluationDetails struct {
	Meta  EvaluationDetailsMeta
	Value float64
}

type RolloutRule struct {
	ComparisonAttribute string
	ComparisonValue     string
	Comparator          int
}

type PercentageRule struct {
	VariationID string
	Percentage  int64
}

func newPublicRolloutRuleOrNil(rule *wireconfig.RolloutRule) *RolloutRule {
	if rule == nil {
		return nil
	}

	return &RolloutRule{
		Comparator:          int(rule.Comparator),
		ComparisonAttribute: rule.ComparisonAttribute,
		ComparisonValue:     rule.ComparisonValue}
}

func newPublicPercentageRuleOrNil(rule *wireconfig.PercentageRule) *PercentageRule {
	if rule == nil {
		return nil
	}

	return &PercentageRule{Percentage: rule.Percentage}
}
