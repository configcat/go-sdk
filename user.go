package configcat

// The User interface represents the user-specific data that can influence
// feature flag rule evaluation. All Users are expected to provide an Identifier
// attribute. A User value is assumed to be immutable once it's been
// provided to the SDK - if it changes between feature flag evaluations,
// there's no guarantee that the feature flag results will change accordingly
// (use WithUser instead of mutating the value).
//
// The ConfigCat client uses reflection to determine
// what attributes are available:
//
// If the User value implements UserAttributes, then that
// method will be used to retrieve attributes.
//
// Otherwise, the implementation is expected to be a pointer to a struct
// type. Each public field in the struct is treated as a possible comparison
// attribute.
//
// If a field's type is map[string]interface{}, the map value is used to look
// up any custom attribute not found directly in the struct.
// There should be at most one of these fields.
//
// Otherwise, a field type must be a numeric type, a string type, []string type, a []byte type, a time.Time type,
// or a github.com/blang/semver.Version.
//
// If a rule uses an attribute that isn't available, that rule will be treated
// as non-matching.
type User interface{}

// UserAttributes can be implemented by a User value to implement
// support for getting arbitrary attributes.
type UserAttributes interface {
	GetAttribute(attr string) interface{}
}

// UserData implements the User interface with the basic
// set of attributes. For an efficient way to use your own
// domain object as a User, see the documentation for the User
// interface.
type UserData struct {
	Identifier string
	Email      string
	Country    string
	Custom     map[string]interface{}
}
