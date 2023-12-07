package configcat

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

const (
	identifierAttr = "Identifier"
)

var (
	getAttributeType = reflect.TypeOf((*UserAttributes)(nil)).Elem()
	anyMapType       = reflect.TypeOf(map[string]interface{}(nil))
	timeType         = reflect.TypeOf((*time.Time)(nil))
)

type evalContext struct {
	logger         *leveledLogger
	evalLogBuilder *evalLogBuilder
	visitedKeys    map[keyID]string
	configJsonSalt []byte
	contextSalt    []byte
}

func newEvaluationContext(logger *leveledLogger, salt []byte) *evalContext {
	ctx := &evalContext{
		logger:         logger,
		visitedKeys:    make(map[keyID]string),
		configJsonSalt: salt,
	}
	if logger.enabled(LogLevelInfo) {
		ctx.evalLogBuilder = &evalLogBuilder{}
	}
	return ctx
}

type settingEvalFunc = func(id keyID, user reflect.Value, info *userTypeInfo) (valueID, string, *TargetingRule, *PercentageOption)

func (c *config) generateEvaluators() {
	// Allocate all key IDs.
	for key := range c.root.Settings {
		idForKey(key, true)
	}
	c.evaluators = make([]settingEvalFunc, numKeys())
	for key, setting := range c.root.Settings {
		c.evaluators[idForKey(key, true)] = settingEvaluator(setting, c.root.Preferences.saltBytes, c.evaluators)
	}
}

func settingEvaluator(setting *Setting, salt []byte, evaluators []settingEvalFunc) settingEvalFunc {
	rules := setting.TargetingRules
	keyBytes := setting.keyBytes
	attr := setting.PercentageOptionsAttribute
	percentageOptions := setting.PercentageOptions
	conditionMatchers := make([]func(user reflect.Value, info *userTypeInfo) (bool, error), len(rules))
	for i, rule := range rules {
		conditionMatchers[i] = conditionsMatcher(rule.Conditions, evaluators, salt, setting.keyBytes)
	}

	return func(_ keyID, user reflect.Value, info *userTypeInfo) (valueID, string, *TargetingRule, *PercentageOption) {
		for i, matcher := range conditionMatchers {
			matched, err := matcher(user, info)
			if !matched || err != nil {
				continue
			}
			rule := rules[i]
			if rule.ServedValue != nil {
				return rule.ServedValue.valueID, rule.ServedValue.VariationID, rule, nil
			}
			if len(rule.PercentageOptions) > 0 {
				matchedOption := evalPercentageOptions(user, info, attr, keyBytes, rule.PercentageOptions)
				if matchedOption != nil {
					return matchedOption.valueID, matchedOption.VariationID, nil, matchedOption
				}
			}
		}
		if len(percentageOptions) > 0 {
			matchedOption := evalPercentageOptions(user, info, attr, keyBytes, percentageOptions)
			if matchedOption != nil {
				return matchedOption.valueID, matchedOption.VariationID, nil, matchedOption
			}
		}
		return setting.valueID, setting.VariationID, nil, nil
	}
}

func evalPercentageOptions(user reflect.Value, info *userTypeInfo, percentageAttr string, settingKey []byte, percentageOptions []*PercentageOption) *PercentageOption {
	if info == nil {
		return nil
	}
	if percentageAttr == "" {
		percentageAttr = identifierAttr
	}
	attrBytes, err := info.getBytes(user, percentageAttr)
	if percentageAttr == identifierAttr && len(attrBytes) == 0 {
		attrBytes = []byte("")
	} else if err != nil {
		return nil
	}
	hashKey := make([]byte, len(settingKey)+len(attrBytes))
	copy(hashKey, settingKey)
	copy(hashKey[len(settingKey):], attrBytes)
	sum := sha1.Sum(hashKey)
	// Treat the first 4 bytes as a number, then knock
	// off the last 4 bits. This is equivalent to turning the
	// entire sum into hex, then decoding the first 7 digits.
	num := int64(binary.BigEndian.Uint32(sum[:4]))
	num >>= 4

	scaled := num % 100
	bucket := int64(0)
	for _, option := range percentageOptions {
		bucket += option.Percentage
		if scaled < bucket {
			return option
		}
	}
	return nil
}

// parseFloat parses a float allowing comma as a decimal point.
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.Replace(s, ",", ".", -1), 64)
}

type userTypeInfo struct {
	fields       map[string]attrInfo
	getAttribute func(v reflect.Value, attr string) interface{}
	deref        bool
}

type attrInfo struct {
	kind          fieldKind
	ftype         reflect.Type
	index         []int
	asString      func(v reflect.Value) string
	asBytes       func(v reflect.Value) []byte
	asSemver      func(v reflect.Value) (*semver.Version, error)
	asFloat       func(v reflect.Value) (float64, error)
	asStringSlice func(v reflect.Value) []string
}

type fieldKind int

const (
	kindText fieldKind = iota
	kindInt
	kindUint
	kindFloat
	kindSlice
	kindStruct
)

func newUserTypeInfo(userType reflect.Type) (*userTypeInfo, error) {
	if userType == nil {
		return nil, nil
	}
	if userType.Implements(getAttributeType) {
		return &userTypeInfo{
			getAttribute: func(v reflect.Value, attr string) interface{} {
				return v.Interface().(UserAttributes).GetAttribute(attr)
			},
		}, nil
	}
	if userType.Kind() != reflect.Ptr || userType.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("user type that does not implement UserAttributes must be pointer to struct, not %v", userType)
	}
	userType = userType.Elem()
	typeInfo := &userTypeInfo{
		deref:  true,
		fields: make(map[string]attrInfo),
	}
	for _, f := range visibleFields(userType) {
		f := f
		if f.PkgPath != "" || f.Anonymous {
			continue
		}
		if f.Type == anyMapType {
			// Should we return an error if there are two map fields?
			if typeInfo.getAttribute != nil {
				return nil, fmt.Errorf("two map-typed fields")
			}
			typeInfo.getAttribute = func(v reflect.Value, attr string) interface{} {
				return v.FieldByIndex(f.Index).Interface().(map[string]interface{})[attr]
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
		if _, ok := typeInfo.fields[fieldName]; ok {
			return nil, fmt.Errorf("ambiguous attribute %q in user value of type %T", fieldName, userType)
		}
		info, err := attrInfoForStructField(f)
		if err != nil {
			return nil, err
		}
		typeInfo.fields[fieldName] = info
	}
	return typeInfo, nil
}

func attrInfoForStructField(field reflect.StructField) (attrInfo, error) {
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
			asFloat: func(v reflect.Value) (float64, error) {
				return parseFloat(v.FieldByIndex(field.Index).String())
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
				asFloat: func(v reflect.Value) (float64, error) {
					return parseFloat(string(v.FieldByIndex(field.Index).Bytes()))
				},
			}, nil
		} else if field.Type.Elem().Kind() == reflect.String {
			return attrInfo{
				kind: kindSlice,
				asStringSlice: func(v reflect.Value) []string {
					sl := v.FieldByIndex(field.Index)
					res := make([]string, sl.Len())
					for i := 0; i < sl.Len(); i++ {
						res[i] = sl.Index(i).String()
					}
					return res
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
			asFloat: func(v reflect.Value) (float64, error) {
				return float64(v.FieldByIndex(field.Index).Int()), nil
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
			asFloat: func(v reflect.Value) (float64, error) {
				return float64(v.FieldByIndex(field.Index).Uint()), nil
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
			asFloat: func(v reflect.Value) (float64, error) {
				return v.FieldByIndex(field.Index).Float(), nil
			},
		}, nil
	case reflect.Struct:
		if field.Type == timeType {
			return attrInfo{
				kind:  kindFloat,
				ftype: field.Type,
				index: field.Index,
				asFloat: func(v reflect.Value) (float64, error) {
					return float64(v.FieldByIndex(field.Index).Interface().(time.Time).UnixMilli() / 1000), nil
				},
			}, nil
		}
		return attrInfo{}, fmt.Errorf("user value field %s has unsupported type %s", field.Name, field.Type)
	default:
		return attrInfo{}, fmt.Errorf("user value field %s has unsupported type %s", field.Name, field.Type)
	}
}

func (t *userTypeInfo) getString(v reflect.Value, attr string) (string, error) {
	var result string
	info, ok := t.fields[attr]
	if ok && info.asString != nil {
		result = info.asString(v)
	}
	if t.getAttribute != nil {
		if res, ok := t.getAttribute(v, attr).(string); ok {
			result = res
		}
		if res, ok := t.getAttribute(v, attr).([]byte); ok {
			result = string(res)
		}
	}
	if len(result) == 0 {
		return "", fmt.Errorf("user attribute '%s' not found", attr)
	}
	return result, nil
}

func (t *userTypeInfo) getBytes(v reflect.Value, attr string) ([]byte, error) {
	var result []byte
	info, ok := t.fields[attr]
	if ok && info.asBytes != nil {
		result = info.asBytes(v)
	}
	if t.getAttribute != nil {
		if res, ok := t.getAttribute(v, attr).([]byte); ok {
			result = res
		}
		if res, ok := t.getAttribute(v, attr).(string); ok {
			result = []byte(res)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("user attribute '%s' not found", attr)
	}
	return result, nil
}

func (t *userTypeInfo) getSemver(v reflect.Value, attr string) (*semver.Version, error) {
	info, ok := t.fields[attr]
	if ok && info.asSemver != nil {
		return info.asSemver(v)
	}
	if t.getAttribute != nil {
		if res, ok := t.getAttribute(v, attr).([]byte); ok {
			return parseSemver(string(res))
		}
		if res, ok := t.getAttribute(v, attr).(string); ok {
			return parseSemver(res)
		}
	}
	return nil, fmt.Errorf("cannot use attribute '%s' as semantic version", attr)
}

func (t *userTypeInfo) getFloat(v reflect.Value, attr string) (float64, error) {
	info, ok := t.fields[attr]
	if ok {
		return info.asFloat(v)
	}
	if t.getAttribute != nil {
		val := t.getAttribute(v, attr)
		switch val := val.(type) {
		case float64:
			return val, nil
		case string:
			return parseFloat(val)
		case []byte:
			return parseFloat(string(val))
		case int:
			return float64(val), nil
		case uint:
			return float64(val), nil
		case int8:
			return float64(val), nil
		case uint8:
			return float64(val), nil
		case int16:
			return float64(val), nil
		case uint16:
			return float64(val), nil
		case int32:
			return float64(val), nil
		case uint32:
			return float64(val), nil
		case int64:
			return float64(val), nil
		case uint64:
			return float64(val), nil
		case uintptr:
			return float64(val), nil
		case time.Time:
			return float64(val.UnixMilli() / 1000), nil
		default:
			return 0, fmt.Errorf("cannot convert '%v' to float64", val)
		}
	}
	return 0, fmt.Errorf("user attribute '%s' not found", attr)
}

func (t *userTypeInfo) getSlice(v reflect.Value, attr string) []string {
	info, ok := t.fields[attr]
	if ok {
		return info.asStringSlice(v)
	}
	if t.getAttribute != nil {
		val := t.getAttribute(v, attr)
		switch val := val.(type) {
		case []string:
			return val
		case string:
			var res []string
			err := json.Unmarshal([]byte(val), &res)
			if err != nil {
				return nil
			}
			return res
		default:
			return nil
		}
	}
	return nil
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

type keyValue struct {
	key   string
	value interface{}
}

func keyValuesForRootNode(root *ConfigJson) map[string]keyValue {
	m := make(map[string]keyValue)
	add := func(variationID string, key string, value interface{}) {
		if _, ok := m[variationID]; !ok {
			m[variationID] = keyValue{
				key:   key,
				value: value,
			}
		}
	}
	for key, setting := range root.Settings {
		add(setting.VariationID, key, fromSettingValue(setting.Type, setting.Value))
		for _, rule := range setting.TargetingRules {
			if rule.ServedValue != nil {
				add(rule.ServedValue.VariationID, key, fromSettingValue(setting.Type, rule.ServedValue.Value))
			}
			if len(rule.PercentageOptions) > 0 {
				for _, option := range rule.PercentageOptions {
					add(option.VariationID, key, fromSettingValue(setting.Type, option.Value))
				}
			}
		}
		for _, rule := range setting.PercentageOptions {
			add(rule.VariationID, key, fromSettingValue(setting.Type, rule.Value))
		}
	}
	return m
}

func keysForRootNode(root *ConfigJson) []string {
	keys := make([]string, 0, len(root.Settings))
	for k := range root.Settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fromSettingValue(settingType SettingType, v *SettingValue) interface{} {
	switch settingType {
	case BoolSetting:
		return v.BoolValue
	case StringSetting:
		return v.StringValue
	case IntSetting:
		return v.IntValue
	case FloatSetting:
		return v.DoubleValue
	default:
		return nil
	}
}
