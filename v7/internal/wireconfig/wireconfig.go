// Package wireconfig holds types and constants that define the
// representation of the ConfigCat configuration as transmitted
// over the wire from the ConfigCat CDN.
package wireconfig

type RootNode struct {
	Entries     map[string]*Entry `json:"f"`
	Preferences *Preferences      `json:"p"`
}

type Entry struct {
	VariationID     string           `json:"i"`
	Value           interface{}      `json:"v"`
	Type            EntryType        `json:"t"`
	RolloutRules    []*RolloutRule   `json:"r"`
	PercentageRules []PercentageRule `json:"p"`
}

type RolloutRule struct {
	VariationID         string      `json:"i"`
	Value               interface{} `json:"v"`
	ComparisonAttribute string      `json:"a"`
	ComparisonValue     string      `json:"c"`
	Comparator          Operator    `json:"t"`
}

type PercentageRule struct {
	VariationID string      `json:"i"`
	Value       interface{} `json:"v"`
	Percentage  int64       `json:"p"`
}

type Preferences struct {
	URL      string           `json:"u"`
	Redirect *RedirectionKind `json:"r"` // NoRedirect, ShouldRedirect or ForceRedirect
}

type RedirectionKind int

const (
	// Nodirect indicates that the configuration is available
	// in this request, but that the next request should be
	// made to the redirected address.
	Nodirect RedirectionKind = 0

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

type EntryType int

const (
	BoolEntry   EntryType = 0
	StringEntry EntryType = 1
	IntEntry    EntryType = 2
	FloatEntry  EntryType = 3
)

type Operator int

const (
	OpOneOf             Operator = 0
	OpNotOneOf          Operator = 1
	OpContains          Operator = 2
	OpNotContains       Operator = 3
	OpOneOfSemver       Operator = 4
	OpNotOneOfSemver    Operator = 5
	OpLessSemver        Operator = 6
	OpLessEqSemver      Operator = 7
	OpGreaterSemver     Operator = 8
	OpGreaterEqSemver   Operator = 9
	OpEqNum             Operator = 10
	OpNotEqNum          Operator = 11
	OpLessNum           Operator = 12
	OpLessEqNum         Operator = 13
	OpGreaterNum        Operator = 14
	OpGreaterEqNum      Operator = 15
	OpOneOfSensitive    Operator = 16
	OpNotOneOfSensitive Operator = 17
)

var opStrings = []string{
	OpOneOf:             "IS ONE OF",
	OpNotOneOf:          "IS NOT ONE OF",
	OpContains:          "CONTAINS",
	OpNotContains:       "DOES NOT CONTAIN",
	OpOneOfSemver:       "IS ONE OF (SemVer)",
	OpNotOneOfSemver:    "IS NOT ONE OF (SemVer)",
	OpLessSemver:        "< (SemVer)",
	OpLessEqSemver:      "<= (SemVer)",
	OpGreaterSemver:     "> (SemVer)",
	OpGreaterEqSemver:   ">= (SemVer)",
	OpEqNum:             "= (Number)",
	OpNotEqNum:          "<> (Number)",
	OpLessNum:           "< (Number)",
	OpLessEqNum:         "<= (Number)",
	OpGreaterNum:        "> (Number)",
	OpGreaterEqNum:      ">= (Number)",
	OpOneOfSensitive:    "IS ONE OF (Sensitive)",
	OpNotOneOfSensitive: "IS NOT ONE OF (Sensitive)",
}

func (op Operator) String() string {
	return opStrings[op]
}
