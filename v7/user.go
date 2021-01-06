package configcat

// The User interface represents the user-specific data that can influence
// configcat rule evaluation. All Users are expected to provide an Identifier
// attribute.
//
// The configcat client uses reflection to determine
// what attributes are available:
//
// If the User value implements UserAttributes, then that
// method will be used to retrieve attributes.
//
// Otherwise the implementation is expected to be a pointer to a struct
// type. Each public field in the struct is treated as a possible comparison
// attribute.
//
// If a field's type implements a `String() string` method, the
// field will be treated as a textual and the String method will
// be called to determine the value.
//
// If a field's type is map[string] string, the map value is used to look
// up any custom attribute not found directly in the struct.
// There should be at most one of these fields.
//
// Otherwise, a field type must be a numeric type, a string type, a []byte type
// or a github.com/blang/semver.Version.
//
// If a rule uses an attribute that isn't available, that rule will be treated
// as non-matching.
type User interface{}

// UserAttributes can be implemented by a User value to implement
// support for getting arbitrary attributes.
type UserAttributes interface {
	GetAttribute(attr string) string
}

// UserData implements the User interface with the basic
// set of attributes. For an efficient way to use your own
// domain object as a User, see the documentation for the User
// interface.
type UserData struct {
	Identifier string
	Email      string
	Country    string
	Custom     map[string]string
}
