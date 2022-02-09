package configcattest

import "github.com/configcat/go-sdk/v7/internal/wireconfig"

// Operator defines a comparison operator.
type Operator wireconfig.Operator

const (
	OpOneOf             = Operator(wireconfig.OpOneOf)
	OpNotOneOf          = Operator(wireconfig.OpNotOneOf)
	OpContains          = Operator(wireconfig.OpContains)
	OpNotContains       = Operator(wireconfig.OpNotContains)
	OpOneOfSemver       = Operator(wireconfig.OpOneOfSemver)
	OpNotOneOfSemver    = Operator(wireconfig.OpNotOneOfSemver)
	OpLessSemver        = Operator(wireconfig.OpLessSemver)
	OpLessEqSemver      = Operator(wireconfig.OpLessEqSemver)
	OpGreaterSemver     = Operator(wireconfig.OpGreaterSemver)
	OpGreaterEqSemver   = Operator(wireconfig.OpGreaterEqSemver)
	OpEqNum             = Operator(wireconfig.OpEqNum)
	OpNotEqNum          = Operator(wireconfig.OpNotEqNum)
	OpLessNum           = Operator(wireconfig.OpLessNum)
	OpLessEqNum         = Operator(wireconfig.OpLessEqNum)
	OpGreaterNum        = Operator(wireconfig.OpGreaterNum)
	OpGreaterEqNum      = Operator(wireconfig.OpGreaterEqNum)
	OpOneOfSensitive    = Operator(wireconfig.OpOneOfSensitive)
	OpNotOneOfSensitive = Operator(wireconfig.OpNotOneOfSensitive)
)

func (op Operator) String() string {
	return wireconfig.Operator(op).String()
}

const invalidEntry wireconfig.EntryType = -1

func typeOf(x interface{}) wireconfig.EntryType {
	switch x.(type) {
	case string:
		return wireconfig.StringEntry
	case int:
		return wireconfig.IntEntry
	case float64:
		return wireconfig.FloatEntry
	case bool:
		return wireconfig.BoolEntry
	}
	return invalidEntry
}
