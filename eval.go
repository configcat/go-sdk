package configcat

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

const (
	identifierAttr     = "Identifier"
	ruleIgnoredMessage = "The current targeting rule is ignored and the evaluation continues with the next rule."
)

var (
	getAttributeType = reflect.TypeOf((*UserAttributes)(nil)).Elem()
	anyMapType       = reflect.TypeOf(map[string]interface{}(nil))
	timeType         = reflect.TypeOf(time.Time{})
)

type userAttrMissingError struct {
	attr string
}

type userAttrError struct {
	err  error
	attr string
}

func (u userAttrMissingError) Error() string {
	return fmt.Sprintf("cannot evaluate, the User.%s attribute is missing", u.attr)
}

func (u userAttrError) Error() string {
	return fmt.Sprintf("cannot evaluate, the User.%s attribute is invalid (%s)", u.attr, u.err.Error())
}

type settingEvalFunc = func(id keyID, user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (valueID, string, *TargetingRule, *PercentageOption)

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
	settingType := setting.Type
	keyBytes := setting.keyBytes
	attr := setting.PercentageOptionsAttribute
	percentageOptions := setting.PercentageOptions
	conditionMatchers := make([]func(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (bool, error), len(rules))
	for i, rule := range rules {
		conditionMatchers[i] = conditionsMatcher(rule.Conditions, evaluators, salt, setting.keyBytes)
	}

	return func(_ keyID, user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger) (valueID, string, *TargetingRule, *PercentageOption) {
		if builder != nil {
			builder.append(fmt.Sprintf("Evaluating '%s'", string(keyBytes)))
			if builder.user != nil && info != nil {
				builder.append(fmt.Sprintf(" for User '%s'", builder.userAsString()))
			}
		}
		userMissingErrorLogged := false
		if builder != nil && len(rules) > 0 {
			builder.newLineString("Evaluating targeting rules and applying the first match if any:")
		}

		for i, matcher := range conditionMatchers {
			rule := rules[i]
			matched, err := matcher(user, info, builder, logger)
			if builder != nil {
				builder.incIndent().newLineString("THEN ")
				if rule.ServedValue != nil {
					builder.append(fmt.Sprintf("'%v'", valueForSettingType(rule.ServedValue.Value, settingType)))
				} else if len(rule.PercentageOptions) > 0 {
					builder.append("%% options")
				}
				if err != nil {
					builder.append(fmt.Sprintf(" => %s", err.Error()))
				} else if matched {
					builder.append(" => MATCH, applying rule")
				} else if !matched {
					builder.append(" => no match")
				}
				builder.decIndent()
			}
			if !matched || err != nil {
				if err != nil {
					var noUserErr *noUserError
					var attrMissing *userAttrMissingError
					var attrErr *userAttrError
					var cmpValErr *comparisonValueError
					switch {
					case errors.As(err, &noUserErr) && !userMissingErrorLogged:
						logger.Warnf(3001, "cannot evaluate targeting rules and %% options for setting '%s' (User Object is missing); you should pass a User Object to the evaluation methods like `GetValue()` in order to make targeting work properly; read more: https://configcat.com/docs/advanced/user-object/", string(keyBytes))
						userMissingErrorLogged = true
					case errors.As(err, &attrMissing):
						logger.Warnf(3003, "cannot evaluate certain targeting rules of setting '%s' (the User.%s attribute is missing); you should set the User.%s attribute in order to make targeting work properly; read more: https://configcat.com/docs/advanced/user-object/", string(keyBytes), attrMissing.attr, attrMissing.attr)
					case errors.As(err, &attrErr):
						logger.Warnf(3004, "cannot evaluate certain targeting rules of setting '%s' (the User.%s attribute is invalid (%s)); please check the User.%s attribute and make sure that its value corresponds to the comparison operator", string(keyBytes), attrErr.attr, attrErr.err.Error(), attrErr.attr)
					case errors.As(err, &cmpValErr):
						logger.Warnf(3004, "cannot evaluate certain targeting rules of setting '%s' (%s)", string(keyBytes), cmpValErr.Error())
					}
					if builder != nil {
						builder.
							incIndent().
							newLineString(ruleIgnoredMessage).
							decIndent()
					}
				}
				continue
			}
			if rule.ServedValue != nil {
				if builder != nil {
					builder.newLine().append(fmt.Sprintf("Returning '%v'.", valueForSettingType(rule.ServedValue.Value, settingType)))
				}
				return rule.ServedValue.valueID, rule.ServedValue.VariationID, rule, nil
			}
			if len(rule.PercentageOptions) > 0 {
				if builder != nil {
					builder.incIndent()
				}
				if info == nil && !userMissingErrorLogged {
					logger.Warnf(3001, "cannot evaluate targeting rules and %% options for setting '%s' (User Object is missing); you should pass a User Object to the evaluation methods like `GetValue()` in order to make targeting work properly; read more: https://configcat.com/docs/advanced/user-object/", string(keyBytes))
					userMissingErrorLogged = true
				}
				matchedOption := evalPercentageOptions(user, info, builder, logger, settingType, attr, keyBytes, rule.PercentageOptions)
				if matchedOption != nil {
					if builder != nil {
						builder.decIndent()
						if builder != nil {
							builder.newLine().append(fmt.Sprintf("Returning '%v'.", valueForSettingType(matchedOption.Value, settingType)))
						}
					}
					return matchedOption.valueID, matchedOption.VariationID, nil, matchedOption
				} else {
					if builder != nil {
						builder.
							newLineString(ruleIgnoredMessage).
							decIndent()
					}
				}
			}
		}
		if len(percentageOptions) > 0 {
			if info == nil && !userMissingErrorLogged {
				logger.Warnf(3001, "cannot evaluate targeting rules and %% options for setting '%s' (User Object is missing); you should pass a User Object to the evaluation methods like `GetValue()` in order to make targeting work properly; read more: https://configcat.com/docs/advanced/user-object/", string(keyBytes))
				userMissingErrorLogged = true
			}
			matchedOption := evalPercentageOptions(user, info, builder, logger, settingType, attr, keyBytes, percentageOptions)
			if matchedOption != nil {
				if builder != nil {
					builder.newLine().append(fmt.Sprintf("Returning '%v'.", valueForSettingType(matchedOption.Value, settingType)))
				}
				return matchedOption.valueID, matchedOption.VariationID, nil, matchedOption
			}
		}
		if builder != nil {
			builder.newLine().append(fmt.Sprintf("Returning '%v'.", valueForSettingType(setting.Value, settingType)))
		}
		return setting.valueID, setting.VariationID, nil, nil
	}
}

func evalPercentageOptions(user reflect.Value, info *userTypeInfo, builder *evalLogBuilder, logger *leveledLogger, settingType SettingType, percentageAttr string, settingKey []byte, percentageOptions []*PercentageOption) *PercentageOption {
	if info == nil {
		if builder != nil {
			builder.newLineString("Skipping %% options because the User Object is missing.")
		}
		return nil
	}
	if percentageAttr == "" {
		percentageAttr = identifierAttr
	}
	attrBytes, err := info.getBytes(user, percentageAttr)
	if percentageAttr == identifierAttr && len(attrBytes) == 0 {
		attrBytes = []byte("")
	} else if err != nil {
		var attrMissing *userAttrMissingError
		switch {
		case errors.As(err, &attrMissing):
			logger.Warnf(3003, "cannot evaluate %% options for setting '%s' (the User.%s attribute is missing); you should set the User.%s attribute in order to make targeting work properly; read more: https://configcat.com/docs/advanced/user-object/", string(settingKey), percentageAttr, percentageAttr)
		}
		if builder != nil {
			builder.newLineString("Skipping %% options because the User." + percentageAttr + " attribute is missing.")
		}
		return nil
	}
	if builder != nil {
		builder.newLineString("Evaluating %% options based on the User." + percentageAttr + " attribute:")
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
	if builder != nil {
		builder.newLineString(fmt.Sprintf("- Computing hash in the [0..99] range from User.%s => %d (this value is sticky and consistent across all SDKs)", percentageAttr, scaled))
	}
	bucket := int64(0)
	for i, option := range percentageOptions {
		bucket += option.Percentage
		if scaled < bucket {
			if builder != nil {
				builder.newLineString("- Hash value " + strconv.FormatInt(scaled, 10) + " selects %% option " + strconv.Itoa(i+1) + " (" + strconv.FormatInt(option.Percentage, 10) + "%%), '" + fmt.Sprintf("%v", valueForSettingType(option.Value, settingType)) + "'.")
			}
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
	asString      func(v reflect.Value) string
	asBytes       func(v reflect.Value) []byte
	asSemver      func(v reflect.Value) (*semver.Version, error)
	asFloat       func(v reflect.Value) (float64, error)
	asStringSlice func(v reflect.Value) ([]string, error)
}

func (c *config) getOrNewUserTypeInfo(userType reflect.Type) (*userTypeInfo, error) {
	if info, ok := c.userInfos.Load(userType); ok {
		return info.(*userTypeInfo), nil
	}
	info, err := newUserTypeInfo(userType)
	if err != nil {
		return nil, err
	}
	res, _ := c.userInfos.LoadOrStore(userType, info)
	return res.(*userTypeInfo), nil
}

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
	if userType == anyMapType {
		return &userTypeInfo{
			getAttribute: func(v reflect.Value, attr string) interface{} {
				return v.Interface().(map[string]interface{})[attr]
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
			asStringSlice: func(v reflect.Value) ([]string, error) {
				return parseStringSliceJson(v.FieldByIndex(field.Index).String())
			},
		}, nil
	case reflect.Slice:
		if field.Type.Elem().Kind() == reflect.Uint8 {
			return attrInfo{
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
				asStringSlice: func(v reflect.Value) ([]string, error) {
					return parseStringSliceJson(string(v.FieldByIndex(field.Index).Bytes()))
				},
			}, nil
		} else if field.Type.Elem().Kind() == reflect.String {
			return attrInfo{
				asStringSlice: func(v reflect.Value) ([]string, error) {
					sl := v.FieldByIndex(field.Index)
					res := make([]string, sl.Len())
					for i := 0; i < sl.Len(); i++ {
						res[i] = sl.Index(i).String()
					}
					return res, nil
				},
			}, nil
		}
		return attrInfo{}, fmt.Errorf("user value field %s has unsupported slice type %s", field.Name, field.Type)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return attrInfo{
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
	} else if t.getAttribute != nil {
		res := t.getAttribute(v, attr)
		if res == nil {
			return "", &userAttrMissingError{attr: attr}
		}
		switch val := res.(type) {
		case string:
			result = val
		case []byte:
			result = string(val)
		case float32:
			result = strconv.FormatFloat(float64(val), 'g', -1, 64)
		case float64:
			result = strconv.FormatFloat(val, 'g', -1, 64)
		case int:
			result = strconv.FormatInt(int64(val), 10)
		case uint:
			result = strconv.FormatUint(uint64(val), 10)
		case int8:
			result = strconv.FormatInt(int64(val), 10)
		case uint8:
			result = strconv.FormatUint(uint64(val), 10)
		case int16:
			result = strconv.FormatInt(int64(val), 10)
		case uint16:
			result = strconv.FormatUint(uint64(val), 10)
		case int32:
			result = strconv.FormatInt(int64(val), 10)
		case uint32:
			result = strconv.FormatUint(uint64(val), 10)
		case int64:
			result = strconv.FormatInt(val, 10)
		case uint64:
			result = strconv.FormatUint(val, 10)
		case uintptr:
			result = strconv.FormatUint(uint64(val), 10)
		}
	}
	if len(result) == 0 {
		return "", &userAttrMissingError{attr: attr}
	}
	return result, nil
}

func (t *userTypeInfo) getBytes(v reflect.Value, attr string) ([]byte, error) {
	var result []byte
	info, ok := t.fields[attr]
	if ok && info.asBytes != nil {
		result = info.asBytes(v)
	} else if t.getAttribute != nil {
		res := t.getAttribute(v, attr)
		if res == nil {
			return nil, &userAttrMissingError{attr: attr}
		}
		switch val := res.(type) {
		case string:
			result = []byte(val)
		case []byte:
			result = val
		case float32:
			result = strconv.AppendFloat(nil, float64(val), 'g', -1, 64)
		case float64:
			result = strconv.AppendFloat(nil, val, 'g', -1, 64)
		case int:
			result = strconv.AppendInt(nil, int64(val), 10)
		case uint:
			result = strconv.AppendUint(nil, uint64(val), 10)
		case int8:
			result = strconv.AppendInt(nil, int64(val), 10)
		case uint8:
			result = strconv.AppendUint(nil, uint64(val), 10)
		case int16:
			result = strconv.AppendInt(nil, int64(val), 10)
		case uint16:
			result = strconv.AppendUint(nil, uint64(val), 10)
		case int32:
			result = strconv.AppendInt(nil, int64(val), 10)
		case uint32:
			result = strconv.AppendUint(nil, uint64(val), 10)
		case int64:
			result = strconv.AppendInt(nil, val, 10)
		case uint64:
			result = strconv.AppendUint(nil, val, 10)
		case uintptr:
			result = strconv.AppendUint(nil, uint64(val), 10)
		}
	}
	if len(result) == 0 {
		return nil, &userAttrMissingError{attr: attr}
	}
	return result, nil
}

func (t *userTypeInfo) getSemver(v reflect.Value, attr string) (*semver.Version, error) {
	info, ok := t.fields[attr]
	if ok && info.asSemver != nil {
		ver, err := info.asSemver(v)
		if err != nil {
			return nil, &userAttrError{attr: attr, err: err}
		}
		return ver, nil
	} else if t.getAttribute != nil {
		if res, ok := t.getAttribute(v, attr).([]byte); ok {
			ver, err := parseSemver(string(res))
			if err != nil {
				return nil, &userAttrError{attr: attr, err: err}
			}
			return ver, nil
		}
		if res, ok := t.getAttribute(v, attr).(string); ok {
			ver, err := parseSemver(res)
			if err != nil {
				return nil, &userAttrError{attr: attr, err: err}
			}
			return ver, nil
		}
	}
	return nil, &userAttrMissingError{attr: attr}
}

func (t *userTypeInfo) getFloat(v reflect.Value, attr string) (float64, error) {
	info, ok := t.fields[attr]
	if ok && info.asFloat != nil {
		res, err := info.asFloat(v)
		if err != nil {
			return 0, &userAttrError{attr: attr, err: err}
		}
		return res, nil
	} else if t.getAttribute != nil {
		val := t.getAttribute(v, attr)
		switch val := val.(type) {
		case float64:
			return val, nil
		case float32:
			return float64(val), nil
		case string:
			res, err := parseFloat(val)
			if err != nil {
				return 0, &userAttrError{attr: attr, err: err}
			}
			return res, nil
		case []byte:
			res, err := parseFloat(string(val))
			if err != nil {
				return 0, &userAttrError{attr: attr, err: err}
			}
			return res, nil
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
			return 0, &userAttrError{attr: attr, err: fmt.Errorf("cannot convert '%v' to float64", val)}
		}
	}
	return 0, &userAttrMissingError{attr: attr}
}

func (t *userTypeInfo) getSlice(v reflect.Value, attr string) ([]string, error) {
	info, ok := t.fields[attr]
	if ok && info.asStringSlice != nil {
		val, err := info.asStringSlice(v)
		if err != nil {
			return nil, &userAttrError{attr: attr, err: err}
		}
		return val, nil
	} else if t.getAttribute != nil {
		val := t.getAttribute(v, attr)
		switch val := val.(type) {
		case []string:
			return val, nil
		case string:
			res, err := parseStringSliceJson(val)
			if err != nil {
				return nil, &userAttrError{attr: attr, err: err}
			}
			return res, nil
		case []byte:
			res, err := parseStringSliceJson(string(val))
			if err != nil {
				return nil, &userAttrError{attr: attr, err: err}
			}
			return res, nil
		default:
			return nil, &userAttrError{attr: attr, err: fmt.Errorf("cannot convert '%v' to []string", val)}
		}
	}
	return nil, &userAttrMissingError{attr: attr}
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

func parseStringSliceJson(s string) ([]string, error) {
	var res []string
	err := json.Unmarshal([]byte(s), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
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
