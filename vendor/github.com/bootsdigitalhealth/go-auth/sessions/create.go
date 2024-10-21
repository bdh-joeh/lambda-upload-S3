package sessions

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/bootsdigitalhealth/go-auth/password"
	"github.com/bootsdigitalhealth/go-db/redis"
	"github.com/golang-jwt/jwt"
	"math/rand"
	"time"
)

const (
	sessionTTL       = 960
	userDeleted      = 2
	userLocked       = 4
	userLockedLogin  = 16
	maxLoginAttempts = 6
)

type RequestBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userForAuth struct {
	UserID         int64
	Status         int
	StoredPassword string
	Created        int64
}

type SessionData struct {
	redis.Session
	Status      int `json:"user_status"`
	UserCreated int64
}

// RefreshToken increases the timeout for the token so that the user is not logged out
func RefreshToken(redisClient *redis.Client, session redis.Session) error {
	var existingSession SessionData

	existingSessionJSON, err := redisClient.Get(session.Token).Bytes()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(existingSessionJSON, &existingSession); err != nil {
		return err
	}

	existingSession.Created = time.Now().Unix()

	updatedSessionData, err := json.Marshal(existingSession)
	if err != nil {
		return err
	}

	if redisClient.Set(session.Token, updatedSessionData, time.Second*sessionTTL).Err() != nil {
		return err
	}

	return nil
}

/*
	Create

1. Finds the user given a username and systemCode

2. Validates the user to make sure they are not locked out or deleted

3. Checks the passwords to see if they match

4. Updates User SQL if they passwords match

5. Creates token and session data

6. Saves session in Redis using token as key

7. Creates a unique token for a given user, and appends session tokens to array (for faster delete)

8. Adds sessions summary to the database
*/
func Create(db *sql.DB, redisClient *redis.Client, jwtSecretString string, body RequestBody, systemCode string) (string, error) {

	userForAuth, err := GetUserForAuth(db, body.Username, systemCode)
	if err != nil {
		return "", err
	}

	if err := ValidateUserStatus(userForAuth); err != nil {
		return "", err
	}

	hash, err := password.Hash(body.Password, userForAuth.Created)
	if err != nil {
		return "", errors.New("password hash failed")
	}

	if !password.SecureCompare(hash, userForAuth.StoredPassword) {

		err = updateUser(db, userForAuth.UserID, false)
		if err != nil {
			return "", err
		}

		return "", PasswordMismatch{}
	}

	token, sessionData, err := CreateTokenAndSessionData(db, userForAuth, jwtSecretString)
	if err != nil {
		return "", err
	}

	if err := Save(redisClient, token, sessionData); err != nil {
		return "", err
	}

	if err := updateUser(db, userForAuth.UserID, true); err != nil {
		return "", err
	}

	if err := CreateSessionSummary(db, token, sessionData); err != nil {
		return "", err
	}

	return token, nil
}

// CreateSessionSummary adds new token and session info into the database.
func CreateSessionSummary(db *sql.DB, token string, data *SessionData) error {
	stmt, err := db.Prepare("INSERT INTO session_summaries (token, user_id, started, last_active, created) VALUES (?, ?, ?, ?, UNIX_TIMESTAMP())")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(token, data.UserID, data.Created, data.Created)
	if err != nil {
		return err
	}
	return nil
}

// ValidateUserStatus ensures the user is not deleted or locked.
func ValidateUserStatus(user *userForAuth) error {
	if user.Status&userDeleted > 0 {
		return UserNotFound{}
	}
	if (user.Status&userLocked > 0) || (user.Status&userLockedLogin > 0) {
		return UserLockedError{}
	}
	return nil
}

// CreateTokenAndSessionData creates the token and session data for an authenticated user.
func CreateTokenAndSessionData(db *sql.DB, user *userForAuth, jwtSecretString string) (string, *SessionData, error) {
	rolesMap, err := getUserRoles(db, user.UserID)
	if err != nil {
		return "", &SessionData{}, err
	}
	session := newSessionData(user, rolesMap)
	token, err := createSessionToken(session, jwtSecretString)
	if err != nil {
		return "", &SessionData{}, err
	}
	return token, session, nil
}

func createSessionToken(session *SessionData, jwtSecretString string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": session.UserID,
		"roles":   session.Roles,
		"iat":     session.Created,
		"rs":      getRandomString(16),
	})
	signedString, err := token.SignedString([]byte(jwtSecretString))
	if err != nil {
		return "", err
	}
	return signedString, nil
}

func getRandomString(n int) string {
	rand.Seed(time.Now().UnixNano())
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func updateUser(db *sql.DB, userID int64, success bool) error {
	if success {
		stmt, err := db.Prepare("UPDATE users SET last_login = UNIX_TIMESTAMP(), login_count = login_count + 1, failed_logins = 0 WHERE user_id = ?")
		if err != nil {
			return err
		}
		_, err = stmt.Exec(userID)
		if err != nil {
			return err
		}
	} else {
		stmt, err := db.Prepare("UPDATE users SET failed_logins = failed_logins + 1, `status` = `status` | IF(failed_logins >= ?, ?, 0) WHERE user_id = ?")
		if err != nil {
			return err
		}
		_, err = stmt.Exec(maxLoginAttempts, userLockedLogin, userID)
		if err != nil {
			return err
		}
	}
	return nil
}

func newSessionData(user *userForAuth, rolesMap map[string]string) *SessionData {
	return &SessionData{
		Session: redis.Session{
			UserID:  user.UserID,
			Roles:   rolesMap,
			Created: time.Now().Unix(),
			Timeout: sessionTTL,
		},
		Status:      user.Status,
		UserCreated: user.Created,
	}
}
