package configcat

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
	keyBytes          []byte
	prerequisiteCycle []string
	saltBytes         []byte
}

// TargetingRule describes a targeting rule used in the flag evaluation process.
type TargetingRule struct {
	// ServedValue is the value associated with the targeting rule or nil if the targeting rule has percentage options THEN part.
	ServedValue *ServedValue `json:"s"`
	// Conditions is the list of conditions (where there is a logical AND relation between the items).
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

// PercentageOption describes a percentage option used in targeting rules.
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

// Condition is a discriminated union of UserCondition, SegmentCondition, and PrerequisiteFlagCondition.
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
	Value *SettingValue `json:"v"`

	valueID                 int32
	prerequisiteSettingType SettingType
}

// SettingValue describes the possible values of a feature flag or setting.
type SettingValue struct {
	// BoolValue holds a bool feature flag's value.
	BoolValue bool `json:"b"`
	// StringValue holds a string setting's value.
	StringValue string `json:"s"`
	// IntValue holds a whole number setting's value.
	IntValue int `json:"i"`
	// DoubleValue holds a decimal number setting's value.
	DoubleValue float64 `json:"d"`
}

type Preferences struct {
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
	BoolSetting   SettingType = 0
	StringSetting SettingType = 1
	IntSetting    SettingType = 2
	FloatSetting  SettingType = 3
)

type Comparator uint8

const (
	OpOneOf                       Comparator = 0
	OpNotOneOf                    Comparator = 1
	OpContains                    Comparator = 2
	OpNotContains                 Comparator = 3
	OpOneOfSemver                 Comparator = 4
	OpNotOneOfSemver              Comparator = 5
	OpLessSemver                  Comparator = 6
	OpLessEqSemver                Comparator = 7
	OpGreaterSemver               Comparator = 8
	OpGreaterEqSemver             Comparator = 9
	OpEqNum                       Comparator = 10
	OpNotEqNum                    Comparator = 11
	OpLessNum                     Comparator = 12
	OpLessEqNum                   Comparator = 13
	OpGreaterNum                  Comparator = 14
	OpGreaterEqNum                Comparator = 15
	OpOneOfHashed                 Comparator = 16
	OpNotOneOfHashed              Comparator = 17
	OpBeforeDateTime              Comparator = 18
	OpAfterDateTime               Comparator = 19
	OpEqHashed                    Comparator = 20
	OpNotEqHashed                 Comparator = 21
	OpStartsWithAnyOfHashed       Comparator = 22
	OpNotStartsWithAnyOfHashed    Comparator = 23
	OpEndsWithAnyOfHashed         Comparator = 24
	OpNotEndsWithAnyOfHashed      Comparator = 25
	OpArrayContainsAnyOfHashed    Comparator = 26
	OpArrayNotContainsAnyOfHashed Comparator = 27
	OpEq                          Comparator = 28
	OpNotEq                       Comparator = 29
	OpStartsWithAnyOf             Comparator = 30
	OpNotStartsWithAnyOf          Comparator = 31
	OpEndsWithAnyOf               Comparator = 32
	OpNotEndsWithAnyOf            Comparator = 33
	OpArrayContainsAnyOf          Comparator = 34
	OpArrayNotContainsAnyOf       Comparator = 35
)

type PrerequisiteComparator uint8

const (
	OpPrerequisiteEq    PrerequisiteComparator = 0
	OpPrerequisiteNotEq PrerequisiteComparator = 1
)

type SegmentComparator uint8

const (
	OpSegmentIsIn    SegmentComparator = 0
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
