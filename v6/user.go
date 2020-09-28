package configcat

// User is an object containing attributes to properly identify a given user for rollout evaluation.
type User struct {
	identifier string
	attributes map[string]string
}

// NewUser creates a new user object. The identifier argument is mandatory.
func NewUser(identifier string) *User {
	return NewUserWithAdditionalAttributes(identifier, "", "", map[string]string{})
}

// NewUserWithAdditionalAttributes creates a new user object with additional attributes. The identifier argument is mandatory.
func NewUserWithAdditionalAttributes(identifier string, email string, country string, custom map[string]string) *User {
	user := &User{identifier: identifier, attributes: map[string]string{}}
	user.attributes["Identifier"] = identifier

	if len(email) > 0 {
		user.attributes["Email"] = email
	}

	if len(country) > 0 {
		user.attributes["Country"] = country
	}

	if len(custom) > 0 {
		for k, v := range custom {
			user.attributes[k] = v
		}
	}

	return user
}

// GetAttribute retrieves a user attribute identified by a key.
func (user *User) GetAttribute(key string) string {
	val := user.attributes[key]
	if len(val) > 0 {
		return val
	}

	return ""
}
