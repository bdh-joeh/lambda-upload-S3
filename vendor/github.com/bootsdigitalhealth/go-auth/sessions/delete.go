package sessions

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/bootsdigitalhealth/go-db/redis"
	baseRedis "github.com/go-redis/redis"
	"time"
)

const (
	keyActiveClinicianList = "active_clinician_list"
)

/*
Delete deletes a single session.

  - If you have a list of sessions to delete, user DeleteMultiple

  - If you need to find and delete all sessions for a user, use FindAndDeleteAllByToken

  - If you have the user hash using password.Hash, use DeleteAllByUserHash
*/
func Delete(db *sql.DB, redisClient *redis.Client, token string) error {
	session, err := redisClient.GetSession(token)
	if err != nil {
		return err
	}
	if session.UserID == 0 {
		return nil
	}

	userSessions, err := FindUserSessionsByAuthToken(db, redisClient, token)
	if err != nil {
		return err
	}
	if len(userSessions.Sessions) != 0 {
		err = removeSessionFromUserSessions(redisClient, &userSessions, token)
	}

	err = redisClient.Del(token).Err()
	if err != nil {
		return err
	}

	err = CloseSessionSummary(db, session.Token)
	if err != nil {
		return err
	}
	return nil
}

func removeSessionFromUserSessions(redisClient *redis.Client, userSessions *UserSessions, token string) error {
	userHashToken := userSessions.UserIDHash
	sessions := userSessions.Sessions
	for i, userSession := range sessions {
		if userSession.Token == token {
			sessions[len(sessions)-1], sessions[i] = sessions[i], sessions[len(sessions)-1]
			userSessions.Sessions = sessions[:len(sessions)-1]
		}
	}
	if len(userSessions.Sessions) == 0 {
		err := redisClient.Del(userHashToken).Err()
		if err != nil {
			return err
		}
		return nil
	}
	updatedUserSessions, err := json.Marshal(userSessions)
	if err != nil {
		return err
	}
	err = redisClient.Set(userHashToken, updatedUserSessions, time.Second*sessionTTL).Err()
	if err != nil {
		return err
	}
	return nil
}

// DeleteMultiple takes a slice of tokens and deletes them using Delete.
func DeleteMultiple(db *sql.DB, redisClient *redis.Client, tokens []string) error {
	for _, token := range tokens {
		err := Delete(db, redisClient, token)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
FindAndDeleteAllByToken finds the user in the DB with the provided auth token, finds all sessions belonging to the user, and deletes them.

  - If you already have the user hash available using password.Hash, use DeleteAllByUserHash
*/
func FindAndDeleteAllByToken(db *sql.DB, redisClient *redis.Client, token string) error {
	session, err := redisClient.GetSession(token)
	if err != nil {
		return err
	}
	if session.UserID == 0 {
		return nil
	}

	userSessions, err := FindUserSessionsByAuthToken(db, redisClient, token)
	if err != nil {
		return err
	}
	if len(userSessions.Sessions) == 0 {
		return nil
	}
	numDeletedSessions, err := DeleteUserSessions(db, redisClient, userSessions)
	if err != nil {
		return err
	}
	fmt.Printf("deleted %v sessions for user: %v\n", numDeletedSessions, session.UserID)
	return nil
}

/*
	DeleteAllByUserHash finds the array of sessions for the given user hash and deletes them.

- Will return nil if no sessions are found (e.g. password reset)
*/
func DeleteAllByUserHash(db *sql.DB, redisClient *redis.Client, userHashToken string, userID int64) error {
	var userSessions UserSessions
	sessionBytes, err := redisClient.Get(userHashToken).Bytes()
	if err != nil {
		switch err {
		case baseRedis.Nil:
			fmt.Printf("no sessions for user: %v\n", userID)
			return nil
		default:
			return err
		}
	}
	if len(sessionBytes) == 0 {
		return nil
	}
	err = json.Unmarshal(sessionBytes, &userSessions)
	numDeletedSessions, err := DeleteUserSessions(db, redisClient, userSessions)
	if err != nil {
		return err
	}
	fmt.Printf("deleted %v sessions for user: %v\n", numDeletedSessions, userID)
	return nil
}

// CloseSessionSummary updates the session_summaries table with a given token, setting the end field to now.
func CloseSessionSummary(db *sql.DB, token string) error {
	stmt, err := db.Prepare("UPDATE session_summaries SET ended = UNIX_TIMESTAMP(), hard_logout = 1 WHERE token = ?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(token)
	if err != nil {
		return err
	}
	return nil
}

// FindUserSessionsByAuthToken finds and returns the user's list of session tokens by their current authentication token.
func FindUserSessionsByAuthToken(db *sql.DB, redisClient *redis.Client, token string) (UserSessions, error) {
	var userSessions UserSessions
	var err error
	redisSession, err := redisClient.GetSession(token)
	if err != nil {
		return userSessions, err
	}
	if redisSession.UserID == 0 {
		return userSessions, nil
	}
	userCreated, err := getUserCreatedByID(db, redisSession.UserID)
	sessionData := createSessionData(redisSession, userCreated)
	if err != nil {
		return userSessions, err
	}
	userHashToken, err := getUserHashToken(sessionData)
	if err != nil {
		return userSessions, err
	}
	sessionBytes, err := redisClient.Get(userHashToken).Bytes()
	if err != nil {
		switch err {
		case baseRedis.Nil:
			return userSessions, nil
		default:
			return userSessions, err
		}
	}
	err = json.Unmarshal(sessionBytes, &userSessions)
	if err != nil {
		return userSessions, err
	}
	return userSessions, nil
}

/*
	 DeleteUserSessions deletes all sessions for a given user by passing
		the array of their sessions and returns the count.

To find userSessions, use FindUserSessionsByAuthToken.
*/
func DeleteUserSessions(db *sql.DB, redisClient *redis.Client, userSessions UserSessions) (int, error) {
	var numDeleted int
	for _, session := range userSessions.Sessions {
		err := deleteKey(redisClient, session.Token)
		if err != nil {
			return 0, err
		}
		for _, roleId := range session.Roles {
			if roleId == 4 {
				err := deleteKey(redisClient, keyActiveClinicianList)
				if err != nil {
					return 0, err
				}
			}
		}
		err = CloseSessionSummary(db, session.Token)
		if err != nil {
			return 0, err
		}
		numDeleted++
	}
	err := deleteKey(redisClient, userSessions.UserIDHash)
	if err != nil {
		return 0, err
	}
	return numDeleted, nil
}

func deleteKey(redisClient *redis.Client, token string) error {
	err := redisClient.Del(token).Err()
	if err != nil {
		return err
	}
	return nil
}

func getUserCreatedByID(db *sql.DB, userID int64) (int64, error) {
	var created int64

	row := db.QueryRow(getUserCreatedByIDSQL(), userID)
	err := row.Scan(&created)
	if err != nil {
		return 0, err
	}
	return created, nil
}

func createSessionData(redisSession redis.Session, userCreated int64) *SessionData {
	return &SessionData{
		Session: redis.Session{
			UserID: redisSession.UserID,
		},
		UserCreated: userCreated,
	}
}

func getUserCreatedByIDSQL() string {
	return `
	SELECT
		created 
	FROM users
	WHERE user_id = ?
	LIMIT 1;
	`
}
