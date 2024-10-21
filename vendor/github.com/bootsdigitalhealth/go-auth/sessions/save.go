package sessions

import (
	"encoding/json"
	"github.com/bootsdigitalhealth/go-auth/password"
	"github.com/bootsdigitalhealth/go-db/redis"
	baseRedis "github.com/go-redis/redis"
	"strconv"
	"time"
)

type UserSession struct {
	Token string `json:"Token"`
	Roles []int  `json:"roles_list"`
}

type UserSessions struct {
	UserIDHash string
	Sessions   []UserSession
}

// Save takes a token and session data and saves it in Redis,
// as well as creating a new hash for the user to append the list of sessions for faster delete.
func Save(redisClient *redis.Client, token string, session *SessionData) error {
	var userSessions UserSessions
	var userSession UserSession

	body, _ := json.Marshal(session)
	tokenExists, err := redisClient.Exists(token).Result()
	if tokenExists == 1 {
		return NonUniqueToken{}
	} else if err != nil {
		return err
	}
	err = redisClient.Set(token, body, time.Second*sessionTTL).Err()
	if err != nil {
		return err
	}

	userHashToken, err := getUserHashToken(session)
	if err != nil {
		return err
	}
	userSessionsBytes, err := redisClient.Get(userHashToken).Bytes()
	if err != nil {
		switch err {
		case baseRedis.Nil:
			userSessions.UserIDHash = userHashToken
		default:
			return err
		}
	} else {
		err = json.Unmarshal(userSessionsBytes, &userSessions)
		if err != nil {
			return err
		}
	}
	var rolesList []int
	for roleID := range session.Roles {
		id, err := strconv.Atoi(roleID)
		if err != nil {
			return err
		}
		rolesList = append(rolesList, id)
	}
	userSession.Token = token
	userSession.Roles = rolesList
	userSessions.Sessions = append(userSessions.Sessions, userSession)
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

func getUserHashToken(session *SessionData) (string, error) {
	return password.Hash(session.UserID, session.UserCreated)
}
