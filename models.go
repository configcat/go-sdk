package configcat

import "encoding/json"

// ConfigJson describes a ConfigCat config JSON.
type ConfigJson struct {
	// Settings is the map of the available feature flags and settings.
	Settings map[string]*Setting `json:"f"`
	// Segments is the list of available segments.
	Segments []*Segment `json:"s"`
	// Preferences contains additional metadata.
	Preferences *Preferences `json:"p"`
}

// Setting holds all the metadata of a ConfigCat feature flag or setting.
type Setting struct {
	// PercentageOptionsAttribute is the User Object attribute which serves as the basis of percentage options evaluation.
	PercentageOptionsAttribute string `json:"a"`
	// VariationID is the variation ID.
	VariationID string `json:"i"`
	// Value holds the setting's default value used when no targeting rules are matching during an evaluation process.
	Value *SettingValue `json:"v"`
	// Type describes the setting's type. It can be BoolSetting, StringSetting, IntSetting, FloatSetting
	Type SettingType `json:"t"`
	// TargetingRules is the list of targeting rules (where there is a logical OR relation between the items).
	TargetingRules []*TargetingRule `json:"r"`
	// PercentageOptions is the list of percentage options.
	PercentageOptions []*PercentageOption `json:"p"`

	valueID           int32
	prerequisiteCycle []string
	saltBytes         []byte
}

// TargetingRule represents a targeting rule used in the flag evaluation process.
type TargetingRule struct {
	// ServedValue is the value associated with the targeting rule or nil if the targeting rule has percentage options THEN part.
	ServedValue *ServedValue `json:"s"`
	// Conditions is the list of conditions that are combined with the AND logical operator.
	Conditions []*Condition `json:"c"`
	// PercentageOptions is the list of percentage options associated with the targeting rule or nil if the targeting rule has a served value THEN part.
	PercentageOptions []*PercentageOption `json:"p"`
}

type ServedValue struct {
	// Value is the value associated with the targeting rule or nil if the targeting rule has percentage options THEN part.
	Value *SettingValue `json:"v"`
	// VariationID of the targeting rule.
	VariationID string `json:"i"`

	valueID int32
}

// PercentageOption represents a percentage option.
type PercentageOption struct {
	// Value is the served value of the percentage option.
	Value *SettingValue `json:"v"`
	// Percentage is a number between 0 and 100 that represents a randomly allocated fraction of the users.
	Percentage int64 `json:"p"`
	// VariationID of the percentage option.
	VariationID string `json:"i"`

	valueID int32
}

// Segment describes a ConfigCat segment.
type Segment struct {
	// Name is the first 4 characters of the Segment's name
	Name string `json:"n"`
	// Conditions is the list of segment rule conditions (has a logical AND relation between the items).
	Conditions []*UserCondition `json:"r"`

	nameBytes []byte
}

// Condition is the discriminated union of UserCondition, SegmentCondition, and PrerequisiteFlagCondition.
type Condition struct {
	// UserCondition describes a condition that works with User Object attributes.
	UserCondition *UserCondition `json:"u"`
	// UserCondition describes a condition that works with a segment.
	SegmentCondition *SegmentCondition `json:"s"`
	// UserCondition describes a condition that works with a prerequisite flag.
	PrerequisiteFlagCondition *PrerequisiteFlagCondition `json:"p"`
}

// UserCondition describes a condition based on User Object attributes
type UserCondition struct {
	// ComparisonAttribute is a User Object attribute that the condition is based on. Can be "Identifier", "Email", "Country" or any custom attribute.
	ComparisonAttribute string `json:"a"`
	// StringValue is a value in text format that the User Object attribute is compared to.
	StringValue *string `json:"s"`
	// DoubleValue is a value in numeric format that the User Object attribute is compared to.
	DoubleValue *float64 `json:"d"`
	// StringArrayValue is a value in text array format that the User Object attribute is compared to.
	StringArrayValue []string `json:"l"`
	// Comparator is the operator which defines the relation between the comparison attribute and the comparison value.
	Comparator Comparator `json:"c"`
}

// SegmentCondition describes a condition based on a segment.
type SegmentCondition struct {
	// Index identifies the segment that the condition is based on.
	Index int `json:"s"`
	// Comparator is the operator which defines the expected result of the evaluation of the segment.
	Comparator SegmentComparator `json:"c"`

	relatedSegment *Segment
}

// PrerequisiteFlagCondition describes a condition based on a prerequisite feature flag.
type PrerequisiteFlagCondition struct {
	// FlagKey is the key of the prerequisite flag that the condition is based on.
	FlagKey string `json:"f"`
	// Comparator is the operator which defines the relation between the evaluated value of the prerequisite flag and the comparison value.
	Comparator PrerequisiteComparator `json:"c"`
	// Value that the evaluated value of the prerequisite flag is compared to.
	Value *settingValueJson `json:"v"`

	valueID                 int32
	prerequisiteSettingType SettingType
}

// SettingValue describes the value of a feature flag or setting.
type SettingValue struct {
	// Value holds the feature flag's value.
	Value interface{}

	invalidValue interface{}
}

type settingValueJson struct {
	// BoolValue holds a bool feature flag's value.
	BoolValue *bool `json:"b"`
	// StringValue holds a string setting's value.
	StringValue *string `json:"s"`
	// IntValue holds a whole number setting's value.
	IntValue *int `json:"i"`
	// DoubleValue holds a decimal number setting's value.
	DoubleValue *float64 `json:"d"`
}

func (s *SettingValue) MarshalJSON() ([]byte, error) {
	var valJson *settingValueJson
	switch val := s.Value.(type) {
	case bool:
		valJson = &settingValueJson{BoolValue: &val}
	case string:
		valJson = &settingValueJson{StringValue: &val}
	case float64:
		valJson = &settingValueJson{DoubleValue: &val}
	case int:
		valJson = &settingValueJson{IntValue: &val}
	default:
		valJson = nil
	}
	return json.Marshal(valJson)
}

func (s *SettingValue) UnmarshalJSON(b []byte) error {
	valJson := &settingValueJson{}
	if err := json.Unmarshal(b, &valJson); err != nil {
		return err
	}
	s.Value = valueFor(valJson)
	return nil
}

type Preferences struct {
	// Salt is used to hash sensitive comparison values.
	Salt     string           `json:"s"`
	URL      string           `json:"u"`
	Redirect *RedirectionKind `json:"r"` // NoRedirect, ShouldRedirect or ForceRedirect

	saltBytes []byte
}

type SimplifiedConfig struct {
	Flags map[string]interface{} `json:"flags"`
}

type RedirectionKind uint8

const (
	// NoDirect indicates that the configuration is available
	// in this request, but that the next request should be
	// made to the redirected address.
	NoDirect RedirectionKind = 0

	// ShouldRedirect indicates that there is no configuration
	// available at this address, and that the client should
	// redirect immediately. This does not take effect when
	// talking to a custom URL.
	ShouldRedirect RedirectionKind = 1

	// ForceRedirect indicates that there is no configuration
	// available at this address, and that the client should redirect
	// immediately even when talking to a custom URL.
	ForceRedirect RedirectionKind = 2
)

type SettingType int8

const (
	UnknownSetting SettingType = -1
	// BoolSetting is the On/off type (feature flag).
	BoolSetting SettingType = 0
	// StringSetting represents a text setting.
	StringSetting SettingType = 1
	// IntSetting is the whole number type.
	IntSetting SettingType = 2
	// FloatSetting is the decimal number type.
	FloatSetting SettingType = 3
)

// Comparator is the User Object attribute comparison operator used during the evaluation process.
type Comparator uint8

const (
	// OpOneOf matches when the comparison attribute is equal to any of the comparison values.
	OpOneOf Comparator = 0
	// OpNotOneOf matches when the comparison attribute is not equal to any of the comparison values.
	OpNotOneOf Comparator = 1
	// OpContains matches when the comparison attribute contains any comparison values as a substring.
	OpContains Comparator = 2
	// OpNotContains matches when the comparison attribute does not contain any comparison values as a substring.
	OpNotContains Comparator = 3
	// OpOneOfSemver matches when the comparison attribute interpreted as a semantic version is equal to any of the comparison values.
	OpOneOfSemver Comparator = 4
	// OpNotOneOfSemver matches when the comparison attribute interpreted as a semantic version is not equal to any of the comparison values.
	OpNotOneOfSemver Comparator = 5
	// OpLessSemver matches when the comparison attribute interpreted as a semantic version is less than the comparison value.
	OpLessSemver Comparator = 6
	// OpLessEqSemver matches when the comparison attribute interpreted as a semantic version is less than or equal to the comparison value.
	OpLessEqSemver Comparator = 7
	// OpGreaterSemver matches when the comparison attribute interpreted as a semantic version is greater than the comparison value.
	OpGreaterSemver Comparator = 8
	// OpGreaterEqSemver matches when the comparison attribute interpreted as a semantic version is greater than or equal to the comparison value.
	OpGreaterEqSemver Comparator = 9
	// OpEqNum  when the comparison attribute interpreted as a decimal number is equal to the comparison value.
	OpEqNum Comparator = 10
	// OpNotEqNum matches when the comparison attribute interpreted as a decimal number is not equal to the comparison value.
	OpNotEqNum Comparator = 11
	// OpLessNum matches when the comparison attribute interpreted as a decimal number is less than the comparison value.
	OpLessNum Comparator = 12
	// OpLessEqNum matches when the comparison attribute interpreted as a decimal number is less than or equal to the comparison value.
	OpLessEqNum Comparator = 13
	// OpGreaterNum matches when the comparison attribute interpreted as a decimal number is greater than the comparison value.
	OpGreaterNum Comparator = 14
	// OpGreaterEqNum matches when the comparison attribute interpreted as a decimal number is greater than or equal to the comparison value.
	OpGreaterEqNum Comparator = 15
	// OpOneOfHashed matches when the comparison attribute is equal to any of the comparison values (where the comparison is performed using the salted SHA256 hashes of the values).
	OpOneOfHashed Comparator = 16
	// OpNotOneOfHashed matches when the comparison attribute is not equal to any of the comparison values (where the comparison is performed using the salted SHA256 hashes of the values).
	OpNotOneOfHashed Comparator = 17
	// OpBeforeDateTime matches when the comparison attribute interpreted as the seconds elapsed since Unix Epoch is less than the comparison value.
	OpBeforeDateTime Comparator = 18
	// OpAfterDateTime matches when the comparison attribute interpreted as the seconds elapsed since Unix Epoch is greater than the comparison value.
	OpAfterDateTime Comparator = 19
	// OpEqHashed matches when the comparison attribute is equal to the comparison value (where the comparison is performed using the salted SHA256 hashes of the values).
	OpEqHashed Comparator = 20
	// OpNotEqHashed matches when the comparison attribute is not equal to the comparison value (where the comparison is performed using the salted SHA256 hashes of the values).
	OpNotEqHashed Comparator = 21
	// OpStartsWithAnyOfHashed matches when the comparison attribute starts with any of the comparison values (where the comparison is performed using the salted SHA256 hashes of the values).
	OpStartsWithAnyOfHashed Comparator = 22
	// OpNotStartsWithAnyOfHashed matches when the comparison attribute does not start with any of the comparison values (where the comparison is performed using the salted SHA256 hashes of the values).
	OpNotStartsWithAnyOfHashed Comparator = 23
	// OpEndsWithAnyOfHashed matches when the comparison attribute ends with any of the comparison values (where the comparison is performed using the salted SHA256 hashes of the values).
	OpEndsWithAnyOfHashed Comparator = 24
	// OpNotEndsWithAnyOfHashed matches when the comparison attribute does not end with any of the comparison values (where the comparison is performed using the salted SHA256 hashes of the values).
	OpNotEndsWithAnyOfHashed Comparator = 25
	// OpArrayContainsAnyOfHashed matches when the comparison attribute interpreted as a string list contains any of the comparison values (where the comparison is performed using the salted SHA256 hashes of the values).
	OpArrayContainsAnyOfHashed Comparator = 26
	// OpArrayNotContainsAnyOfHashed matches when the comparison attribute interpreted as a string list does not contain any of the comparison values (where the comparison is performed using the salted SHA256 hashes of the values).
	OpArrayNotContainsAnyOfHashed Comparator = 27
	// OpEq matches when the comparison attribute is equal to the comparison value.
	OpEq Comparator = 28
	// OpNotEq matches when the comparison attribute is not equal to the comparison value.
	OpNotEq Comparator = 29
	// OpStartsWithAnyOf matches when the comparison attribute starts with any of the comparison values.
	OpStartsWithAnyOf Comparator = 30
	// OpNotStartsWithAnyOf matches when the comparison attribute does not start with any of the comparison values.
	OpNotStartsWithAnyOf Comparator = 31
	// OpEndsWithAnyOf matches when the comparison attribute ends with any of the comparison values.
	OpEndsWithAnyOf Comparator = 32
	// OpNotEndsWithAnyOf matches when the comparison attribute does not end with any of the comparison values.
	OpNotEndsWithAnyOf Comparator = 33
	// OpArrayContainsAnyOf matches when the comparison attribute interpreted as a string list contains any of the comparison values.
	OpArrayContainsAnyOf Comparator = 34
	// OpArrayNotContainsAnyOf matches when the comparison attribute interpreted as a string list does not contain any of the comparison values.
	OpArrayNotContainsAnyOf Comparator = 35
)

// PrerequisiteComparator is the prerequisite flag comparison operator used during the evaluation process.
type PrerequisiteComparator uint8

const (
	// OpPrerequisiteEq matches when the evaluated value of the specified prerequisite flag is equal to the comparison value.
	OpPrerequisiteEq PrerequisiteComparator = 0
	// OpPrerequisiteNotEq matches when the evaluated value of the specified prerequisite flag is not equal to the comparison value.
	OpPrerequisiteNotEq PrerequisiteComparator = 1
)

// SegmentComparator is the segment comparison operator used during the evaluation process.
type SegmentComparator uint8

const (
	// OpSegmentIsIn matches when the conditions of the specified segment are evaluated to true.
	OpSegmentIsIn SegmentComparator = 0
	// OpSegmentIsNotIn matches when the conditions of the specified segment are evaluated to false.
	OpSegmentIsNotIn SegmentComparator = 1
)

var opStrings = []string{
	OpOneOf:                       "IS ONE OF",
	OpNotOneOf:                    "IS NOT ONE OF",
	OpContains:                    "CONTAINS ANY OF",
	OpNotContains:                 "NOT CONTAINS ANY OF",
	OpOneOfSemver:                 "IS ONE OF",
	OpNotOneOfSemver:              "IS NOT ONE OF",
	OpLessSemver:                  "<",
	OpLessEqSemver:                "<=",
	OpGreaterSemver:               ">",
	OpGreaterEqSemver:             ">=",
	OpEqNum:                       "=",
	OpNotEqNum:                    "!=",
	OpLessNum:                     "<",
	OpLessEqNum:                   "<=",
	OpGreaterNum:                  ">",
	OpGreaterEqNum:                ">=",
	OpOneOfHashed:                 "IS ONE OF",
	OpNotOneOfHashed:              "IS NOT ONE OF",
	OpBeforeDateTime:              "BEFORE",
	OpAfterDateTime:               "AFTER",
	OpEqHashed:                    "EQUALS",
	OpNotEqHashed:                 "NOT EQUALS",
	OpStartsWithAnyOfHashed:       "STARTS WITH ANY OF",
	OpNotStartsWithAnyOfHashed:    "NOT STARTS WITH ANY OF",
	OpEndsWithAnyOfHashed:         "ENDS WITH ANY OF",
	OpNotEndsWithAnyOfHashed:      "NOT ENDS WITH ANY OF",
	OpArrayContainsAnyOfHashed:    "ARRAY CONTAINS ANY OF",
	OpArrayNotContainsAnyOfHashed: "ARRAY NOT CONTAINS ANY OF",
	OpEq:                          "EQUALS",
	OpNotEq:                       "NOT EQUALS",
	OpStartsWithAnyOf:             "STARTS WITH ANY OF",
	OpNotStartsWithAnyOf:          "NOT STARTS WITH ANY OF",
	OpEndsWithAnyOf:               "ENDS WITH ANY OF",
	OpNotEndsWithAnyOf:            "NOT ENDS WITH ANY OF",
	OpArrayContainsAnyOf:          "ARRAY CONTAINS ANY OF",
	OpArrayNotContainsAnyOf:       "ARRAY NOT CONTAINS ANY OF",
}

var opPrerequisiteStrings = []string{
	OpPrerequisiteEq:    "EQUALS",
	OpPrerequisiteNotEq: "DOES NOT EQUAL",
}

var opSegmentStrings = []string{
	OpSegmentIsIn:    "IS IN SEGMENT",
	OpSegmentIsNotIn: "IS NOT IN SEGMENT",
}

func (op Comparator) String() string {
	if op < 0 || int(op) >= len(opStrings) {
		return ""
	}
	return opStrings[op]
}

func (op Comparator) IsList() bool {
	switch op {
	case OpOneOf, OpOneOfHashed, OpNotOneOf, OpNotOneOfHashed, OpOneOfSemver, OpNotOneOfSemver, OpContains, OpNotContains,
		OpStartsWithAnyOf, OpStartsWithAnyOfHashed, OpEndsWithAnyOf, OpEndsWithAnyOfHashed,
		OpNotStartsWithAnyOf, OpNotStartsWithAnyOfHashed, OpNotEndsWithAnyOf, OpNotEndsWithAnyOfHashed,
		OpArrayContainsAnyOf, OpArrayNotContainsAnyOf, OpArrayContainsAnyOfHashed, OpArrayNotContainsAnyOfHashed:
		return true
	default:
		return false
	}
}

func (op Comparator) IsNumeric() bool {
	switch op {
	case OpEqNum, OpNotEqNum, OpLessNum, OpLessEqNum, OpGreaterNum, OpGreaterEqNum:
		return true
	default:
		return false
	}
}

func (op Comparator) IsSensitive() bool {
	switch op {
	case OpOneOfHashed, OpNotOneOfHashed, OpEqHashed, OpNotEqHashed, OpStartsWithAnyOfHashed, OpNotStartsWithAnyOfHashed,
		OpEndsWithAnyOfHashed, OpNotEndsWithAnyOfHashed, OpArrayContainsAnyOfHashed, OpArrayNotContainsAnyOfHashed:
		return true
	default:
		return false
	}
}

func (op Comparator) IsDateTime() bool {
	switch op {
	case OpBeforeDateTime, OpAfterDateTime:
		return true
	default:
		return false
	}
}

func (op PrerequisiteComparator) String() string {
	if op < 0 || int(op) >= len(opPrerequisiteStrings) {
		return ""
	}
	return opPrerequisiteStrings[op]
}

func (op SegmentComparator) String() string {
	if op < 0 || int(op) >= len(opSegmentStrings) {
		return ""
	}
	return opSegmentStrings[op]
}

func valueFor(v *settingValueJson) interface{} {
	switch {
	case v.BoolValue != nil:
		return *v.BoolValue
	case v.IntValue != nil:
		return *v.IntValue
	case v.DoubleValue != nil:
		return *v.DoubleValue
	case v.StringValue != nil:
		return *v.StringValue
	default:
		return nil
	}
}

func settingTypeFor(v *settingValueJson) SettingType {
	switch {
	case v.BoolValue != nil:
		return BoolSetting
	case v.IntValue != nil:
		return IntSetting
	case v.DoubleValue != nil:
		return FloatSetting
	case v.StringValue != nil:
		return StringSetting
	default:
		return UnknownSetting
	}
}
