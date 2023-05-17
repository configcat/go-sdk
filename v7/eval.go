package configcat

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/configcat/go-sdk/v7/internal/wireconfig"
)

var (
	getAttributeType = reflect.TypeOf((*UserAttributes)(nil)).Elem()
	stringMapType    = reflect.TypeOf(map[string]string(nil))
	stringerType     = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
)

func (conf *config) evaluatorsForUserType(userType reflect.Type) ([]entryEvalFunc, error) {
	if entries, ok := conf.evaluators.Load(userType); ok {
		return entries.([]entryEvalFunc), nil
	}
	// We haven't made an entry for this user type yet,
	// so preprocess it and store the result in the evaluators
	// map.
	entries, err := entryEvaluators(conf.root, userType)
	if err != nil {
		return nil, err
	}
	entries1, _ := conf.evaluators.LoadOrStore(userType, entries)
	return entries1.([]entryEvalFunc), nil
}

type entryEvalFunc = func(id keyID, logger *leveledLogger, userv reflect.Value) (valueID, string, *wireconfig.RolloutRule, *wireconfig.PercentageRule)

func entryEvaluators(root *wireconfig.RootNode, userType reflect.Type) ([]entryEvalFunc, error) {
	tinfo, err := newUserTypeInfo(userType)
	if err != nil {
		return nil, err
	}
	// Allocate all key IDs.
	// TODO we might want to add a configuration option to ignore
	// all keys in the configuration that don't have associated
	// key IDs already.
	for key := range root.Entries {
		idForKey(key, true)
	}
	entries := make([]entryEvalFunc, numKeys())
	for key, entry := range root.Entries {
		entries[idForKey(key, true)] = entryEvaluator(key, entry, tinfo)
	}
	return entries, nil
}

func entryEvaluator(key string, node *wireconfig.Entry, tinfo *userTypeInfo) entryEvalFunc {
	rules := node.RolloutRules
	noUser := func(_ keyID, logger *leveledLogger, user reflect.Value) (valueID, string, *wireconfig.RolloutRule, *wireconfig.PercentageRule) {
		if logger.enabled(LogLevelWarn) && (len(rules) > 0 || len(node.PercentageRules) > 0) {
			logger.Warnf(3001,
				"cannot evaluate targeting rules and %% options for setting '%s' (User Object is missing); " +
				"you should pass a User Object to the evaluation methods like `GetValue()` in order to make targeting work properly; " +
				"read more: https://configcat.com/docs/advanced/user-object/",
				key)
		}
		return node.ValueID, node.VariationID, nil, nil
	}

	if tinfo == nil {
		// No user provided
		return noUser
	}
	matchers := make([]func(userv reflect.Value) (bool, error), len(rules))
	attrInfos := make([]attrInfo, len(rules))
	for i, rule := range rules {
		attrInfos[i] = tinfo.attrInfo(rule.ComparisonAttribute)
		matchers[i] = rolloutMatcher(rule, &attrInfos[i])
	}
	identifierInfo := tinfo.attrInfo("Identifier")
	keyBytes := []byte(key)

	return func(id keyID, logger *leveledLogger, userv reflect.Value) (valueID, string, *wireconfig.RolloutRule, *wireconfig.PercentageRule) {
		if tinfo.deref {
			if userv.IsNil() {
				return noUser(id, logger, userv)
			}
			userv = userv.Elem()
		}
		for i, matcher := range matchers {
			rule := rules[i]
			matched, err := matcher(userv)
			if matched {
				if logger.enabled(LogLevelInfo) {
					logger.Infof(5000, "evaluating rule: [%s:%s] [%s] [%s] => match",
						rule.ComparisonAttribute,
						attrInfos[i].asString(userv),
						rule.Comparator,
						rule.ComparisonValue,
					)
				}
				return rule.ValueID, rule.VariationID, rule, nil
			}
			if err != nil {
				if logger.enabled(LogLevelInfo) {
					logger.Infof(5000, "evaluating rule: [%s:%s] [%s] [%s] => SKIP rule; validation error: %v",
						rule.ComparisonAttribute,
						attrInfos[i].asString(userv),
						rule.Comparator,
						rule.ComparisonValue,
						err,
					)
				}
			} else if logger.enabled(LogLevelInfo) {
				logger.Infof(5000, "evaluating rule: [%s:%s] [%s] [%s] => no match",
					rule.ComparisonAttribute,
					attrInfos[i].asString(userv),
					rule.Comparator,
					rule.ComparisonValue,
				)
			}
		}
		// evaluate percentage rules
		if len(node.PercentageRules) > 0 {
			idBytes := identifierInfo.asBytes(userv)
			hashKey := make([]byte, len(keyBytes)+len(idBytes))
			copy(hashKey, keyBytes)
			copy(hashKey[len(keyBytes):], idBytes)
			sum := sha1.Sum(hashKey)
			// Treat the first 4 bytes as a number, then knock
			// off the last 4 bits. This is equivalent to turning the
			// entire sum into hex, then decoding the first 7 digits.
			num := int64(binary.BigEndian.Uint32(sum[:4]))
			num >>= 4

			scaled := num % 100
			bucket := int64(0)
			for _, rule := range node.PercentageRules {
				bucket += rule.Percentage
				if scaled < bucket {
					return rule.ValueID, rule.VariationID, nil, rule
				}
			}
		}
		return node.ValueID, node.VariationID, nil, nil
	}
}

func rolloutMatcher(rule *wireconfig.RolloutRule, info *attrInfo) func(userVal reflect.Value) (bool, error) {
	op, needTrue := uninvert(rule.Comparator)
	switch op {
	case wireconfig.OpOneOf:
		items := splitFields(rule.ComparisonValue)
		switch info.kind {
		case kindInt:
			m := make(map[int64]bool)
			var single int64
			for _, item := range items {
				cmpVal, _ := intMatch(item, info.ftype)
				i, ok := cmpVal.(int64)
				if !ok {
					continue
				}
				single = i
				m[i] = true
			}
			if len(m) == 1 {
				return func(userVal reflect.Value) (bool, error) {
					return (userVal.FieldByIndex(info.index).Int() == single) == needTrue, nil
				}
			} else {
				return func(userVal reflect.Value) (bool, error) {
					return m[userVal.FieldByIndex(info.index).Int()] == needTrue, nil
				}
			}
		case kindUint:
			m := make(map[uint64]bool)
			var single uint64
			for _, item := range items {
				cmpVal, _ := uintMatch(item, info.ftype)
				i, ok := cmpVal.(uint64)
				if !ok {
					continue
				}
				single = i
				m[i] = true
			}
			if len(m) == 1 {
				return func(userVal reflect.Value) (bool, error) {
					return (userVal.FieldByIndex(info.index).Uint() == single) == needTrue, nil
				}
			} else {
				return func(userVal reflect.Value) (bool, error) {
					return m[userVal.FieldByIndex(info.index).Uint()] == needTrue, nil
				}
			}
		case kindFloat:
			m := make(map[float64]bool)
			var single float64
			for _, item := range items {
				f, err := strconv.ParseFloat(item, 64)
				if err != nil || math.IsNaN(f) {
					continue
				}
				single = f
				m[f] = true
			}
			if len(m) == 1 {
				return func(userVal reflect.Value) (bool, error) {
					return (userVal.FieldByIndex(info.index).Float() == single) == needTrue, nil
				}
			} else {
				return func(userVal reflect.Value) (bool, error) {
					return m[userVal.FieldByIndex(info.index).Float()] == needTrue, nil
				}
			}
		default:
			set := make(map[string]bool)
			for _, item := range items {
				set[item] = true
			}
			return func(user reflect.Value) (bool, error) {
				if s := info.asString(user); s != "" {
					return set[s] == needTrue, nil
				}
				return false, nil
			}
		}
	case wireconfig.OpContains:
		return func(user reflect.Value) (bool, error) {
			if s := info.asString(user); s != "" {
				return strings.Contains(s, rule.ComparisonValue) == needTrue, nil
			}
			return false, nil
		}
	case wireconfig.OpOneOfSemver:
		if info.asSemver == nil {
			return errorMatcher(fmt.Errorf("%v can never match a semver", info.ftype))
		}
		items := splitFields(rule.ComparisonValue)
		versions := make([]semver.Version, 0, len(items))
		for _, item := range items {
			item := strings.TrimSpace(item)
			if len(item) == 0 {
				continue
			}
			semVer, err := semver.Make(item)
			if err != nil {
				return errorMatcher(err)
			}
			versions = append(versions, semVer)
		}
		return func(user reflect.Value) (bool, error) {
			userVersion, err := info.asSemver(user)
			if err != nil || userVersion == nil {
				return false, err
			}
			for _, vers := range versions {
				if vers.EQ(*userVersion) {
					return needTrue, nil
				}
			}
			return !needTrue, nil
		}
	case wireconfig.OpLessSemver, wireconfig.OpLessEqSemver:
		if info.asSemver == nil {
			return errorMatcher(fmt.Errorf("%v can never match a semver", info.ftype))
		}
		cmpval := 0
		if op == wireconfig.OpLessSemver {
			cmpval = -1
		}
		cmpVersion, err := semver.Make(strings.TrimSpace(rule.ComparisonValue))
		if err != nil {
			return errorMatcher(err)
		}
		return func(userValue reflect.Value) (bool, error) {
			userVer, err := info.asSemver(userValue)
			if err != nil || userVer == nil {
				return false, err
			}
			return (userVer.Compare(cmpVersion) <= cmpval) == needTrue, nil
		}
	case wireconfig.OpEqNum:
		switch info.kind {
		case kindInt:
			cmpVal, err := intMatch(rule.ComparisonValue, info.ftype)
			if err != nil {
				return errorMatcher(err)
			}
			i, ok := cmpVal.(int64)
			if !ok {
				return falseMatcher(needTrue)
			}
			return func(user reflect.Value) (bool, error) {
				return (user.FieldByIndex(info.index).Int() == i) == needTrue, nil
			}
		case kindUint:
			cmpVal, err := uintMatch(rule.ComparisonValue, info.ftype)
			if err != nil {
				return errorMatcher(err)
			}
			i, ok := cmpVal.(uint64)
			if !ok {
				return falseMatcher(needTrue)
			}
			return func(user reflect.Value) (bool, error) {
				return (user.FieldByIndex(info.index).Uint() == i) == needTrue, nil
			}
		case kindFloat:
			f, err := parseFloat(rule.ComparisonValue)
			if err != nil {
				return errorMatcher(err)
			}
			if math.IsNaN(f) {
				return alwaysFalseMatcher
			}
			return func(user reflect.Value) (bool, error) {
				f1 := user.FieldByIndex(info.index).Float()
				if math.IsNaN(f1) {
					return false, nil
				}
				return (f1 == f) == needTrue, nil
			}
		default:
			f, err := parseFloat(rule.ComparisonValue)
			if err != nil {
				return errorMatcher(err)
			}
			if math.IsNaN(f) {
				return alwaysFalseMatcher
			}
			return func(user reflect.Value) (bool, error) {
				s := info.asString(user)
				if s == "" {
					return false, nil
				}
				f1, err := parseFloat(s)
				if err != nil || math.IsNaN(f1) {
					return false, err
				}
				return (f == f1) == needTrue, nil
			}
		}
	case wireconfig.OpLessNum, wireconfig.OpLessEqNum:
		switch info.kind {
		case kindInt:
			cmpVal, err := intMatch(rule.ComparisonValue, info.ftype)
			if err != nil {
				return errorMatcher(err)
			}
			switch v := cmpVal.(type) {
			case int64:
				switch op {
				case wireconfig.OpLessNum:
					return func(user reflect.Value) (bool, error) {
						return (user.FieldByIndex(info.index).Int() < v) == needTrue, nil
					}
				case wireconfig.OpLessEqNum:
					return func(user reflect.Value) (bool, error) {
						return (user.FieldByIndex(info.index).Int() <= v) == needTrue, nil
					}
				default:
					panic("unreachable")
				}
			case float64:
				switch op {
				case wireconfig.OpLessNum:
					return func(user reflect.Value) (bool, error) {
						return (float64(user.FieldByIndex(info.index).Int()) < v) == needTrue, nil
					}
				case wireconfig.OpLessEqNum:
					return func(user reflect.Value) (bool, error) {
						return (float64(user.FieldByIndex(info.index).Int()) <= v) == needTrue, nil
					}
				default:
					panic("unreachable")
				}
			default:
				panic("unreachable")
			}
		case kindUint:
			cmpVal, err := uintMatch(rule.ComparisonValue, info.ftype)
			if err != nil {
				return errorMatcher(err)
			}
			switch v := cmpVal.(type) {
			case uint64:
				switch op {
				case wireconfig.OpLessNum:
					return func(user reflect.Value) (bool, error) {
						return (user.FieldByIndex(info.index).Uint() < v) == needTrue, nil
					}
				case wireconfig.OpLessEqNum:
					return func(user reflect.Value) (bool, error) {
						return (user.FieldByIndex(info.index).Uint() <= v) == needTrue, nil
					}
				default:
					panic("unreachable")
				}
			case float64:
				switch op {
				case wireconfig.OpLessNum:
					return func(user reflect.Value) (bool, error) {
						return (float64(user.FieldByIndex(info.index).Uint()) < v) == needTrue, nil
					}
				case wireconfig.OpLessEqNum:
					return func(user reflect.Value) (bool, error) {
						return (float64(user.FieldByIndex(info.index).Uint()) <= v) == needTrue, nil
					}
				default:
					panic("unreachable")
				}
			default:
				panic("unreachable")
			}
		case kindFloat:
			f, err := parseFloat(rule.ComparisonValue)
			if err != nil {
				return errorMatcher(err)
			}
			if math.IsNaN(f) {
				return alwaysFalseMatcher
			}
			switch op {
			case wireconfig.OpLessNum:
				return func(user reflect.Value) (bool, error) {
					f1 := user.FieldByIndex(info.index).Float()
					if math.IsNaN(f1) {
						return false, nil
					}
					return (f1 < f) == needTrue, nil
				}
			case wireconfig.OpLessEqNum:
				return func(user reflect.Value) (bool, error) {
					f1 := user.FieldByIndex(info.index).Float()
					if math.IsNaN(f1) {
						return false, nil
					}
					return (f1 <= f) == needTrue, nil
				}
			default:
				panic("unreachable")
			}
		default:
			f, err := parseFloat(rule.ComparisonValue)
			if err != nil {
				return errorMatcher(err)
			}
			if math.IsNaN(f) {
				return alwaysFalseMatcher
			}
			var cmp func(f1, f2 float64) bool
			switch op {
			case wireconfig.OpLessNum:
				cmp = func(f1, f2 float64) bool {
					return f1 < f2
				}
			case wireconfig.OpLessEqNum:
				cmp = func(f1, f2 float64) bool {
					return f1 <= f2
				}
			default:
				panic("unreachable")
			}
			return func(user reflect.Value) (bool, error) {
				s := info.asString(user)
				if s == "" {
					return false, nil
				}
				f1, err := parseFloat(s)
				if err != nil || math.IsNaN(f1) {
					return false, err
				}
				return cmp(f1, f) == needTrue, nil
			}
		}
	case wireconfig.OpOneOfSensitive:
		separated := splitFields(rule.ComparisonValue)
		set := make(map[[sha1.Size]byte]bool)
		for _, item := range separated {
			var hash [sha1.Size]byte
			h, err := hex.DecodeString(strings.TrimSpace(item))
			if err != nil || len(h) != sha1.Size {
				// It can never match.
				continue
			}
			copy(hash[:], h)
			set[hash] = true
		}
		return func(user reflect.Value) (bool, error) {
			b := info.asBytes(user)
			if len(b) == 0 {
				return false, nil
			}
			hash := sha1.Sum(b)
			return set[hash] == needTrue, nil
		}
	default:
		// TODO log if this happens?
		return alwaysFalseMatcher
	}
}

// intMatch parses s as a signed integer and
// returns a value (either float64 or int64)
// to compare it against. It only returns float64
// when the match cannot be exact for the given type.
func intMatch(s string, t reflect.Type) (val interface{}, _ error) {
	// Try for exact parsing, but fall back to float parsing.
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, err := parseFloat(s)
		if err != nil {
			return nil, err
		}
		i = int64(f)
		if float64(i) != f {
			// Doesn't fit exactly into an int64.
			return f, nil
		}
	}
	if reflect.Zero(t).OverflowInt(i) {
		return float64(i), nil
	}
	return i, nil
}

// uintMatch parses s as an unsigned integer and
// returns a value (either float64 or int64)
// to compare it against. It only returns float64
// when the match cannot be exact for the given type.
func uintMatch(s string, t reflect.Type) (val interface{}, _ error) {
	// Try for exact parsing, but fall back to float parsing.
	i, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		f, err := parseFloat(s)
		if err != nil {
			return nil, err
		}
		i = uint64(f)
		if float64(i) != f {
			// Doesn't fit exactly into a uint64.
			return f, nil
		}
	}
	if reflect.Zero(t).OverflowUint(i) {
		return float64(i), nil
	}
	return i, nil
}

// parseFloat parses a float allowing comma as a decimal point.
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.Replace(s, ",", ".", -1), 64)
}

type userTypeInfo struct {
	fields       map[string]attrInfo
	getAttribute func(v reflect.Value, attr string) string
	deref        bool
}

type attrInfo struct {
	kind     fieldKind
	ftype    reflect.Type
	index    []int
	asString func(v reflect.Value) string
	asBytes  func(v reflect.Value) []byte
	asSemver func(v reflect.Value) (*semver.Version, error)
}

type fieldKind int

const (
	kindText fieldKind = iota
	kindInt
	kindUint
	kindFloat
)

func newUserTypeInfo(userType reflect.Type) (*userTypeInfo, error) {
	if userType == nil {
		return nil, nil
	}
	if userType.Implements(getAttributeType) {
		return &userTypeInfo{
			getAttribute: func(v reflect.Value, attr string) string {
				return v.Interface().(UserAttributes).GetAttribute(attr)
			},
		}, nil
	}
	if userType.Kind() != reflect.Ptr || userType.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("user type that does not implement UserAttributes must be pointer to struct, not %v", userType)
	}
	userType = userType.Elem()
	tinfo := &userTypeInfo{
		deref:  true,
		fields: make(map[string]attrInfo),
	}
	for _, f := range visibleFields(userType) {
		f := f
		if f.PkgPath != "" || f.Anonymous {
			continue
		}
		if f.Type == stringMapType {
			// Should we return an error if there are two map fields?
			if tinfo.getAttribute != nil {
				return nil, fmt.Errorf("two map-typed fields")
			}
			tinfo.getAttribute = func(v reflect.Value, attr string) string {
				return v.FieldByIndex(f.Index).Interface().(map[string]string)[attr]
			}
			continue
		}
		fieldName := f.Name
		tag := f.Tag.Get("configcat")
		if tag == "-" {
			continue
		}
		if tag != "" {
			fieldName = tag
		}
		if _, ok := tinfo.fields[fieldName]; ok {
			return nil, fmt.Errorf("ambiguous attribute %q in user value of type %T", fieldName, userType)
		}
		info, err := attrInfoForStructField(f)
		if err != nil {
			return nil, err
		}
		tinfo.fields[fieldName] = info
	}
	return tinfo, nil
}

func attrInfoForStructField(field reflect.StructField) (attrInfo, error) {
	if field.Type.Implements(stringerType) {
		return attrInfo{
			ftype: field.Type,
			asString: func(v reflect.Value) string {
				return v.FieldByIndex(field.Index).Interface().(fmt.Stringer).String()
			},
			asBytes: func(v reflect.Value) []byte {
				return []byte(v.FieldByIndex(field.Index).Interface().(fmt.Stringer).String())
			},
			asSemver: func(v reflect.Value) (*semver.Version, error) {
				return parseSemver(v.FieldByIndex(field.Index).Interface().(fmt.Stringer).String())
			},
		}, nil
	}
	if reflect.PtrTo(field.Type).Implements(stringerType) {
		return attrInfo{
			ftype: field.Type,
			asString: func(v reflect.Value) string {
				return v.FieldByIndex(field.Index).Addr().Interface().(fmt.Stringer).String()
			},
			asBytes: func(v reflect.Value) []byte {
				return []byte(v.FieldByIndex(field.Index).Addr().Interface().(fmt.Stringer).String())
			},
			asSemver: func(v reflect.Value) (*semver.Version, error) {
				return parseSemver(v.FieldByIndex(field.Index).Addr().Interface().(fmt.Stringer).String())
			},
		}, nil
	}
	switch field.Type.Kind() {
	case reflect.String:
		return attrInfo{
			ftype: field.Type,
			asString: func(v reflect.Value) string {
				return v.FieldByIndex(field.Index).String()
			},
			asBytes: func(v reflect.Value) []byte {
				return []byte(v.FieldByIndex(field.Index).String())
			},
			asSemver: func(v reflect.Value) (*semver.Version, error) {
				return parseSemver(v.FieldByIndex(field.Index).String())
			},
		}, nil
	case reflect.Slice:
		if field.Type.Elem().Kind() == reflect.Uint8 {
			return attrInfo{
				ftype: field.Type,
				asString: func(v reflect.Value) string {
					return string(v.FieldByIndex(field.Index).Bytes())
				},
				asBytes: func(v reflect.Value) []byte {
					return v.FieldByIndex(field.Index).Bytes()
				},
				asSemver: func(v reflect.Value) (*semver.Version, error) {
					return parseSemver(string(v.FieldByIndex(field.Index).Bytes()))
				},
			}, nil
		}
		return attrInfo{}, fmt.Errorf("user value field %s has unsupported slice type %s", field.Name, field.Type)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return attrInfo{
			kind:  kindInt,
			ftype: field.Type,
			index: field.Index,
			asString: func(v reflect.Value) string {
				return strconv.FormatInt(v.FieldByIndex(field.Index).Int(), 10)
			},
			asBytes: func(v reflect.Value) []byte {
				return strconv.AppendInt(nil, v.FieldByIndex(field.Index).Int(), 10)
			},
		}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return attrInfo{
			kind:  kindUint,
			ftype: field.Type,
			index: field.Index,
			asString: func(v reflect.Value) string {
				return strconv.FormatUint(v.FieldByIndex(field.Index).Uint(), 10)
			},
			asBytes: func(v reflect.Value) []byte {
				return strconv.AppendUint(nil, v.FieldByIndex(field.Index).Uint(), 10)
			},
		}, nil
	case reflect.Float32, reflect.Float64:
		return attrInfo{
			kind:  kindFloat,
			ftype: field.Type,
			index: field.Index,
			asString: func(v reflect.Value) string {
				return strconv.FormatFloat(v.FieldByIndex(field.Index).Float(), 'g', -1, 64)
			},
			asBytes: func(v reflect.Value) []byte {
				return strconv.AppendFloat(nil, v.FieldByIndex(field.Index).Float(), 'g', -1, 64)
			},
		}, nil
	default:
		return attrInfo{}, fmt.Errorf("user value field %s has unsupported type %s", field.Name, field.Type)
	}
}

// attrInfo returns information on the attribute with the given name.
func (tinfo *userTypeInfo) attrInfo(name string) attrInfo {
	info, ok := tinfo.fields[name]
	if ok {
		return info
	}
	if tinfo.getAttribute == nil {
		return attrInfo{
			asString: func(v reflect.Value) string {
				return ""
			},
			asBytes: func(v reflect.Value) []byte {
				return nil
			},
		}
	}
	return attrInfo{
		asString: func(v reflect.Value) string {
			return tinfo.getAttribute(v, name)
		},
		asBytes: func(v reflect.Value) []byte {
			return []byte(tinfo.getAttribute(v, name))
		},
		asSemver: func(v reflect.Value) (*semver.Version, error) {
			return parseSemver(tinfo.getAttribute(v, name))
		},
	}
}

func parseSemver(s string) (*semver.Version, error) {
	if s == "" {
		return nil, nil
	}
	vers, err := semver.Parse(s)
	if err != nil {
		return nil, err
	}
	return &vers, nil
}

// uninvert reduces the set of operations we need to consider
// by returning only non-negative operations and less-than comparisons.
//
// For a negative or greater-than comparison, it returns
// the equivalent non-inverted operation and false.
func uninvert(op wireconfig.Operator) (_ wireconfig.Operator, needTrue bool) {
	switch op {
	case wireconfig.OpNotOneOf:
		return wireconfig.OpOneOf, false
	case wireconfig.OpNotContains:
		return wireconfig.OpContains, false
	case wireconfig.OpGreaterSemver:
		return wireconfig.OpLessEqSemver, false
	case wireconfig.OpGreaterEqSemver:
		return wireconfig.OpLessSemver, false
	case wireconfig.OpNotOneOfSemver:
		return wireconfig.OpOneOfSemver, false
	case wireconfig.OpNotEqNum:
		return wireconfig.OpEqNum, false
	case wireconfig.OpGreaterNum:
		return wireconfig.OpLessEqNum, false
	case wireconfig.OpGreaterEqNum:
		return wireconfig.OpLessNum, false
	case wireconfig.OpNotOneOfSensitive:
		return wireconfig.OpOneOfSensitive, false
	}
	return op, true
}

func errorMatcher(err error) func(reflect.Value) (bool, error) {
	return func(reflect.Value) (bool, error) {
		return false, err
	}
}

func alwaysFalseMatcher(reflect.Value) (bool, error) {
	return false, nil
}

func alwaysTrueMatcher(reflect.Value) (bool, error) {
	return false, nil
}

func falseMatcher(needTrue bool) func(reflect.Value) (bool, error) {
	return trueMatcher(!needTrue)
}

func trueMatcher(needTrue bool) func(reflect.Value) (bool, error) {
	if needTrue {
		return alwaysTrueMatcher
	}
	return alwaysFalseMatcher
}

type keyValue struct {
	key   string
	value interface{}
}

func keyValuesForRootNode(root *wireconfig.RootNode) map[string]keyValue {
	m := make(map[string]keyValue)
	add := func(variationID string, key string, value interface{}) {
		if _, ok := m[variationID]; !ok {
			m[variationID] = keyValue{
				key:   key,
				value: value,
			}
		}
	}
	for key, entry := range root.Entries {
		add(entry.VariationID, key, entry.Value)
		for _, rule := range entry.RolloutRules {
			add(rule.VariationID, key, rule.Value)
		}
		for _, rule := range entry.PercentageRules {
			add(rule.VariationID, key, rule.Value)
		}
	}
	return m
}

func keysForRootNode(root *wireconfig.RootNode) []string {
	keys := make([]string, 0, len(root.Entries))
	for k := range root.Entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func splitFields(s string) []string {
	fields := strings.Split(s, ",")
	for i, field := range fields {
		fields[i] = strings.TrimSpace(field)
	}
	return fields
}
