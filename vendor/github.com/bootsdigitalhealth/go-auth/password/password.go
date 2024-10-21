package password

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"github.com/tredoe/osutil/user/crypt/sha512_crypt"
	"strings"
	"unicode"
)

/*
Hash combines input and created field (unix timestamp) and returns a hashed string and an
error.

Will return the same hash for a given (password, created) so use this when:
	1. Creating new hash
	2. Comparing existing hash
	3. Creating the user hash to find the list of sessions for a user
*/
func Hash(input interface{}, created int64) (string, error) {
	key := []byte(fmt.Sprint(input))
	salt := fmt.Sprintf("$6$rounds=5000$%v$", created)
	sha512Crypt := sha512_crypt.New()
	hash, err := sha512Crypt.Generate(key, []byte(salt))
	if err != nil {
		return "", err
	}
	return strings.Replace(hash, salt, "", 1), nil
}

/*
ValidatePassword checks to make sure the string contains:
	1. At least 10 characters
	2. At least 1 uppercase
	3. At least 1 lowercase
	4. At least 1 number
	5. At least 1 non-alphanumeric
*/
func ValidatePassword(password string) error {
	var containsUpper, containsLower, containsNumber, containsNonAlphaNumeric bool
	if len(password) < 10 {
		return errors.New("password must be at least 10 characters long")
	}
	for _, v := range password {
		containsUpper = containsUpper || unicode.IsUpper(v)
		containsLower = containsLower || unicode.IsLower(v)
		containsNumber = containsNumber || unicode.IsNumber(v)
		containsNonAlphaNumeric = containsNonAlphaNumeric || (!unicode.IsLetter(v) && !unicode.IsNumber(v))
	}
	if !containsUpper {
		return errors.New("password must contain an uppercase letter")
	}
	if !containsLower {
		return errors.New("password must contain a lowercase letter")
	}
	if !containsNumber {
		return errors.New("password must contain a number")
	}
	if !containsNonAlphaNumeric {
		return errors.New("password must contain a non-alphanumeric character")
	}
	return nil
}

// SecureCompare performs a constant time compare of two strings to limit timing attacks.
func SecureCompare(given string, actual string) bool {
	if subtle.ConstantTimeEq(int32(len(given)), int32(len(actual))) == 1 {
		return subtle.ConstantTimeCompare([]byte(given), []byte(actual)) == 1
	} else {
		/* Securely compare actual to itself to keep constant time, but always return false */
		return subtle.ConstantTimeCompare([]byte(actual), []byte(actual)) == 1 && false
	}
}
