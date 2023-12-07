package configcat

import (
	"time"
)

// EvaluationDetailsData holds the additional evaluation information of a feature flag or setting.
type EvaluationDetailsData struct {
	Key                     string
	VariationID             string
	User                    User
	IsDefaultValue          bool
	Error                   error
	FetchTime               time.Time
	MatchedTargetingRule    *TargetingRule
	MatchedPercentageOption *PercentageOption
}

// EvaluationDetails holds the additional evaluation information along with the value of a feature flag or setting.
type EvaluationDetails struct {
	Data  EvaluationDetailsData
	Value interface{}
}

// BoolEvaluationDetails holds the additional evaluation information along with the value of a bool flag.
type BoolEvaluationDetails struct {
	Data  EvaluationDetailsData
	Value bool
}

// IntEvaluationDetails holds the additional evaluation information along with the value of a whole number flag.
type IntEvaluationDetails struct {
	Data  EvaluationDetailsData
	Value int
}

// StringEvaluationDetails holds the additional evaluation information along with the value of a string flag.
type StringEvaluationDetails struct {
	Data  EvaluationDetailsData
	Value string
}

// FloatEvaluationDetails holds the additional evaluation information along with the value of a decimal number flag.
type FloatEvaluationDetails struct {
	Data  EvaluationDetailsData
	Value float64
}
