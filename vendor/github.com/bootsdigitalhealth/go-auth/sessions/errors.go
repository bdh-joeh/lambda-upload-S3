package sessions

type PasswordMismatch struct{}

func (e PasswordMismatch) Error() string {
	return "Password is incorrect"
}

type UserNotFound struct{}

func (e UserNotFound) Error() string {
	return "User not found"
}

type UserLockedError struct{}

func (e UserLockedError) Error() string {
	return "Account is locked"
}

type NonUniqueToken struct{}

func (e NonUniqueToken) Error() string {
	return "Generated token was not unique"
}
