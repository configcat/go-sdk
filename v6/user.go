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

	if email != "" {
		user.attributes["Email"] = email
	}
	if country != "" {
		user.attributes["Country"] = country
	}
	for k, v := range custom {
		user.attributes[k] = v
	}
	return user
}

// GetAttribute retrieves a user attribute identified by a key.
func (user *User) GetAttribute(key string) string {
	return user.attributes[key]
}
